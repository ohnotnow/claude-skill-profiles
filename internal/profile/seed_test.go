package profile

import "testing"

func TestSeedFromOverridesAppliesKnownOverrides(t *testing.T) {
	p := SeedFromOverrides(
		[]string{"ait", "flux-ui", "docker-fleet"},
		map[string]string{
			"flux-ui":      "name-only",
			"docker-fleet": "off",
		},
	)
	if got := p.Get("ait"); got != StateEnabled {
		t.Errorf("ait: want enabled (default), got %q", got)
	}
	if got := p.Get("flux-ui"); got != StateNameOnly {
		t.Errorf("flux-ui: want name-only, got %q", got)
	}
	if got := p.Get("docker-fleet"); got != StateOff {
		t.Errorf("docker-fleet: want off, got %q", got)
	}
}

func TestSeedFromOverridesDropsUndiscoveredSkills(t *testing.T) {
	p := SeedFromOverrides(
		[]string{"ait"},
		map[string]string{"uninstalled-skill": "off"},
	)
	if _, present := p.Skills["uninstalled-skill"]; present {
		t.Error("uninstalled skill should not appear in seeded profile")
	}
	if len(p.Skills) != 1 {
		t.Errorf("want 1 skill (ait), got %d", len(p.Skills))
	}
}

func TestSeedFromOverridesIgnoresUnknownStates(t *testing.T) {
	p := SeedFromOverrides(
		[]string{"ait"},
		map[string]string{"ait": "bogus-state"},
	)
	if got := p.Get("ait"); got != StateEnabled {
		t.Errorf("ait should fall back to enabled when override state is invalid, got %q", got)
	}
}

func TestSeedFromOverridesEmptyOverrides(t *testing.T) {
	p := SeedFromOverrides([]string{"ait", "ant"}, nil)
	if p.Get("ait") != StateEnabled || p.Get("ant") != StateEnabled {
		t.Error("with no overrides, all skills should be enabled")
	}
}
