package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverMissingDir(t *testing.T) {
	got, err := Discover(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("missing dir should not error, got: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("missing dir should return empty slice, got %d entries", len(got))
	}
}

func TestDiscoverOrdersAlphabetically(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"zebra", "alpha", "mango"} {
		if err := os.Mkdir(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("want %d skills, got %d", len(want), len(got))
	}
	for i, s := range got {
		if s.Name != want[i] {
			t.Errorf("position %d: want %q, got %q", i, want[i], s.Name)
		}
	}
}

func TestDiscoverSkipsFilesAndDotDirs(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "real"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, ".hidden"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("not a skill"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "real" {
		t.Errorf("want only 'real', got %+v", got)
	}
}

func TestDiscoverReadsDescriptionFromFrontmatter(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "fancy-skill")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `---
name: fancy-skill
description: A one-line summary of the skill.
---

# Skill body
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 skill, got %d", len(got))
	}
	if got[0].Description != "A one-line summary of the skill." {
		t.Errorf("description: got %q", got[0].Description)
	}
}

func TestDiscoverHandlesMissingSkillMd(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "bare"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := Discover(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Description != "" {
		t.Errorf("want 1 skill with empty description, got %+v", got)
	}
}

func TestExtractFrontmatter(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{"basic", "---\nfoo: bar\n---\nbody", "foo: bar", true},
		{"no delim", "no frontmatter here\n", "", false},
		{"open but no close", "---\nfoo: bar\nnothing more", "", false},
		{"multiline", "---\nname: x\ndescription: y\n---\n# body\n", "name: x\ndescription: y", true},
	}
	for _, tc := range cases {
		got, ok := extractFrontmatter([]byte(tc.in))
		if ok != tc.found {
			t.Errorf("%s: ok=%v, want %v", tc.name, ok, tc.found)
		}
		if string(got) != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}
