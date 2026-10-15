package repo

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSyncConfigRoundTrip(t *testing.T) {
	dir := t.TempDir()
	r := NewSyncConfigRepository(dir)

	ctx := context.Background()
	// absent file → (nil, nil) so the caller can apply defaults
	got, err := r.Load(ctx)
	if err != nil || got != nil {
		t.Fatalf("Load absent = %v, %v; want nil, nil", got, err)
	}

	want := &SyncConfig{
		RemoteURL: "https://github.com/u/cf-backup.git",
		Branch:    "dev",
		Triggers: SyncTriggers{
			OnProgress:    true,
			IntervalMin:   30,
			OnStartupPull: true,
		},
		LastSync: "2026-09-27T12:00:00Z",
		Enabled:  true,
		Exclude:  []string{"draft-course", "sandbox"},
	}
	if err := r.Save(ctx, want); err != nil {
		t.Fatal(err)
	}
	got, err = r.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, want %+v", got, want)
	}

	// overwrite replaces the whole document
	next := &SyncConfig{RemoteURL: "https://github.com/u/other.git", Branch: "main"}
	if err := r.Save(ctx, next); err != nil {
		t.Fatal(err)
	}
	got, err = r.Load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.RemoteURL != next.RemoteURL || got.Enabled || len(got.Exclude) != 0 {
		t.Fatalf("after overwrite = %+v", got)
	}

	// a fresh instance over the same dataDir sees the saved document
	if _, err := os.Stat(filepath.Join(dir, "sync_config.json")); err != nil {
		t.Fatalf("sync_config.json not created: %v", err)
	}
	fresh := NewSyncConfigRepository(dir)
	got, err = fresh.Load(ctx)
	if err != nil || got.RemoteURL != next.RemoteURL {
		t.Fatalf("fresh repo Load = %+v, %v", got, err)
	}
}

func TestSyncConfigConcurrentSave(t *testing.T) {
	r := NewSyncConfigRepository(t.TempDir())
	ctx := context.Background()
	errs := make(chan error, 8)
	for i := range 8 {
		go func() {
			errs <- r.Save(ctx, &SyncConfig{RemoteURL: "u", Branch: "main", Enabled: i%2 == 0})
		}()
	}
	for range 8 {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if _, err := r.Load(ctx); err != nil {
		t.Fatal(err)
	}
}
