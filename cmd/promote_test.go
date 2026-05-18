package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"claude-skill-profiles/internal/profile"
)

// writeProjectSettings writes a .claude/settings.local.json under dir with the
// given skillOverrides map, creating the directory as needed.
func writeProjectSettings(t *testing.T, dir string, overrides map[string]string) string {
	t.Helper()
	path := filepath.Join(dir, ".claude", "settings.local.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	body := map[string]any{}
	if overrides != nil {
		body["skillOverrides"] = overrides
	}
	data, err := json.MarshalIndent(body, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return path
}

func tmpStore(t *testing.T) *profile.Store {
	t.Helper()
	return &profile.Store{Dir: filepath.Join(t.TempDir(), "profiles")}
}

func TestRunPromoteCreatesProfileFromOverrides(t *testing.T) {
	dir := t.TempDir()
	projectPath := writeProjectSettings(t, dir, map[string]string{
		"alpha": "off",
		"beta":  "name-only",
	})
	store := tmpStore(t)

	var out bytes.Buffer
	err := runPromote(&out, store, projectPath, []string{"alpha", "beta", "gamma"}, "myproj", false)
	if err != nil {
		t.Fatalf("runPromote: %v", err)
	}

	got, err := store.Load("myproj")
	if err != nil {
		t.Fatalf("loading promoted profile: %v", err)
	}
	if s := got.Get("alpha"); s != profile.StateOff {
		t.Errorf("alpha: got %v, want off", s)
	}
	if s := got.Get("beta"); s != profile.StateNameOnly {
		t.Errorf("beta: got %v, want name-only", s)
	}
	if s := got.Get("gamma"); s != profile.StateEnabled {
		t.Errorf("gamma: got %v, want enabled (unspecified -> default)", s)
	}
	if !strings.Contains(out.String(), "Promoted") {
		t.Errorf("expected confirmation in output, got: %s", out.String())
	}
}

func TestRunPromoteRefusesIfProfileExists(t *testing.T) {
	dir := t.TempDir()
	projectPath := writeProjectSettings(t, dir, map[string]string{"alpha": "off"})
	store := tmpStore(t)

	// Pre-seed a profile under the same name.
	existing := profile.New()
	existing.Set("alpha", profile.StateEnabled)
	if err := store.Save("myproj", existing, false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var out bytes.Buffer
	err := runPromote(&out, store, projectPath, []string{"alpha"}, "myproj", false)
	if err == nil {
		t.Fatal("expected error when profile exists, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error should mention existing profile, got: %v", err)
	}

	// The existing profile should be untouched.
	reloaded, _ := store.Load("myproj")
	if s := reloaded.Get("alpha"); s != profile.StateEnabled {
		t.Errorf("existing profile mutated by refused promote: alpha=%v", s)
	}
}

func TestRunPromoteForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	projectPath := writeProjectSettings(t, dir, map[string]string{"alpha": "off"})
	store := tmpStore(t)

	existing := profile.New()
	existing.Set("alpha", profile.StateEnabled)
	if err := store.Save("myproj", existing, false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var out bytes.Buffer
	if err := runPromote(&out, store, projectPath, []string{"alpha"}, "myproj", true); err != nil {
		t.Fatalf("runPromote --force: %v", err)
	}
	reloaded, _ := store.Load("myproj")
	if s := reloaded.Get("alpha"); s != profile.StateOff {
		t.Errorf("after force-promote, alpha: got %v, want off", s)
	}
}

func TestRunPromoteRefusesWhenNothingToPromote(t *testing.T) {
	dir := t.TempDir()
	// Project file with no skillOverrides at all.
	projectPath := writeProjectSettings(t, dir, nil)
	store := tmpStore(t)

	var out bytes.Buffer
	err := runPromote(&out, store, projectPath, []string{"alpha"}, "myproj", false)
	if err == nil {
		t.Fatal("expected error when no overrides present, got nil")
	}
	if !strings.Contains(err.Error(), "nothing to promote") {
		t.Errorf("error should explain the empty-overrides case, got: %v", err)
	}
	if _, loadErr := store.Load("myproj"); !errors.Is(loadErr, profile.ErrNotFound) {
		t.Errorf("no profile should have been written on refusal")
	}
}

func TestRunPromoteRefusesWhenProjectFileMissing(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json") // never created
	store := tmpStore(t)

	var out bytes.Buffer
	err := runPromote(&out, store, projectPath, []string{"alpha"}, "myproj", false)
	if err == nil {
		t.Fatal("expected error when project file missing, got nil")
	}
	if !strings.Contains(err.Error(), "nothing to promote") {
		t.Errorf("missing file should report nothing-to-promote, got: %v", err)
	}
}

func TestRunPromoteRejectsInvalidName(t *testing.T) {
	dir := t.TempDir()
	projectPath := writeProjectSettings(t, dir, map[string]string{"alpha": "off"})
	store := tmpStore(t)

	var out bytes.Buffer
	err := runPromote(&out, store, projectPath, []string{"alpha"}, "../escape", false)
	if err == nil {
		t.Fatal("expected error for invalid name, got nil")
	}
	if !strings.Contains(err.Error(), "invalid profile name") {
		t.Errorf("expected validation message, got: %v", err)
	}
}

func TestRunPromoteDropsOverridesForUnknownSkills(t *testing.T) {
	// An override for a skill the user no longer has installed should not
	// pollute the new profile (matches SeedFromOverrides' contract and keeps
	// auto-prune semantics consistent).
	dir := t.TempDir()
	projectPath := writeProjectSettings(t, dir, map[string]string{
		"alpha":      "off",
		"ghost-skill": "name-only",
	})
	store := tmpStore(t)

	var out bytes.Buffer
	if err := runPromote(&out, store, projectPath, []string{"alpha"}, "myproj", false); err != nil {
		t.Fatalf("runPromote: %v", err)
	}
	got, err := store.Load("myproj")
	if err != nil {
		t.Fatalf("loading promoted profile: %v", err)
	}
	if _, present := got.Skills["ghost-skill"]; present {
		t.Errorf("override for uninstalled skill leaked into profile: %v", got.Skills)
	}
	if s := got.Get("alpha"); s != profile.StateOff {
		t.Errorf("alpha: got %v, want off", s)
	}
}
