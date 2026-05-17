package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyToMissingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.local.json")

	overrides := map[string]string{
		"docker-fleet": "off",
		"flux-ui":      "name-only",
	}
	if err := ApplySkillOverrides(path, overrides); err != nil {
		t.Fatalf("ApplySkillOverrides: %v", err)
	}

	got, err := ReadSkillOverrides(path)
	if err != nil {
		t.Fatalf("ReadSkillOverrides: %v", err)
	}
	if got["docker-fleet"] != "off" || got["flux-ui"] != "name-only" {
		t.Errorf("round-trip mismatch: %v", got)
	}
}

func TestApplyPreservesUnrelatedKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")

	initial := `{
  "permissions": {
    "allow": ["Bash(ls:*)"]
  },
  "voiceEnabled": true,
  "skillOverrides": {
    "old-skill": "off"
  }
}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ApplySkillOverrides(path, map[string]string{"new-skill": "name-only"}); err != nil {
		t.Fatalf("ApplySkillOverrides: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, data)
	}
	if _, ok := raw["permissions"]; !ok {
		t.Error("permissions key missing after apply")
	}
	if raw["voiceEnabled"] != true {
		t.Errorf("voiceEnabled changed: %v", raw["voiceEnabled"])
	}
	// skillOverrides should have been replaced wholesale.
	ov, _ := raw["skillOverrides"].(map[string]any)
	if _, gone := ov["old-skill"]; gone {
		t.Error("old-skill should have been removed by replace semantics")
	}
	if ov["new-skill"] != "name-only" {
		t.Errorf("new-skill: want name-only, got %v", ov["new-skill"])
	}
}

func TestApplyEmptyRemovesKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")

	initial := `{"skillOverrides": {"docker-fleet": "off"}, "voiceEnabled": true}`
	if err := os.WriteFile(path, []byte(initial), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ApplySkillOverrides(path, map[string]string{}); err != nil {
		t.Fatalf("ApplySkillOverrides: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, present := raw["skillOverrides"]; present {
		t.Error("skillOverrides should have been removed when overrides is empty")
	}
	if raw["voiceEnabled"] != true {
		t.Error("voiceEnabled should still be present")
	}
}

func TestReadSkillOverridesAbsent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.json")

	got, err := ReadSkillOverrides(path)
	if err != nil {
		t.Fatalf("missing file should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty map, got %d entries", len(got))
	}
}

func TestOutputIsIndented(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.local.json")

	if err := ApplySkillOverrides(path, map[string]string{"a": "off", "b": "name-only"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "\n  \"") {
		t.Errorf("output should use 2-space indent; got:\n%s", data)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("output should end with a trailing newline")
	}
}
