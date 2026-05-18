package tui

import (
	"reflect"
	"testing"

	"claude-skill-profiles/internal/profile"
)

func mkProfile(entries map[string]profile.State) *profile.Profile {
	p := profile.New()
	for k, v := range entries {
		p.Set(k, v)
	}
	return p
}

func TestComputeNewSkillsEmpty(t *testing.T) {
	got := computeNewSkills(nil, map[string]*profile.Profile{
		"laravel": mkProfile(nil),
	})
	if len(got) != 0 {
		t.Errorf("want empty, got %v", got)
	}
}

func TestComputeNewSkillsAllPresent(t *testing.T) {
	installed := []string{"ait", "flux-ui"}
	profiles := map[string]*profile.Profile{
		"laravel": mkProfile(map[string]profile.State{"ait": profile.StateOff, "flux-ui": profile.StateEnabled}),
		"golang":  mkProfile(map[string]profile.State{"ait": profile.StateOff, "flux-ui": profile.StateOff}),
	}
	got := computeNewSkills(installed, profiles)
	if len(got) != 0 {
		t.Errorf("want empty when every profile has every skill, got %v", got)
	}
}

func TestComputeNewSkillsMissingFromOne(t *testing.T) {
	installed := []string{"ait", "amazing-1", "amazing-2"}
	profiles := map[string]*profile.Profile{
		"laravel": mkProfile(map[string]profile.State{"ait": profile.StateOff}),
		"golang":  mkProfile(map[string]profile.State{"ait": profile.StateOff, "amazing-1": profile.StateOff}),
	}
	got := computeNewSkills(installed, profiles)
	want := []string{"amazing-1", "amazing-2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestComputeNewSkillsAlphabetised(t *testing.T) {
	installed := []string{"zebra", "alpha", "mango"}
	profiles := map[string]*profile.Profile{
		"laravel": mkProfile(nil), // empty profile, every skill is new
	}
	got := computeNewSkills(installed, profiles)
	want := []string{"alpha", "mango", "zebra"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("want %v, got %v", want, got)
	}
}

func TestDisplayStateFallsBackToUserInvocable(t *testing.T) {
	p := mkProfile(map[string]profile.State{"explicit": profile.StateOff})
	if got := displayState(p, "explicit"); got != profile.StateOff {
		t.Errorf("explicit entry: want off, got %q", got)
	}
	if got := displayState(p, "untouched"); got != profile.StateUserInvocable {
		t.Errorf("missing entry: want user-invocable-only, got %q", got)
	}
}

func TestCommitDefaultsOnlyFillsMissing(t *testing.T) {
	profiles := map[string]*profile.Profile{
		"laravel": mkProfile(map[string]profile.State{"amazing-1": profile.StateOff}), // already explicit
		"golang":  mkProfile(nil),
		"ml":      mkProfile(nil),
	}
	touched := commitDefaults(profiles, "amazing-1")
	want := []string{"golang", "ml"}
	if !reflect.DeepEqual(touched, want) {
		t.Errorf("touched: want %v, got %v", want, touched)
	}
	if profiles["laravel"].Get("amazing-1") != profile.StateOff {
		t.Error("laravel's explicit entry should not have been overwritten")
	}
	if profiles["golang"].Get("amazing-1") != profile.StateUserInvocable {
		t.Errorf("golang should default to user-invocable-only, got %q", profiles["golang"].Get("amazing-1"))
	}
	if profiles["ml"].Get("amazing-1") != profile.StateUserInvocable {
		t.Errorf("ml should default to user-invocable-only, got %q", profiles["ml"].Get("amazing-1"))
	}
}
