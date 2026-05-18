package tui

import "claude-skill-profiles/internal/profile"

// stateRank returns the position of s in profile.AllStates. Used as the sort
// key for "sort by state" views.
func stateRank(s profile.State) int {
	switch s {
	case profile.StateEnabled:
		return 0
	case profile.StateNameOnly:
		return 1
	case profile.StateUserInvocable:
		return 2
	case profile.StateOff:
		return 3
	}
	return 99
}

// stepState returns the state dir steps from s through profile.AllStates,
// wrapping at both ends. +1 cycles forward, -1 back.
func stepState(s profile.State, dir int) profile.State {
	states := profile.AllStates
	cur := 0
	for i, st := range states {
		if st == s {
			cur = i
			break
		}
	}
	n := (cur + dir + len(states)) % len(states)
	return states[n]
}
