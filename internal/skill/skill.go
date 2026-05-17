// Package skill discovers Claude Code skills installed under ~/.claude/skills/.
//
// A "skill" is a directory whose name is what Claude Code's skillOverrides
// uses as the key. The directory may contain a SKILL.md with YAML frontmatter;
// when present, the frontmatter's "description" field gives us a one-line
// summary suitable for display in the TUI.
//
// Plugin-provided skills are explicitly out of scope here (see ant ADR-001).
package skill

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Skill is one discovered skill.
type Skill struct {
	// Name is the directory name under ~/.claude/skills/. This is the value
	// that appears as a key in Claude Code's skillOverrides.
	Name string

	// Description is the one-line summary from SKILL.md's frontmatter, or "" if
	// no SKILL.md was found or the frontmatter was unparseable.
	Description string

	// Path is the absolute path to the skill's directory.
	Path string
}

// DefaultDir returns the default location for user skills: ~/.claude/skills/.
//
// If $HOME cannot be resolved, it returns the literal path "~/.claude/skills"
// which will fail at the next filesystem call with a clear error.
func DefaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "~/.claude/skills"
	}
	return filepath.Join(home, ".claude", "skills")
}

// Discover walks root and returns every skill directory found at the top
// level, alphabetised by name. A missing root directory is not an error — it
// returns an empty slice so callers can show a friendly empty state.
func Discover(root string) ([]Skill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", root, err)
	}

	skills := make([]Skill, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(root, name)
		s := Skill{Name: name, Path: path}
		if desc, ok := readDescription(path); ok {
			s.Description = desc
		}
		skills = append(skills, s)
	}

	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}

// readDescription returns the "description" field from skillDir/SKILL.md's
// YAML frontmatter, or ok=false if it isn't available.
func readDescription(skillDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(skillDir, "SKILL.md"))
	if err != nil {
		return "", false
	}

	fm, ok := extractFrontmatter(data)
	if !ok {
		return "", false
	}

	var meta struct {
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal(fm, &meta); err != nil {
		return "", false
	}

	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		return "", false
	}
	return desc, true
}

// extractFrontmatter pulls the bytes between the first two "---" lines of a
// markdown file. Returns ok=false if no frontmatter block is present.
func extractFrontmatter(data []byte) ([]byte, bool) {
	const delim = "---"
	// File must start with --- (optionally preceded by a BOM, but skip that
	// for now; SKILL.md files in the wild don't seem to use one).
	if !bytes.HasPrefix(data, []byte(delim)) {
		return nil, false
	}
	// Skip the opening delimiter line.
	rest := data[len(delim):]
	if i := bytes.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	} else {
		return nil, false
	}
	// Find the closing delimiter line.
	closingIdx := bytes.Index(rest, []byte("\n"+delim))
	if closingIdx < 0 {
		return nil, false
	}
	return rest[:closingIdx], true
}
