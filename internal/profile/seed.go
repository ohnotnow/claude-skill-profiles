package profile

// SeedFromOverrides builds a fresh profile from a list of discovered skill
// names plus a "skillOverrides"-shaped map (skill name -> state string).
//
// Every discovered skill starts at StateEnabled. The overrides are then
// applied on top, but only for skills that were actually discovered locally —
// entries referring to skills the user doesn't have installed are silently
// dropped. Unknown state values in the overrides map are also dropped (rather
// than turning into an error: best-effort seeding).
//
// This is how `csp new` and the TUI's "n" flow start a profile from the
// user's current global Claude Code config, instead of a blank slate.
func SeedFromOverrides(skillNames []string, overrides map[string]string) *Profile {
	p := New()
	discovered := make(map[string]bool, len(skillNames))
	for _, n := range skillNames {
		p.Set(n, StateEnabled)
		discovered[n] = true
	}
	for skillName, stateStr := range overrides {
		if !discovered[skillName] {
			continue
		}
		st, err := ParseState(stateStr)
		if err != nil {
			continue
		}
		p.Set(skillName, st)
	}
	return p
}
