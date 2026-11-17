package filerecords

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/prusa3d/prusa-connect/sdk/stateapi"
)

func TestARecordIsWrittenListedReadAndDeleted(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	dir := New(root)

	for _, key := range []stateapi.Key{"mesh/a/2", "mesh/a/1", "mesh/ab/1"} {
		if _, err := dir.Put(ctx, key, []byte(`{"k":"`+string(key)+`"}`)); err != nil {
			t.Fatal(err)
		}
	}

	keys, err := dir.List(ctx, "mesh/a/")
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(keys, []stateapi.Key{"mesh/a/1", "mesh/a/2"}) {
		t.Fatalf("listed %v", keys)
	}

	value, err := dir.Get(ctx, "mesh/a/1")
	if err != nil || string(value.Bytes) != `{"k":"mesh/a/1"}` {
		t.Fatalf("read %q, %v", value.Bytes, err)
	}

	if err = dir.Delete(ctx, "mesh/ab/1"); err != nil {
		t.Fatal(err)
	}

	if _, err = os.Stat(filepath.Join(root, "mesh", "ab")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("the emptied folder is still there: %v", err)
	}

	if _, err = dir.Get(ctx, "mesh/ab/1"); !errors.Is(err, stateapi.ErrNoRecord) {
		t.Fatalf("a deleted record reads %v", err)
	}

	if err = dir.Delete(ctx, "mesh/ab/1"); !errors.Is(err, stateapi.ErrNoRecord) {
		t.Fatalf("deleting twice answers %v", err)
	}

	usage, err := dir.Usage(ctx)
	if err != nil || usage.Bytes == 0 || usage.Limit != Limit {
		t.Fatalf("usage %+v, %v", usage, err)
	}
}

func TestAKeyCannotLeaveTheDirectory(t *testing.T) {
	dir := New(t.TempDir())

	for _, key := range []stateapi.Key{"mesh/../../etc/passwd", "mesh/./x", "mesh//x", `mesh\x`, "mesh/.owner"} {
		if _, err := dir.Put(context.Background(), key, []byte("{}")); !errors.Is(err, stateapi.ErrKey) {
			t.Errorf("%q was written: %v", key, err)
		}
	}
}
