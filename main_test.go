package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/prusa3d/prusa-connect/sdk/deviceapi"
	"github.com/prusa3d/prusa-connect/sdk/stateapi"

	"github.com/honzajaros/connect-bed-mesh/internal/store"
)

const printerID = "6968e07e-2fe8-498a-9ce3-cdebcf415f94"

// report is what Buddy firmware prints for M420 V on a 3 by 3 grid.
const report = `Recv: Bed Topography Report:
Recv:
Recv:     (  2,206)              (242,206)
Recv:         0      1      2
Recv:  2 | +0.100  -0.050 [ 0.000]
Recv:    |
Recv:  1 |   .     +0.020  -0.010
Recv:    |
Recv:  0 | -0.200  +0.000  +0.030
Recv:         0      1      2
Recv:     (  2, -4)              (242, -4)
Recv: ok`

func TestAPasteIsPreviewed(t *testing.T) {
	handler, _ := serve()

	recorder := call(handler, http.MethodPost, "/parse", map[string]string{"text": report})
	if recorder.Code != http.StatusOK {
		t.Fatalf("POST /parse answered %d: %s", recorder.Code, recorder.Body)
	}

	var parsed Parsed
	decode(t, recorder, &parsed)

	if parsed.Cols != 3 || parsed.Rows != 3 || parsed.Stats.Range != 0.3 || parsed.Bounds == nil {
		t.Fatalf("parsed %+v", parsed)
	}
}

func TestAMeshIsSavedListedReadAndDeleted(t *testing.T) {
	handler, _ := serve()
	base := "/printers/" + printerID + "/meshes"

	saved := call(handler, http.MethodPost, base, map[string]string{"text": report, "note": "satin sheet"})
	if saved.Code != http.StatusCreated {
		t.Fatalf("save answered %d: %s", saved.Code, saved.Body)
	}

	var record store.Record
	decode(t, saved, &record)

	if record.Printer.Model != "MK4S" || record.Printer.BedTemp == nil || *record.Printer.BedTemp != 60 {
		t.Fatalf("the printer was recorded as %+v", record.Printer)
	}

	var listed struct {
		Meshes []store.Summary `json:"meshes"`
	}
	decode(t, call(handler, http.MethodGet, base, nil), &listed)

	if len(listed.Meshes) != 1 || listed.Meshes[0].ID != record.ID || listed.Meshes[0].Note != "satin sheet" {
		t.Fatalf("listed %+v", listed.Meshes)
	}

	read := call(handler, http.MethodGet, base+"/"+record.ID, nil)
	if read.Code != http.StatusOK || !strings.Contains(read.Body.String(), `"z":[[-0.2,0,0.03]`) {
		t.Fatalf("read answered %d: %s", read.Code, read.Body)
	}

	if deleted := call(handler, http.MethodDelete, base+"/"+record.ID, nil); deleted.Code != http.StatusNoContent {
		t.Fatalf("delete answered %d", deleted.Code)
	}

	assertCode(t, call(handler, http.MethodGet, base+"/"+record.ID, nil), http.StatusNotFound, "MESH_NOT_FOUND")
}

func TestTheHistoryKeepsTheNewest(t *testing.T) {
	handler, _ := serve()
	base := "/printers/" + printerID + "/meshes"

	held.Store(&Settings{KeepHistory: 2})
	t.Cleanup(func() { held.Store(nil) })

	for i := range 3 {
		if code := call(handler, http.MethodPost, base, map[string]string{"text": report, "note": fmt.Sprint(i)}).Code; code != http.StatusCreated {
			t.Fatalf("save %d answered %d", i, code)
		}
	}

	var listed struct {
		Meshes []store.Summary `json:"meshes"`
	}
	decode(t, call(handler, http.MethodGet, base, nil), &listed)

	notes := []string{}
	for _, summary := range listed.Meshes {
		notes = append(notes, summary.Note)
	}

	if !slices.Equal(notes, []string{"2", "1"}) {
		t.Fatalf("the history holds %v, want the two newest, newest first", notes)
	}
}

