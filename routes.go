package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/prusa3d/prusa-connect/sdk/deviceapi"
	"github.com/prusa3d/prusa-connect/sdk/fault"
	"github.com/prusa3d/prusa-connect/sdk/routesapi"

	"github.com/honzajaros/connect-bed-mesh/internal/mesh"
	"github.com/honzajaros/connect-bed-mesh/internal/store"
)

// Fleet is the one call this module makes to the core: reading a printer as
// the person who asked, which both proves they may see it and says what it is.
type Fleet interface {
	Get(ctx context.Context, params deviceapi.GetParams) (deviceapi.Device, error)
}

// Meshes is the store the routes read and write.
type Meshes interface {
	Save(ctx context.Context, deviceID string, measured mesh.Mesh, note string, printer store.Printer) (store.Record, error)
	List(ctx context.Context, deviceID string) ([]store.Summary, error)
	Get(ctx context.Context, deviceID, id string) (store.Record, error)
	Delete(ctx context.Context, deviceID, id string) error
}

// Parsed is a mesh read out of pasted text and not saved.
type Parsed struct {
	Format mesh.Format  `json:"format"`
	Cols   int          `json:"cols"`
	Rows   int          `json:"rows"`
	Z      [][]*float64 `json:"z"`
	Bounds *mesh.Bounds `json:"bounds,omitempty"`
	Stats  mesh.Stats   `json:"stats"`
}

// pasted is the body of POST /parse and POST /printers/{device_id}/meshes.
type pasted struct {
	Text string `json:"text"`
	Note string `json:"note"`
}

// The routes, relative to the mount:
//
//	POST   /parse                                  read a mesh out of text, save nothing
//	GET    /printers/{device_id}/meshes            the printer's meshes, newest first
//	POST   /printers/{device_id}/meshes            read a mesh out of text and save it
//	GET    /printers/{device_id}/meshes/{mesh_id}  one mesh with its heights
//	DELETE /printers/{device_id}/meshes/{mesh_id}  delete one mesh
func routes(fleet Fleet, meshes Meshes) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /parse", func(w http.ResponseWriter, r *http.Request) {
		body, ok := readPasted(w, r)
		if !ok {
			return
		}

		measured, ok := parse(w, r, body.Text)
		if !ok {
			return
		}

		answer(w, http.StatusOK, Parsed{
			Format: measured.Format, Cols: measured.Cols, Rows: measured.Rows,
			Z: measured.Z, Bounds: measured.Bounds, Stats: measured.Stats(),
		})
	})

	mux.HandleFunc("GET /printers/{device_id}/meshes", func(w http.ResponseWriter, r *http.Request) {
		deviceID, _, ok := printer(w, r, fleet)
		if !ok {
			return
		}

		summaries, err := meshes.List(r.Context(), deviceID)
		if err != nil {
			unavailable(w, r, err)

			return
		}

		answer(w, http.StatusOK, map[string]any{"meshes": summaries})
	})

	mux.HandleFunc("POST /printers/{device_id}/meshes", func(w http.ResponseWriter, r *http.Request) {
		deviceID, device, ok := printer(w, r, fleet)
		if !ok {
			return
		}

		body, ok := readPasted(w, r)
		if !ok {
			return
		}

		if len(body.Note) > store.MaxNote {
			refuse(w, r, NoteTooLong.New("the note is longer than a mesh may carry").
				With(fault.IntParam("limit", store.MaxNote)))

			return
		}

		measured, ok := parse(w, r, body.Text)
		if !ok {
			return
		}

		saved, err := meshes.Save(r.Context(), deviceID, measured, body.Note, describe(device))
		if err != nil {
			unavailable(w, r, err)

			return
		}

		answer(w, http.StatusCreated, saved)
	})

	mux.HandleFunc("GET /printers/{device_id}/meshes/{mesh_id}", func(w http.ResponseWriter, r *http.Request) {
		deviceID, _, ok := printer(w, r, fleet)
		if !ok {
			return
		}

		record, err := meshes.Get(r.Context(), deviceID, r.PathValue("mesh_id"))
		if !found(w, r, err) {
			return
		}

		answer(w, http.StatusOK, record)
	})

	mux.HandleFunc("DELETE /printers/{device_id}/meshes/{mesh_id}", func(w http.ResponseWriter, r *http.Request) {
		deviceID, _, ok := printer(w, r, fleet)
		if !ok {
			return
		}

		if !found(w, r, meshes.Delete(r.Context(), deviceID, r.PathValue("mesh_id"))) {
			return
		}

		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		refuse(w, r, RouteNotFound.
			New("this module serves no route at "+r.URL.Path).
			With(fault.StringParam("path", r.URL.Path)))
	})

	return mux
}

// printer reads the printer the path names, as the person asking. A printer
// the person may not see is answered as one that does not exist.
func printer(w http.ResponseWriter, r *http.Request, fleet Fleet) (string, deviceapi.Device, bool) {
	deviceID := r.PathValue("device_id")

	device, err := fleet.Get(r.Context(), deviceapi.GetParams{
		Principal: routesapi.PrincipalOf(r),
		ID:        deviceID,
	})

	switch {
	case err == nil && device.ID == deviceID:
		return deviceID, device, true
	case err == nil, errors.Is(err, deviceapi.ErrNoDevice), errors.Is(err, deviceapi.ErrDenied),
		errors.Is(err, deviceapi.ErrInvalidQuery):
		refuse(w, r, PrinterNotFound.New("there is no printer "+deviceID).
			With(fault.StringParam("device_id", deviceID)))
	default:
		unavailable(w, r, err)
	}

	return "", deviceapi.Device{}, false
}

// describe is what a saved mesh records about its printer.
func describe(device deviceapi.Device) store.Printer {
	described := store.Printer{
		Name:     device.Name,
		Model:    device.Model,
		Firmware: device.Firmware,
	}

	if raw, ok := device.State["/temperature/bed"]; ok {
		var temp float64
		if json.Unmarshal(raw, &temp) == nil {
			described.BedTemp = &temp
		}
	}

	return described
}

func readPasted(w http.ResponseWriter, r *http.Request) (pasted, bool) {
	var body pasted

	decoder := json.NewDecoder(io.LimitReader(r.Body, mesh.MaxInput+store.MaxNote+1024))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&body); err != nil {
		refuse(w, r, InvalidBody.New("the body is not a JSON object with text and note: "+err.Error()))

		return pasted{}, false
	}

	return body, true
}

func parse(w http.ResponseWriter, r *http.Request, text string) (mesh.Mesh, bool) {
	measured, err := mesh.Parse(text)

	switch {
	case err == nil:
		return measured, true
	case errors.Is(err, mesh.ErrTooLarge):
		refuse(w, r, TextTooLong.New("the text is longer than the parser reads").
			With(fault.IntParam("limit", mesh.MaxInput)))
	default:
		refuse(w, r, NoMeshInText.New(err.Error()))
	}

	return mesh.Mesh{}, false
}

func found(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case err == nil:
		return true
	case errors.Is(err, store.ErrNoMesh):
		id := r.PathValue("mesh_id")
		refuse(w, r, MeshNotFound.New("there is no mesh "+strconv.Quote(id)).
			With(fault.StringParam("mesh_id", id)))
	default:
		unavailable(w, r, err)
	}

	return false
}

func unavailable(w http.ResponseWriter, r *http.Request, err error) {
	fmt.Fprintf(os.Stderr, "%s: %s %s: %v\n", name, r.Method, r.URL.Path, err)
	refuse(w, r, Unavailable.New(err.Error()))
}

// answer writes one JSON body.
func answer(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(body)
}
