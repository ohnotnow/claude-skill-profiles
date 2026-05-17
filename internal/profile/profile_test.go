package profile

import (
	"strings"
	"testing"
)

func TestParseState(t *testing.T) {
	cases := []struct {
		in      string
		want    State
		wantErr bool
	}{
		{"enabled", StateEnabled, false},
		{"name-only", StateNameOnly, false},
		{"user-invocable-only", StateUserInvocable, false},
		{"off", StateOff, false},
		{"", "", true},
		{"on", "", true},
		{"disabled", "", true},
		{"ENABLED", "", true}, // case-sensitive on purpose; matches Claude Code's values
	}
	for _, tc := range cases {
		got, err := ParseState(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Errorf("ParseState(%q): want error, got %q", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseState(%q): unexpected error: %v", tc.in, err)
		}
		if got != tc.want {
			t.Errorf("ParseState(%q): want %q, got %q", tc.in, tc.want, got)
		}
	}
}

func TestProfileGetDefault(t *testing.T) {
	p := New()
	if got := p.Get("never-touched"); got != StateEnabled {
		t.Errorf("Get on absent skill: want enabled (default), got %q", got)
	}
}

func TestProfileSetThenGet(t *testing.T) {
	p := New()
	p.Set("flux-ui", StateNameOnly)
	if got := p.Get("flux-ui"); got != StateNameOnly {
		t.Errorf("Get after Set: want name-only, got %q", got)
	}
}

func TestRoundTrip(t *testing.T) {
	p := New()
	p.Set("ait", StateEnabled)
	p.Set("flux-ui", StateNameOnly)
	p.Set("docker-fleet", StateOff)
	p.Set("purchase-order", StateUserInvocable)

	b, err := p.MarshalBytes()
	if err != nil {
		t.Fatalf("MarshalBytes: %v", err)
	}

	p2, err := UnmarshalBytes(b)
	if err != nil {
		t.Fatalf("UnmarshalBytes: %v", err)
	}
	for _, skill := range []string{"ait", "flux-ui", "docker-fleet", "purchase-order"} {
		if p.Get(skill) != p2.Get(skill) {
			t.Errorf("round-trip changed %s: %q -> %q", skill, p.Get(skill), p2.Get(skill))
		}
	}
}

func TestMarshalIsAlphabetised(t *testing.T) {
	p := New()
	// Insert in non-alphabetical order — output should still be sorted.
	p.Set("zebra", StateOff)
	p.Set("alpha", StateOff)
	p.Set("mango", StateOff)

	b, err := p.MarshalBytes()
	if err != nil {
		t.Fatalf("MarshalBytes: %v", err)
	}
	out := string(b)
	ai := strings.Index(out, "alpha")
	mi := strings.Index(out, "mango")
	zi := strings.Index(out, "zebra")
	if !(ai < mi && mi < zi) {
		t.Errorf("expected alphabetical order, got:\n%s", out)
	}
}

func TestUnmarshalRejectsUnknownState(t *testing.T) {
	in := []byte("skills:\n  ait: bogus\n")
	_, err := UnmarshalBytes(in)
	if err == nil {
		t.Fatal("want error for unknown state, got nil")
	}
	if !strings.Contains(err.Error(), "ait") {
		t.Errorf("error should mention the offending skill name, got: %v", err)
	}
}

func TestUnmarshalEmpty(t *testing.T) {
	p, err := UnmarshalBytes([]byte(""))
	if err != nil {
		t.Fatalf("empty input should be OK, got: %v", err)
	}
	if len(p.Skills) != 0 {
		t.Errorf("empty input should produce empty profile, got %d skills", len(p.Skills))
	}
}

func TestToSkillOverridesOmitsEnabled(t *testing.T) {
	p := New()
	p.Set("ait", StateEnabled)
	p.Set("flux-ui", StateEnabled)
	p.Set("docker-fleet", StateOff)
	p.Set("purchase-order", StateUserInvocable)

	got := p.ToSkillOverrides()

	if _, ok := got["ait"]; ok {
		t.Error("enabled skill should not appear in overrides")
	}
	if _, ok := got["flux-ui"]; ok {
		t.Error("enabled skill should not appear in overrides")
	}
	if got["docker-fleet"] != "off" {
		t.Errorf("docker-fleet: want off, got %q", got["docker-fleet"])
	}
	if got["purchase-order"] != "user-invocable-only" {
		t.Errorf("purchase-order: want user-invocable-only, got %q", got["purchase-order"])
	}
	if len(got) != 2 {
		t.Errorf("want 2 entries in overrides, got %d: %v", len(got), got)
	}
}