func TestRefusals(t *testing.T) {
	handler, _ := serve()
	base := "/printers/" + printerID + "/meshes"

	for name, test := range map[string]struct {
		method, path string
		body         any
		status       int
		code         string
	}{
		"no mesh in the text": {http.MethodPost, base, map[string]string{"text": "echo:Invalid mesh."}, http.StatusUnprocessableEntity, "NO_MESH_IN_TEXT"},
		"an unknown field":    {http.MethodPost, "/parse", map[string]string{"txt": report}, http.StatusBadRequest, "INVALID_BODY"},
		"a long note":         {http.MethodPost, base, map[string]string{"text": report, "note": strings.Repeat("x", 501)}, http.StatusBadRequest, "NOTE_TOO_LONG"},
		"another printer":     {http.MethodGet, "/printers/someone-elses/meshes", nil, http.StatusNotFound, "PRINTER_NOT_FOUND"},
		"a malformed mesh id": {http.MethodGet, base + "/..%2F..", nil, http.StatusNotFound, "MESH_NOT_FOUND"},
		"an unserved path":    {http.MethodGet, "/nothing-here", nil, http.StatusNotFound, "ROUTE_NOT_FOUND"},
	} {
		t.Run(name, func(t *testing.T) {
			assertCode(t, call(handler, test.method, test.path, test.body), test.status, test.code)
		})
	}
}

// TestTheManifestPublishesTheCodesDefault holds the default of keep_history
// in module.yaml to the one this code uses.
func TestTheManifestPublishesTheCodesDefault(t *testing.T) {
	manifest, err := os.ReadFile("module.yaml")
	if err != nil {
		t.Fatal(err)
	}

	if want := fmt.Sprintf("default: \"%d\"", defaultKeep); !strings.Contains(string(manifest), want) {
		t.Fatalf("module.yaml does not publish %s", want)
	}
}

func serve() (http.Handler, *memory) {
	records := &memory{values: map[stateapi.Key][]byte{}}

	return routes(fleet{}, store.New(records, keep)), records
}

func call(handler http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	var reader *strings.Reader

	if body != nil {
		encoded, _ := json.Marshal(body)
		reader = strings.NewReader(string(encoded))
	} else {
		reader = strings.NewReader("")
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, reader))

	return recorder
}

func decode(t *testing.T, recorder *httptest.ResponseRecorder, into any) {
	t.Helper()

	if err := json.Unmarshal(recorder.Body.Bytes(), into); err != nil {
		t.Fatalf("the answer is not JSON: %v: %s", err, recorder.Body)
	}
}

func assertCode(t *testing.T, recorder *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	var problem struct {
		Code string `json:"code"`
	}
	decode(t, recorder, &problem)

	if recorder.Code != status || !strings.HasSuffix(problem.Code, code) {
		t.Fatalf("answered %d %q, want %d %s", recorder.Code, problem.Code, status, code)
	}
}

// fleet holds one printer, which every caller may see.
type fleet struct{}

func (fleet) Get(_ context.Context, params deviceapi.GetParams) (deviceapi.Device, error) {
	if params.ID != printerID {
		return deviceapi.Device{}, deviceapi.ErrNoDevice
	}

	return deviceapi.Device{
		ID: printerID, Name: "MK4S bench", Model: "MK4S", Firmware: "6.4.0",
		State: map[string]json.RawMessage{"/temperature/bed": json.RawMessage("60")},
	}, nil
}

// memory is the records of one namespace, in memory.
type memory struct {
	mu     sync.Mutex
	values map[stateapi.Key][]byte
}

func (m *memory) Get(_ context.Context, key stateapi.Key) (stateapi.Value, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	value, ok := m.values[key]
	if !ok {
		return stateapi.Value{}, stateapi.ErrNoRecord
	}

	return stateapi.Value{Bytes: value, Version: 1}, nil
}

func (m *memory) Put(_ context.Context, key stateapi.Key, value []byte) (uint64, error) {
	if err := key.Validate(); err != nil {
		return 0, err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.values[key] = value

	return 1, nil
}

func (m *memory) CompareAndPut(ctx context.Context, key stateapi.Key, _ uint64, value []byte) (uint64, error) {
	return m.Put(ctx, key, value)
}

func (m *memory) Delete(_ context.Context, key stateapi.Key) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.values[key]; !ok {
		return stateapi.ErrNoRecord
	}

	delete(m.values, key)

	return nil
}

func (m *memory) List(_ context.Context, prefix stateapi.Key) ([]stateapi.Key, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var keys []stateapi.Key

	for key := range m.values {
		if strings.HasPrefix(string(key), string(prefix)) {
			keys = append(keys, key)
		}
	}

	slices.Sort(keys)

	return keys, nil
}

func (m *memory) Version(_ context.Context, key stateapi.Key) (uint64, error) {
	if _, err := m.Get(context.Background(), key); err != nil {
		return 0, err
	}

	return 1, nil
}

func (m *memory) Usage(context.Context) (stateapi.Usage, error) { return stateapi.Usage{}, nil }
