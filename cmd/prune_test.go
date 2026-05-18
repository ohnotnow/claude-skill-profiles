package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-skill-profiles/internal/profile"
)

func tmpStoreWithProfiles(t *testing.T, profiles map[string]map[string]profile.State) *profile.Store {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "profiles")
	store := &profile.Store{Dir: dir}
	for name, entries := range profiles {
		p := profile.New()
		for k, v := range entries {
			p.Set(k, v)
		}
		if err := store.Save(name, p, false); err != nil {
			t.Fatalf("seeding profile %q: %v", name, err)
		}
	}
	return store
}

func TestRunPruneNothingToPrune(t *testing.T) {
	store := tmpStoreWithProfiles(t, map[string]map[string]profile.State{
		"laravel": {"ait": profile.StateOff, "flux-ui": profile.StateNameOnly},
	})
	var out bytes.Buffer
	if err := runPrune(&out, store, []string{"ait", "flux-ui"}, false); err != nil {
		t.Fatalf("runPrune: %v", err)
	}
	if !strings.Contains(out.String(), "Nothing to prune") {
		t.Errorf("expected nothing-to-prune message, got: %s", out.String())
	}
}

func TestRunPruneRemovesAndReports(t *testing.T) {
	store := tmpStoreWithProfiles(t, map[string]map[string]profile.State{
		"laravel": {"ait": profile.StateOff, "ghost": profile.StateOff},
		"golang":  {"ait": profile.StateOff, "phantom": profile.StateOff, "old-skill": profile.StateNameOnly},
	})
	var out bytes.Buffer
	if err := runPrune(&out, store, []string{"ait"}, false); err != nil {
		t.Fatalf("runPrune: %v", err)
	}
	s := out.String()
	for _, want := range []string{"Pruned 3", "laravel: ghost", "golang: old-skill, phantom"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}

	// Verify the writes actually happened.
	laravel, _ := store.Load("laravel")
	if _, ok := laravel.Skills["ghost"]; ok {
		t.Error("ghost should be gone from laravel after prune")
	}
	golang, _ := store.Load("golang")
	if _, ok := golang.Skills["phantom"]; ok {
		t.Error("phantom should be gone from golang after prune")
	}
}

func TestRunPruneDryRunMakesNoWrites(t *testing.T) {
	store := tmpStoreWithProfiles(t, map[string]map[string]profile.State{
		"laravel": {"ait": profile.StateOff, "ghost": profile.StateOff},
	})
	path := filepath.Join(store.Dir, "laravel.yaml")
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := runPrune(&out, store, []string{"ait"}, true); err != nil {
		t.Fatalf("runPrune dry-run: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Would prune") || !strings.Contains(s, "ghost") {
		t.Errorf("dry-run output should describe the change, got: %s", s)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("dry-run rewrote the file (mtime changed)")
	}
	// Confirm the stale entry still lives on disk.
	reloaded, _ := store.Load("laravel")
	if _, ok := reloaded.Skills["ghost"]; !ok {
		t.Error("dry-run should not have removed ghost from disk")
	}
}

func TestRunPruneIsIdempotent(t *testing.T) {
	store := tmpStoreWithProfiles(t, map[string]map[string]profile.State{
		"laravel": {"ait": profile.StateOff, "ghost": profile.StateOff},
	})

	var first bytes.Buffer
	if err := runPrune(&first, store, []string{"ait"}, false); err != nil {
		t.Fatalf("first runPrune: %v", err)
	}
	if !strings.Contains(first.String(), "Pruned 1") {
		t.Errorf("first run should report 1 pruned, got: %s", first.String())
	}

	var second bytes.Buffer
	if err := runPrune(&second, store, []string{"ait"}, false); err != nil {
		t.Fatalf("second runPrune: %v", err)
	}
	if !strings.Contains(second.String(), "Nothing to prune") {
		t.Errorf("second run should be a no-op, got: %s", second.String())
	}
}

func TestRunPruneEmptyStore(t *testing.T) {
	store := &profile.Store{Dir: filepath.Join(t.TempDir(), "empty")}
	var out bytes.Buffer
	if err := runPrune(&out, store, []string{"ait"}, false); err != nil {
		t.Fatalf("runPrune: %v", err)
	}
	if !strings.Contains(out.String(), "Nothing to prune") {
		t.Errorf("empty store should report nothing-to-prune, got: %s", out.String())
	}
}
