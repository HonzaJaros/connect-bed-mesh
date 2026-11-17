// Package filerecords keeps records as files in the module's own data
// directory, behind the same interface as the server's record store.
//
// Prusa Connect serves its record store only when the operator declared a
// record backend, and the first-run wizard declares none. Every server grants
// a data directory to a module that asks for one, so the meshes live there:
// one JSON file per record, at the key's path with ".json" after it.
package filerecords

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/prusa3d/prusa-connect/sdk/stateapi"
)

// Limit is how much the directory may hold, in bytes.
const Limit = 256 << 20

const suffix = ".json"

// Dir is the records of one module in one directory.
type Dir struct {
	root string
	mu   sync.Mutex
}

var _ stateapi.Records = (*Dir)(nil)

// New keeps records under root, which must exist.
func New(root string) *Dir { return &Dir{root: root} }

// path is where a key's record lives. A key is a path of plain names: no
// empty segment, no "." or "..", nothing that escapes the root.
func (d *Dir) path(key stateapi.Key) (string, error) {
	if err := key.Validate(); err != nil {
		return "", err
	}

	segments := strings.Split(string(key), "/")
	for _, segment := range segments {
		if segment == "" || segment == "." || segment == ".." || strings.ContainsAny(segment, `\:`) ||
			strings.HasPrefix(segment, ".") {
			return "", fmt.Errorf("%w: %q is not a plain name", stateapi.ErrKey, segment)
		}
	}

	return filepath.Join(append([]string{d.root}, segments...)...) + suffix, nil
}

// Get returns the record under key.
func (d *Dir) Get(_ context.Context, key stateapi.Key) (stateapi.Value, error) {
	path, err := d.path(key)
	if err != nil {
		return stateapi.Value{}, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	value, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return stateapi.Value{}, stateapi.ErrNoRecord
	}

	if err != nil {
		return stateapi.Value{}, fmt.Errorf("read %s: %w", key, err)
	}

	return stateapi.Value{Bytes: value, Version: 1}, nil
}

// Put writes value under key, through a temporary file, so a reader never
// sees half a record.
func (d *Dir) Put(_ context.Context, key stateapi.Key, value []byte) (uint64, error) {
	path, err := d.path(key)
	if err != nil {
		return 0, err
	}

	if err = stateapi.ValidateValue(value); err != nil {
		return 0, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if err = os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return 0, fmt.Errorf("make the folder of %s: %w", key, err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".write-*")
	if err != nil {
		return 0, fmt.Errorf("write %s: %w", key, err)
	}

	_, err = temp.Write(value)
	if closeErr := temp.Close(); err == nil {
		err = closeErr
	}

	if err == nil {
		err = os.Rename(temp.Name(), path)
	}

	if err != nil {
		_ = os.Remove(temp.Name())

		return 0, fmt.Errorf("write %s: %w", key, err)
	}

	return 1, nil
}

// CompareAndPut writes value under key. This module has one writer per key,
// so it writes whatever is there.
func (d *Dir) CompareAndPut(ctx context.Context, key stateapi.Key, _ uint64, value []byte) (uint64, error) {
	return d.Put(ctx, key, value)
}

// Delete removes the record under key, and the folders it leaves empty.
func (d *Dir) Delete(_ context.Context, key stateapi.Key) error {
	path, err := d.path(key)
	if err != nil {
		return err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if err = os.Remove(path); errors.Is(err, fs.ErrNotExist) {
		return stateapi.ErrNoRecord
	} else if err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}

	for dir := filepath.Dir(path); dir != d.root && strings.HasPrefix(dir, d.root); dir = filepath.Dir(dir) {
		if os.Remove(dir) != nil {
			break // not empty
		}
	}

	return nil
}

// List returns every key under prefix, sorted.
func (d *Dir) List(_ context.Context, prefix stateapi.Key) ([]stateapi.Key, error) {
	if err := stateapi.ValidatePrefix(prefix); err != nil {
		return nil, err
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	var keys []stateapi.Key

	err := filepath.WalkDir(d.root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") || !strings.HasSuffix(path, suffix) {
			return nil
		}

		relative, err := filepath.Rel(d.root, strings.TrimSuffix(path, suffix))
		if err != nil {
			return err
		}

		if key := stateapi.Key(filepath.ToSlash(relative)); strings.HasPrefix(string(key), string(prefix)) {
			keys = append(keys, key)
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("list the records: %w", err)
	}

	slices.Sort(keys)

	return keys, nil
}

// Version is 1 for a record that exists.
func (d *Dir) Version(ctx context.Context, key stateapi.Key) (uint64, error) {
	if _, err := d.Get(ctx, key); err != nil {
		return 0, err
	}

	return 1, nil
}

// Usage is what the directory holds, and Limit.
func (d *Dir) Usage(context.Context) (stateapi.Usage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	usage := stateapi.Usage{Limit: Limit}

	err := filepath.WalkDir(d.root, func(_ string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}

		info, err := entry.Info()
		if err == nil {
			usage.Bytes += info.Size()
		}

		return err
	})

	return usage, err
}
