// The codes this module refuses a caller with.
//
// A refusal a person or a client can see is a registered code, never a bare
// English sentence. The code is what the browser half switches on to choose a
// translation, and the English sentence travels beside it so a log line and an
// untranslated screen still read.
package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/prusa3d/prusa-connect/sdk/fault"
)

// faultDomain is this module's namespace, derived from the Go module path
// github.com/honzajaros/prusa-connect-bed-mesh.
//
//nolint:gochecknoglobals // a domain built once at start-up; a const cannot hold one.
var faultDomain = mustDomain("com.github.aux.honzajaros.prusa-connect-bed-mesh")

// The codes this module raises.
//
//nolint:gochecknoglobals // definitions built once at start-up.
var (
	// RouteNotFound is a path this module does not serve.
	RouteNotFound = fault.Register(faultDomain, "ROUTE_NOT_FOUND", http.StatusNotFound, fault.Document(
		"This module does not serve that path",
		"Read openapi.json. Every path in it is relative to the mount the operator gave this module.",
		fault.RetryNever,
		fault.ShowDetail,
		"path",
	))

	// InvalidBody is a request body that is not the JSON the route takes.
	InvalidBody = fault.Register(faultDomain, "INVALID_BODY", http.StatusBadRequest, fault.Document(
		"The request body is not what this route takes",
		"Send the JSON object openapi.json describes for this route.",
		fault.RetryNever,
		fault.ShowDetail,
	))

	// NoMeshInText is pasted text the parser found no bed mesh in.
	NoMeshInText = fault.Register(faultDomain, "NO_MESH_IN_TEXT", http.StatusUnprocessableEntity, fault.Document(
		"No bed mesh was found in the pasted text",
		"Paste everything the printer printed after M420 V, from the line Bed Topography Report to the last row of numbers.",
		fault.RetryNever,
		fault.ShowDetail,
	))

	// TextTooLong is pasted text longer than the parser reads.
	TextTooLong = fault.Register(faultDomain, "TEXT_TOO_LONG", http.StatusRequestEntityTooLarge, fault.Document(
		"The pasted text is too long",
		"Paste only the output of M420 V rather than a whole console log.",
		fault.RetryNever,
		fault.ShowDetail,
		"limit",
	))

	// NoteTooLong is a note longer than a mesh may carry.
	NoteTooLong = fault.Register(faultDomain, "NOTE_TOO_LONG", http.StatusBadRequest, fault.Document(
		"The note is too long",
		"Shorten the note.",
		fault.RetryNever,
		fault.ShowDetail,
		"limit",
	))

	// PrinterNotFound is a printer that does not exist or that the person may
	// not see.
	PrinterNotFound = fault.Register(faultDomain, "PRINTER_NOT_FOUND", http.StatusNotFound, fault.Document(
		"There is no such printer",
		"Open the printer from the printer list. A printer you may not see is not found either.",
		fault.RetryNever,
		fault.ShowDetail,
		"device_id",
	))

	// MeshNotFound is a mesh id the printer holds no mesh under.
	MeshNotFound = fault.Register(faultDomain, "MESH_NOT_FOUND", http.StatusNotFound, fault.Document(
		"There is no such mesh",
		"Reload the history. The mesh may have been deleted, or dropped as the oldest of the history.",
		fault.RetryNever,
		fault.ShowDetail,
		"mesh_id",
	))

	// Unavailable is a call to the server that failed for a reason of the
	// server's own.
	Unavailable = fault.Register(faultDomain, "UNAVAILABLE", http.StatusServiceUnavailable, fault.Document(
		"The bed mesh store is not available right now",
		"Try again in a moment. The server log says why.",
		fault.RetryAllowed,
		fault.RedactDetail,
	))
)

// mustDomain builds this module's fault domain. A name the sdk refuses is a
// mistake in this file, so it stops the process at start-up.
func mustDomain(name string) fault.Domain {
	domain, err := fault.NewDomain(name)
	if err != nil {
		panic(fmt.Sprintf("bedmesh: %s is not a fault domain: %v", name, err))
	}

	return domain
}

// refuse writes one refusal as RFC 9457 frames it.
func refuse(w http.ResponseWriter, r *http.Request, raised *fault.Fault) {
	problem := raised.ProblemJSON(r.URL.Path)

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)

	_ = json.NewEncoder(w).Encode(problem)
}
