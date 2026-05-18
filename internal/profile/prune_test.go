package profile

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPrune(t *testing.T) {
	cases := []struct {
		name        string
		entries     map[string]State
		known       []string
		wantRemoved []string
		wantRemain  []string
	}{
		{
			name:        "no entries",
			entries:     nil,
			known:       []string{"ait", "flux-ui"},
			wantRemoved: nil,
			wantRemain:  nil,
		},
		{
			name:        "everything matches",
			entries:     map[string]State{"ait": StateOff, "flux-ui": StateNameOnly},
			known:       []string{"ait", "flux-ui"},
			wantRemoved: nil,
			wantRemain:  []string{"ait", "flux-ui"},
		},
		{
			name:        "one stale entry",
			entries:     map[string]State{"ait": StateOff, "ghost": StateOff},
			known:       []string{"ait"},
			wantRemoved: []string{"ghost"},
			wantRemain:  []string{"ait"},
		},
		{
			name:        "multiple stale entries returned alphabetised",
			entries:     map[string]State{"zebra": StateOff, "alpha": StateOff, "kept": StateNameOnly},
			known:       []string{"kept"},
			wantRemoved: []string{"alpha", "zebra"},
			wantRemain:  []string{"kept"},
		},
		{
			name:        "empty known list drops everything",
			entries:     map[string]State{"a": StateOff, "b": StateOff},
			known:       nil,
			wantRemoved: []string{"a", "b"},
			wantRemain:  nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := New()
			for k, v := range tc.entries {
				p.Set(k, v)
			}
			got := p.Prune(tc.known)
			if !reflect.DeepEqual(got, tc.wantRemoved) {
				t.Errorf("removed: want %v, got %v", tc.wantRemoved, got)
			}
			for _, name := range tc.wantRemain {
				if _, ok := p.Skills[name]; !ok {
					t.Errorf("expected %q to remain, but it was removed", name)
				}
			}
			if len(p.Skills) != len(tc.wantRemain) {
				t.Errorf("skills remaining: want %d (%v), got %d (%v)",
					len(tc.wantRemain), tc.wantRemain, len(p.Skills), p.Skills)
			}
		})
	}
}

func TestPruneAllSkipsUnchangedProfiles(t *testing.T) {
	s := newTestStore(t)

	clean := New()
	clean.Set("ait", StateOff)
	if err := s.Save("clean", clean, false); err != nil {
		t.Fatal(err)
	}
	dirty := New()
	dirty.Set("ait", StateOff)
	dirty.Set("ghost", StateOff)
	if err := s.Save("dirty", dirty, false); err != nil {
		t.Fatal(err)
	}

	// Capture mtime of the clean profile so we can assert it wasn't rewritten.
	cleanPath := filepath.Join(s.Dir, "clean.yaml")
	before, err := os.Stat(cleanPath)
	if err != nil {
		t.Fatal(err)
	}

	removed, errs := PruneAll(s, []string{"ait"})
	if errs != nil {
		t.Fatalf("unexpected errs: %v", errs)
	}
	if len(removed) != 1 {
		t.Fatalf("want 1 changed profile, got %d: %v", len(removed), removed)
	}
	if !reflect.DeepEqual(removed["dirty"], []string{"ghost"}) {
		t.Errorf("dirty: want [ghost], got %v", removed["dirty"])
	}

	after, err := os.Stat(cleanPath)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Errorf("clean profile was rewritten (mtime changed) — PruneAll should skip unchanged profiles")
	}

	reloaded, err := s.Load("dirty")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := reloaded.Skills["ghost"]; ok {
		t.Error("ghost should be gone from dirty after PruneAll")
	}
}

func TestPruneAllNoProfiles(t *testing.T) {
	s := newTestStore(t)
	removed, errs := PruneAll(s, []string{"ait"})
	if errs != nil {
		t.Fatalf("unexpected errs: %v", errs)
	}
	if len(removed) != 0 {
		t.Errorf("want no changes, got %v", removed)
	}
}

func TestPruneAllRecordsLoadErrors(t *testing.T) {
	s := newTestStore(t)
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A bogus YAML file with an unknown state value triggers a parse failure.
	bad := filepath.Join(s.Dir, "broken.yaml")
	if err := os.WriteFile(bad, []byte("skills:\n  ait: bogus\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	good := New()
	good.Set("ait", StateOff)
	good.Set("ghost", StateOff)
	if err := s.Save("good", good, false); err != nil {
		t.Fatal(err)
	}

	removed, errs := PruneAll(s, []string{"ait"})
	if errs == nil || errs["broken"] == nil {
		t.Fatalf("want errs[\"broken\"] to be set, got %v", errs)
	}
	if !reflect.DeepEqual(removed["good"], []string{"ghost"}) {
		t.Errorf("good profile should still have been pruned, got %v", removed["good"])
	}
}
