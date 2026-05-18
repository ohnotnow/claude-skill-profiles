package tui

import (
	"sort"

	"claude-skill-profiles/internal/profile"
)

// computeNewSkills returns the alphabetised list of skill names that exist on
// disk but are missing from at least one profile's YAML map. These are the
// "new" skills the refresh screen surfaces for triage.
func computeNewSkills(installed []string, profiles map[string]*profile.Profile) []string {
	var out []string
	for _, name := range installed {
		for _, p := range profiles {
			if _, ok := p.Skills[name]; !ok {
				out = append(out, name)
				break
			}
		}
	}
	sort.Strings(out)
	return out
}

// displayState reports what the refresh UI should show for a (skill, profile)
// pair. If the skill has been explicitly set in the profile, that state wins.
// Otherwise we display the refresh default (user-invocable-only) — see ant
// ADR-002: a new skill is unlikely to belong in every project, and
// user-invocable-only keeps it reachable via /skill-name without exposing it
// to autonomous skill selection.
func displayState(p *profile.Profile, skillName string) profile.State {
	if s, ok := p.Skills[skillName]; ok {
		return s
	}
	return profile.StateUserInvocable
}

// commitDefaults writes the refresh default (user-invocable-only) into every
// profile that doesn't already have an explicit entry for skillName. Returns
// the names of profiles that were touched, alphabetised. Caller is responsible
// for saving.
func commitDefaults(profiles map[string]*profile.Profile, skillName string) []string {
	var touched []string
	for name, p := range profiles {
		if _, ok := p.Skills[skillName]; !ok {
			p.Set(skillName, profile.StateUserInvocable)
			touched = append(touched, name)
		}
	}
	sort.Strings(touched)
	return touched
}
