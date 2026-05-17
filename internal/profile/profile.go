// Package profile defines the on-disk representation of a csp profile —
// a named mapping from skill name to exposure state.
package profile

import (
	"bytes"
	"fmt"
	"io"
	"sort"

	"gopkg.in/yaml.v3"
)

// State is one of the four Claude Code skill exposure states.
type State string

const (
	StateEnabled       State = "enabled"
	StateNameOnly      State = "name-only"
	StateUserInvocable State = "user-invocable-only"
	StateOff           State = "off"
)

// AllStates lists every valid State in the order they're presented in the UI:
// enabled (1), name-only (2), user-invocable-only (3), off (4).
var AllStates = []State{StateEnabled, StateNameOnly, StateUserInvocable, StateOff}

// ParseState converts a string into a State, returning an error for unknown values.
func ParseState(s string) (State, error) {
	for _, v := range AllStates {
		if string(v) == s {
			return v, nil
		}
	}
	return "", fmt.Errorf("unknown skill state %q (want one of: enabled, name-only, user-invocable-only, off)", s)
}

// Valid reports whether s is one of the four known states.
func (s State) Valid() bool {
	_, err := ParseState(string(s))
	return err == nil
}

// Profile is the in-memory form of a csp profile YAML file.
//
// Skills maps a skill name (the directory name under ~/.claude/skills/) to the
// state the user wants that skill to be in when the profile is applied. A skill
// absent from the map is treated as StateEnabled — Claude Code's default.
type Profile struct {
	Skills map[string]State `yaml:"skills"`
}

// New returns an empty profile with an initialised skills map.
func New() *Profile {
	return &Profile{Skills: map[string]State{}}
}

// Get returns the state of skill in this profile. Skills not present in the
// map default to StateEnabled.
func (p *Profile) Get(skill string) State {
	if s, ok := p.Skills[skill]; ok {
		return s
	}
	return StateEnabled
}

// Set sets the state of skill, creating the map if needed.
func (p *Profile) Set(skill string, s State) {
	if p.Skills == nil {
		p.Skills = map[string]State{}
	}
	p.Skills[skill] = s
}

// ToSkillOverrides converts the profile into the shape Claude Code expects in
// settings.local.json under the "skillOverrides" key.
//
// Skills set to StateEnabled are omitted: enabled is the default, and Claude
// Code's skillOverrides only lists non-default states.
func (p *Profile) ToSkillOverrides() map[string]string {
	out := map[string]string{}
	for skill, s := range p.Skills {
		if s == StateEnabled {
			continue
		}
		out[skill] = string(s)
	}
	return out
}

// Marshal writes the profile to w as YAML, with skills sorted alphabetically so
// diffs stay stable.
func (p *Profile) Marshal(w io.Writer) error {
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	defer enc.Close()
	return enc.Encode(sortedView{p})
}

// MarshalBytes is a convenience wrapper around Marshal that returns the YAML
// bytes directly.
func (p *Profile) MarshalBytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := p.Marshal(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal reads YAML from r into a new Profile. Unknown state values produce
// a clear error.
func Unmarshal(r io.Reader) (*Profile, error) {
	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)
	var p Profile
	if err := dec.Decode(&p); err != nil && err != io.EOF {
		return nil, err
	}
	if p.Skills == nil {
		p.Skills = map[string]State{}
	}
	for skill, s := range p.Skills {
		if _, err := ParseState(string(s)); err != nil {
			return nil, fmt.Errorf("skill %q: %w", skill, err)
		}
	}
	return &p, nil
}

// UnmarshalBytes is a convenience wrapper around Unmarshal.
func UnmarshalBytes(b []byte) (*Profile, error) {
	return Unmarshal(bytes.NewReader(b))
}

// --- internal helpers ---

// sortedView is a yaml.Marshaler that emits the skills map in alphabetical
// order. yaml.v3 maps default to unordered output; we want stable diffs.
type sortedView struct{ p *Profile }

func (s sortedView) MarshalYAML() (any, error) {
	node := &yaml.Node{Kind: yaml.MappingNode}
	skillsNode := &yaml.Node{Kind: yaml.MappingNode}

	keys := make([]string, 0, len(s.p.Skills))
	for k := range s.p.Skills {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		skillsNode.Content = append(skillsNode.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: k, Tag: "!!str"},
			&yaml.Node{Kind: yaml.ScalarNode, Value: string(s.p.Skills[k]), Tag: "!!str", Style: yaml.DoubleQuotedStyle},
		)
	}

	node.Content = []*yaml.Node{
		{Kind: yaml.ScalarNode, Value: "skills", Tag: "!!str"},
		skillsNode,
	}
	return node, nil
}

