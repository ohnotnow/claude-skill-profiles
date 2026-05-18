package profile

import (
	"fmt"
	"sort"
)

// Prune drops every entry in p.Skills whose key isn't in known, returning the
// names that were removed (alphabetised). The profile is mutated in place;
// callers that care about the original state must clone first.
func (p *Profile) Prune(known []string) []string {
	if len(p.Skills) == 0 {
		return nil
	}
	keep := make(map[string]bool, len(known))
	for _, n := range known {
		keep[n] = true
	}
	var removed []string
	for name := range p.Skills {
		if !keep[name] {
			removed = append(removed, name)
			delete(p.Skills, name)
		}
	}
	sort.Strings(removed)
	return removed
}

// PruneAll loads every profile in s, prunes each against known, and saves only
// those that actually changed. The returned map has an entry for each profile
// that lost at least one skill, keyed by profile name, with the removed skill
// names as the value. Profiles with nothing to prune are silently skipped.
//
// Profiles that fail to load (corrupt YAML, unreadable file) are skipped: the
// error is recorded in the returned errs map but the function continues so a
// single bad profile doesn't block reconciliation of the rest. A nil errs map
// is returned when every profile loaded cleanly.
func PruneAll(s *Store, known []string) (removed map[string][]string, errs map[string]error) {
	names, err := s.List()
	if err != nil {
		return nil, map[string]error{"": fmt.Errorf("listing profiles: %w", err)}
	}
	for _, name := range names {
		p, err := s.Load(name)
		if err != nil {
			if errs == nil {
				errs = map[string]error{}
			}
			errs[name] = err
			continue
		}
		gone := p.Prune(known)
		if len(gone) == 0 {
			continue
		}
		if err := s.Save(name, p, true); err != nil {
			if errs == nil {
				errs = map[string]error{}
			}
			errs[name] = fmt.Errorf("saving pruned profile: %w", err)
			continue
		}
		if removed == nil {
			removed = map[string][]string{}
		}
		removed[name] = gone
	}
	return removed, errs
}
