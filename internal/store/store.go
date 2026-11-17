// Package store keeps the meshes of every printer in the module's own
// records.
//
// One mesh is one record, under mesh/<device id>/<mesh id>. A mesh id is the
// time it was saved, zero padded, so the keys of one printer sort oldest
// first. A value is the JSON of a Record, well under the 64 KiB a record may
// hold: the XL's 36 by 36 grid is about 15 KiB.
package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/prusa3d/prusa-connect/sdk/stateapi"

	"github.com/honzajaros/prusa-connect-bed-mesh/internal/mesh"
)

// ErrNoMesh is a mesh id the printer holds no mesh under.
var ErrNoMesh = errors.New("store: no such mesh")

// MaxNote is the longest note a person may write on a mesh, in bytes.
const MaxNote = 500

// Printer is what the store records about the printer a mesh was measured on,
// as Prusa Connect knew it when the mesh was saved.
type Printer struct {
	Name     string   `json:"name,omitempty"`
	Model    string   `json:"model,omitempty"`
	Firmware string   `json:"firmware,omitempty"`
	BedTemp  *float64 `json:"bed_temp,omitempty"`
}

// Summary is a saved mesh without its heights: what a history list shows.
type Summary struct {
	ID       string      `json:"id"`
	DeviceID string      `json:"device_id"`
	SavedAt  time.Time   `json:"saved_at"`
	Note     string      `json:"note,omitempty"`
	Format   mesh.Format `json:"format"`
	Cols     int         `json:"cols"`
	Rows     int         `json:"rows"`
	Stats    mesh.Stats  `json:"stats"`
	Printer  Printer     `json:"printer"`
}

// Record is one saved mesh.
type Record struct {
	Summary

	Z      [][]*float64 `json:"z"`
	Bounds *mesh.Bounds `json:"bounds,omitempty"`
}

// Store is the meshes of every printer.
type Store struct {
	records stateapi.Records
	now     func() time.Time
	// Keep is how many meshes one printer keeps. Saving one more deletes the
	// oldest.
	keep func() int

	mu   sync.Mutex
	last int64 // the millisecond the newest id was drawn at
}

// New is a store over the module's records.
func New(records stateapi.Records, keep func() int) *Store {
	return &Store{records: records, now: time.Now, keep: keep}
}

// Save stores a mesh for a printer and returns it as saved.
func (s *Store) Save(ctx context.Context, deviceID string, measured mesh.Mesh, note string, printer Printer) (Record, error) {
	now := s.now().UTC()
	stamp := s.stamp(now)

	suffix := make([]byte, 3)
	if _, err := rand.Read(suffix); err != nil {
		return Record{}, fmt.Errorf("draw a mesh id: %w", err)
	}

	record := Record{
		Summary: Summary{
			ID:       fmt.Sprintf("%016d-%s", stamp, hex.EncodeToString(suffix)),
			DeviceID: deviceID,
			SavedAt:  now,
			Note:     note,
			Format:   measured.Format,
			Cols:     measured.Cols,
			Rows:     measured.Rows,
			Stats:    measured.Stats(),
			Printer:  printer,
		},
		Z:      measured.Z,
		Bounds: measured.Bounds,
	}

	value, err := json.Marshal(record)
	if err != nil {
		return Record{}, fmt.Errorf("encode the mesh: %w", err)
	}

	if _, err = s.records.Put(ctx, key(deviceID, record.ID), value); err != nil {
		return Record{}, fmt.Errorf("store the mesh: %w", err)
	}

	if err = s.prune(ctx, deviceID); err != nil {
		return Record{}, err
	}

	return record, nil
}

// stamp is the millisecond a new id carries: now, or one past the newest id
// this process drew, so two meshes saved in one millisecond still sort in the
// order they were saved.
func (s *Store) stamp(now time.Time) int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.last = max(now.UnixMilli(), s.last+1)

	return s.last
}

// List returns a printer's meshes, newest first, without their heights.
func (s *Store) List(ctx context.Context, deviceID string) ([]Summary, error) {
	ids, err := s.ids(ctx, deviceID)
	if err != nil {
		return nil, err
	}

	summaries := make([]Summary, 0, len(ids))

	for _, id := range slices.Backward(ids) {
		record, err := s.Get(ctx, deviceID, id)
		if errors.Is(err, ErrNoMesh) {
			continue // deleted between the list and the read
		}

		if err != nil {
			return nil, err
		}

		summaries = append(summaries, record.Summary)
	}

	return summaries, nil
}

// Get returns one mesh.
func (s *Store) Get(ctx context.Context, deviceID, id string) (Record, error) {
	if !validID(id) {
		return Record{}, ErrNoMesh
	}

	value, err := s.records.Get(ctx, key(deviceID, id))
	if errors.Is(err, stateapi.ErrNoRecord) {
		return Record{}, ErrNoMesh
	}

	if err != nil {
		return Record{}, fmt.Errorf("read mesh %s: %w", id, err)
	}

	var record Record
	if err = json.Unmarshal(value.Bytes, &record); err != nil {
		return Record{}, fmt.Errorf("decode mesh %s: %w", id, err)
	}

	return record, nil
}

// Delete removes one mesh.
func (s *Store) Delete(ctx context.Context, deviceID, id string) error {
	if !validID(id) {
		return ErrNoMesh
	}

	err := s.records.Delete(ctx, key(deviceID, id))
	if errors.Is(err, stateapi.ErrNoRecord) {
		return ErrNoMesh
	}

	if err != nil {
		return fmt.Errorf("delete mesh %s: %w", id, err)
	}

	return nil
}

// ids are a printer's mesh ids, oldest first.
func (s *Store) ids(ctx context.Context, deviceID string) ([]string, error) {
	prefix := stateapi.Key("mesh/" + deviceID + "/")

	keys, err := s.records.List(ctx, prefix)
	if err != nil {
		return nil, fmt.Errorf("list the meshes: %w", err)
	}

	ids := make([]string, 0, len(keys))

	for _, k := range keys {
		if id, ok := strings.CutPrefix(string(k), string(prefix)); ok && validID(id) {
			ids = append(ids, id)
		}
	}

	slices.Sort(ids)

	return ids, nil
}

// prune deletes a printer's oldest meshes past the number it keeps.
func (s *Store) prune(ctx context.Context, deviceID string) error {
	ids, err := s.ids(ctx, deviceID)
	if err != nil {
		return err
	}

	for len(ids) > max(1, s.keep()) {
		if err = s.records.Delete(ctx, key(deviceID, ids[0])); err != nil && !errors.Is(err, stateapi.ErrNoRecord) {
			return fmt.Errorf("delete the oldest mesh: %w", err)
		}

		ids = ids[1:]
	}

	return nil
}

func key(deviceID, id string) stateapi.Key { return stateapi.Key("mesh/" + deviceID + "/" + id) }

// validID is the shape Save gives an id: 16 digits, a dash and 6 hex digits.
func validID(id string) bool {
	if len(id) != 23 || id[16] != '-' {
		return false
	}

	for i, r := range id {
		switch {
		case i == 16:
		case i < 16 && r >= '0' && r <= '9':
		case i > 16 && (r >= '0' && r <= '9' || r >= 'a' && r <= 'f'):
		default:
			return false
		}
	}

	return true
}
