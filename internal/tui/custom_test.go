package tui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"claude-skill-profiles/internal/profile"
	"claude-skill-profiles/internal/settings"
)

// writeJSON marshals v and writes it to path, creating parent dirs as needed.
func writeJSON(t *testing.T, path string, v any) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestBuildCustomProfile_UsesProjectOverridesWhenPresent(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")
	globalPath := filepath.Join(dir, "global.json")

	writeJSON(t, projectPath, map[string]any{
		"skillOverrides": map[string]string{
			"alpha": "off",
			"beta":  "name-only",
		},
	})
	writeJSON(t, globalPath, map[string]any{
		"skillOverrides": map[string]string{
			"alpha": "enabled",
			"beta":  "enabled",
			"gamma": "user-invocable-only",
		},
	})

	p, err := buildCustomProfile([]string{"alpha", "beta", "gamma"}, projectPath, globalPath)
	if err != nil {
		t.Fatalf("buildCustomProfile: %v", err)
	}
	if got, want := p.Get("alpha"), profile.StateOff; got != want {
		t.Errorf("alpha: got %v, want %v (project should win over global)", got, want)
	}
	if got, want := p.Get("beta"), profile.StateNameOnly; got != want {
		t.Errorf("beta: got %v, want %v", got, want)
	}
	if got, want := p.Get("gamma"), profile.StateEnabled; got != want {
		t.Errorf("gamma: got %v, want %v (global must NOT leak when project is non-empty)", got, want)
	}
}

func TestBuildCustomProfile_FallsBackToGlobalWhenNoProjectFile(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")
	globalPath := filepath.Join(dir, "global.json")

	// No project file — only global exists.
	writeJSON(t, globalPath, map[string]any{
		"skillOverrides": map[string]string{
			"alpha": "off",
			"beta":  "user-invocable-only",
		},
	})

	p, err := buildCustomProfile([]string{"alpha", "beta", "gamma"}, projectPath, globalPath)
	if err != nil {
		t.Fatalf("buildCustomProfile: %v", err)
	}
	if got, want := p.Get("alpha"), profile.StateOff; got != want {
		t.Errorf("alpha: got %v, want %v", got, want)
	}
	if got, want := p.Get("beta"), profile.StateUserInvocable; got != want {
		t.Errorf("beta: got %v, want %v", got, want)
	}
	if got, want := p.Get("gamma"), profile.StateEnabled; got != want {
		t.Errorf("gamma: got %v, want %v (unspecified should default to enabled)", got, want)
	}
}

func TestBuildCustomProfile_FallsBackWhenProjectHasEmptyOverrides(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")
	globalPath := filepath.Join(dir, "global.json")

	// Project file exists but has no skillOverrides key at all.
	writeJSON(t, projectPath, map[string]any{
		"someOtherKey": "value",
	})
	writeJSON(t, globalPath, map[string]any{
		"skillOverrides": map[string]string{
			"alpha": "off",
		},
	})

	p, err := buildCustomProfile([]string{"alpha"}, projectPath, globalPath)
	if err != nil {
		t.Fatalf("buildCustomProfile: %v", err)
	}
	if got, want := p.Get("alpha"), profile.StateOff; got != want {
		t.Errorf("alpha: got %v, want %v (global fallback should fire on empty project overrides)", got, want)
	}
}

func TestBuildCustomProfile_EmptyEverywhereDefaultsToEnabled(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")
	globalPath := filepath.Join(dir, "missing.json") // doesn't exist

	p, err := buildCustomProfile([]string{"alpha", "beta"}, projectPath, globalPath)
	if err != nil {
		t.Fatalf("buildCustomProfile: %v", err)
	}
	for _, name := range []string{"alpha", "beta"} {
		if got, want := p.Get(name), profile.StateEnabled; got != want {
			t.Errorf("%s: got %v, want %v", name, got, want)
		}
	}
}

func TestBuildCustomProfile_EmptyGlobalPathTolerated(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")

	p, err := buildCustomProfile([]string{"alpha"}, projectPath, "")
	if err != nil {
		t.Fatalf("buildCustomProfile: %v", err)
	}
	if got, want := p.Get("alpha"), profile.StateEnabled; got != want {
		t.Errorf("alpha: got %v, want %v", got, want)
	}
}

func TestEnsureProjectFile_CreatesWhenMissing(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")

	p := profile.New()
	p.Set("alpha", profile.StateOff)
	p.Set("beta", profile.StateEnabled) // should be dropped (default)

	if err := ensureProjectFile(projectPath, p); err != nil {
		t.Fatalf("ensureProjectFile: %v", err)
	}

	got, err := settings.ReadSkillOverrides(projectPath)
	if err != nil {
		t.Fatalf("ReadSkillOverrides: %v", err)
	}
	if got["alpha"] != "off" {
		t.Errorf("alpha: got %q, want off", got["alpha"])
	}
	if _, present := got["beta"]; present {
		t.Errorf("beta (enabled, default) should not be written")
	}
}

func TestEnsureProjectFile_LeavesExistingFileAlone(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")

	// Pre-existing file with content that doesn't match the profile.
	writeJSON(t, projectPath, map[string]any{
		"skillOverrides": map[string]string{"alpha": "name-only"},
		"someOtherKey":   "preserved",
	})
	infoBefore, err := os.Stat(projectPath)
	if err != nil {
		t.Fatal(err)
	}

	p := profile.New()
	p.Set("alpha", profile.StateOff)
	p.Set("beta", profile.StateOff)

	if err := ensureProjectFile(projectPath, p); err != nil {
		t.Fatalf("ensureProjectFile: %v", err)
	}

	infoAfter, err := os.Stat(projectPath)
	if err != nil {
		t.Fatal(err)
	}
	if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
		t.Error("existing file was rewritten (mtime changed); ensureProjectFile should be a no-op when the file exists")
	}

	got, err := settings.ReadSkillOverrides(projectPath)
	if err != nil {
		t.Fatalf("ReadSkillOverrides: %v", err)
	}
	if got["alpha"] != "name-only" {
		t.Errorf("alpha clobbered: got %q, want name-only", got["alpha"])
	}
	if _, present := got["beta"]; present {
		t.Errorf("beta was added to existing file by ensureProjectFile")
	}
}

// TestCustomSaveRoundTrip exercises the save side: the callback that
// customModel installs is just settings.ApplySkillOverrides over
// profile.ToSkillOverrides. This test verifies the round-trip behaves the way
// custom-mode users will see it — write happens, .claude/ is created on
// demand, and non-default states land in the file while enabled states drop.
func TestCustomSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, ".claude", "settings.local.json")

	p := profile.New()
	p.Set("alpha", profile.StateOff)
	p.Set("beta", profile.StateEnabled)
	p.Set("gamma", profile.StateNameOnly)

	if err := settings.ApplySkillOverrides(projectPath, p.ToSkillOverrides()); err != nil {
		t.Fatalf("ApplySkillOverrides: %v", err)
	}

	// The .claude/ directory should have been created.
	if _, err := os.Stat(filepath.Join(dir, ".claude")); err != nil {
		t.Fatalf(".claude/ not created on first save: %v", err)
	}

	got, err := settings.ReadSkillOverrides(projectPath)
	if err != nil {
		t.Fatalf("ReadSkillOverrides: %v", err)
	}
	want := map[string]string{
		"alpha": "off",
		"gamma": "name-only",
	}
	if len(got) != len(want) {
		t.Errorf("override count: got %d, want %d (got=%v)", len(got), len(want), got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
	if _, present := got["beta"]; present {
		t.Errorf("beta should be omitted (enabled is the default)")
	}
}
