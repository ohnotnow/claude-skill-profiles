// Package settings reads and writes a project's .claude/settings.local.json,
// touching only the "skillOverrides" key and preserving everything else
// verbatim.
//
// The on-disk format is JSON with 2-space indentation, matching Claude Code's
// own output. Top-level key order is not guaranteed to be preserved — Go's
// encoding/json marshals maps in alphabetical order. This is acceptable for
// v1: the first apply may reorder keys, but subsequent applies will be stable.
package settings

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Default returns the conventional path: <cwd>/.claude/settings.local.json.
func Default() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, ".claude", "settings.local.json"), nil
}

// ReadSkillOverrides returns the existing skillOverrides map from path, or an
// empty map if the file or key is missing.
func ReadSkillOverrides(path string) (map[string]string, error) {
	raw, err := readRaw(path)
	if err != nil {
		return nil, err
	}
	return extractSkillOverrides(raw)
}

// ApplySkillOverrides writes overrides to path under the "skillOverrides" key,
// replacing whatever was there before. All other top-level keys in the file
// are preserved untouched.
//
// If path does not exist, it (and its parent directory) are created. If
// overrides is empty, the key is removed from the file entirely rather than
// left as an empty object.
func ApplySkillOverrides(path string, overrides map[string]string) error {
	raw, err := readRaw(path)
	if err != nil {
		return err
	}

	if len(overrides) == 0 {
		delete(raw, "skillOverrides")
	} else {
		// json.Marshal sorts map keys alphabetically; round-tripping through
		// RawMessage lets the outer MarshalIndent indent it consistently.
		ovBytes, err := json.Marshal(overrides)
		if err != nil {
			return fmt.Errorf("encoding skillOverrides: %w", err)
		}
		raw["skillOverrides"] = ovBytes
	}

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("encoding settings: %w", err)
	}
	out = append(out, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// --- internal ---

// readRaw reads path as a JSON object of RawMessage values. A missing file is
// not an error — it returns an empty map.
func readRaw(path string) (map[string]json.RawMessage, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]json.RawMessage{}, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}
	if len(data) == 0 {
		return map[string]json.RawMessage{}, nil
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if raw == nil {
		raw = map[string]json.RawMessage{}
	}
	return raw, nil
}

// extractSkillOverrides decodes the "skillOverrides" key into a map, returning
// an empty map if absent.
func extractSkillOverrides(raw map[string]json.RawMessage) (map[string]string, error) {
	v, ok := raw["skillOverrides"]
	if !ok {
		return map[string]string{}, nil
	}
	var out map[string]string
	if err := json.Unmarshal(v, &out); err != nil {
		return nil, fmt.Errorf("parsing skillOverrides: %w", err)
	}
	if out == nil {
		out = map[string]string{}
	}
	return out, nil
}

