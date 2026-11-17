// Command bedmesh is the process half of the Bed Mesh extension for Prusa
// Connect Local. It reads a bed mesh out of the text a printer printed for
// `M420 V`, keeps every printer's meshes in the module's own records, and
// serves them to the printer tab the browser half draws.
//
// The routes are in routes.go and openapi.json. Every path is relative to the
// mount, /api/v1/modules/com.github.aux.honzajaros.prusa-connect-bed-mesh/.
// The host decides who may call a route (view_resource on the instance for a
// read, operate_resource for a write), and this process reads every printer as
// the person who asked, so nobody reads the meshes of a printer they may not
// see.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"maps"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"

	"github.com/prusa3d/prusa-connect/sdk/deviceapi"
	"github.com/prusa3d/prusa-connect/sdk/extension"
	"github.com/prusa3d/prusa-connect/sdk/routesapi"
	"github.com/prusa3d/prusa-connect/sdk/stateapi"
	"github.com/prusa3d/prusa-connect/sdk/wire"
	"github.com/prusa3d/prusa-connect/sdk/wirerpc"

	"github.com/honzajaros/prusa-connect-bed-mesh/internal/filerecords"
	"github.com/honzajaros/prusa-connect-bed-mesh/internal/store"
)

// manifestYAML is module.yaml, built into the binary, so the name and version
// this process reports are the manifest's own.
//
//go:embed module.yaml
var manifestYAML []byte

//nolint:gochecknoglobals // read once from the embedded manifest.
var (
	name    = manifestScalar(manifestYAML, "name")
	release = extension.Version(manifestScalar(manifestYAML, "version"))
)

// manifestScalar is the value of a top-level `key: value` line of the
// manifest, without a trailing comment or quotes.
func manifestScalar(document []byte, key string) string {
	for _, line := range strings.Split(string(document), "\n") {
		value, found := strings.CutPrefix(line, key+":")
		if !found {
			continue
		}

		if comment := strings.Index(value, " #"); comment >= 0 {
			value = value[:comment]
		}

		return strings.Trim(strings.TrimSpace(value), `"'`)
	}

	return ""
}

// defaultKeep is how many meshes one printer keeps when the operator sets
// nothing. It is the default of `keep_history` in module.yaml.
const defaultKeep = 50

// Settings is what an operator writes under this instance's config block.
type Settings struct {
	// KeepHistory is how many meshes one printer keeps.
	KeepHistory int `json:"keep_history"`
}

//nolint:gochecknoglobals // the settings this process runs on.
var held atomic.Pointer[Settings]

func keep() int {
	if current := held.Load(); current != nil && current.KeepHistory > 0 {
		return current.KeepHistory
	}

	return defaultKeep
}

//go:embed openapi.json
var openAPI []byte

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
		os.Exit(1)
	}
}

// run writes the handshake line and then serves frames until the host closes
// the channel.
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	line, err := wire.Handshake{
		Protocol: wire.ProtocolMajor,
		MinMinor: wire.MinMinor,
		MaxMinor: wire.MaxMinor,
		Address:  wire.StdioAddress,
	}.Line()
	if err != nil {
		return fmt.Errorf("render the handshake: %w", err)
	}

	if _, err = os.Stdout.WriteString(line); err != nil {
		return fmt.Errorf("write the handshake: %w", err)
	}

	// The connection serves the host's calls and carries this process's own
	// calls to the host: the records and the fleet. The methods are filled in
	// once the clients over it exist.
	serves := wirerpc.Methods{}

	conn, err := wirerpc.NewConn(stdio{}, wirerpc.Options{
		Role:    wirerpc.RoleChild,
		Methods: serves,
	})
	if err != nil {
		return fmt.Errorf("build the session: %w", err)
	}

	records := &backend{fallback: stateapi.NewClient(conn)}
	meshes := store.New(records, keep)
	maps.Copy(serves, methods(deviceapi.NewClient(conn), meshes, records))

	fmt.Fprintf(os.Stderr, "%s %s is serving on stdio as pid %d\n", name, release, os.Getpid())

	if err = conn.Serve(ctx); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

// methods is the lifecycle set every child answers, plus the call that
// carries this module's HTTP surface.
func methods(fleet Fleet, meshes Meshes, records *backend) wirerpc.Methods {
	return wirerpc.Methods{
		wirerpc.MethodInfo: func(context.Context, wirerpc.Call) (any, error) {
			return routesapi.Describe(
				wirerpc.Info{
					Name:     name,
					Version:  release,
					Contract: wirerpc.Contract,
					Serves:   []wirerpc.Method{wirerpc.MethodConfigure},
				}, openAPI)
		},
		wirerpc.MethodConfigure: wirerpc.ConfigureGranted(func(_ context.Context, sent Settings, grants wirerpc.Grants) error {
			if sent.KeepHistory < 1 {
				sent.KeepHistory = defaultKeep
			}

			held.Store(&sent)

			if grants.Data != "" {
				records.dir.Store(filerecords.New(grants.Data))
				fmt.Fprintf(os.Stderr, "%s keeps its meshes in %s\n", name, grants.Data)
			} else {
				fmt.Fprintf(os.Stderr, "%s was granted no data directory, so it keeps its meshes in the server's record store\n", name)
			}

			return nil
		}),
		wirerpc.MethodShutdown: func(context.Context, wirerpc.Call) (any, error) {
			fmt.Fprintf(os.Stderr, "%s was asked to stop\n", name)

			return nil, nil
		},
		routesapi.MethodServe: routesapi.Serve(routes(fleet, meshes)),
	}
}

// backend is where the meshes are kept: the data directory the operator
// granted, or the server's record store when none was granted. The record
// store is served only on a server that declared a record backend, which the
// first-run wizard does not, so the directory is what most servers use.
type backend struct {
	fallback stateapi.Records
	dir      atomic.Pointer[filerecords.Dir]
}

func (b *backend) current() stateapi.Records {
	if dir := b.dir.Load(); dir != nil {
		return dir
	}

	return b.fallback
}

func (b *backend) Get(ctx context.Context, key stateapi.Key) (stateapi.Value, error) {
	return b.current().Get(ctx, key)
}

func (b *backend) Put(ctx context.Context, key stateapi.Key, value []byte) (uint64, error) {
	return b.current().Put(ctx, key, value)
}

func (b *backend) CompareAndPut(ctx context.Context, key stateapi.Key, expected uint64, value []byte) (uint64, error) {
	return b.current().CompareAndPut(ctx, key, expected, value)
}

func (b *backend) Delete(ctx context.Context, key stateapi.Key) error {
	return b.current().Delete(ctx, key)
}

func (b *backend) List(ctx context.Context, prefix stateapi.Key) ([]stateapi.Key, error) {
	return b.current().List(ctx, prefix)
}

func (b *backend) Version(ctx context.Context, key stateapi.Key) (uint64, error) {
	return b.current().Version(ctx, key)
}

func (b *backend) Usage(ctx context.Context) (stateapi.Usage, error) {
	return b.current().Usage(ctx)
}

// stdio is the pair the host handed over.
type stdio struct{}

func (stdio) Read(p []byte) (int, error)  { return os.Stdin.Read(p) }
func (stdio) Write(p []byte) (int, error) { return os.Stdout.Write(p) }
func (stdio) Close() error                { return os.Stdin.Close() }
