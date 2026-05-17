package profile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Store reads and writes profiles in a directory of YAML files.
type Store struct {
	// Dir is the directory holding <name>.yaml files. Created on first write.
	Dir string
}

// DefaultDir returns the conventional profile location:
//   - $XDG_CONFIG_HOME/csp/profiles/ when XDG_CONFIG_HOME is set
//   - $HOME/.config/csp/profiles/ otherwise
func DefaultDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "csp", "profiles")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".config", "csp", "profiles")
	}
	return filepath.Join(home, ".config", "csp", "profiles")
}

// DefaultStore returns a Store pointing at DefaultDir().
func DefaultStore() *Store {
	return &Store{Dir: DefaultDir()}
}

// ErrNotFound is returned by Load when the named profile does not exist.
var ErrNotFound = errors.New("profile not found")

// ErrExists is returned by Save when called with overwrite=false and the
// profile already exists on disk.
var ErrExists = errors.New("profile already exists")

// validName matches names safe for use as filenames: alphanumeric plus '-'
// and '_'. No path separators, no leading dots.
var validName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// ValidateName reports whether name is a legal profile name.
func ValidateName(name string) error {
	if !validName.MatchString(name) {
		return fmt.Errorf("invalid profile name %q (use letters, digits, '-' or '_'; must start with a letter or digit)", name)
	}
	return nil
}

// Path returns the on-disk path for the named profile (without checking
// whether it exists).
func (s *Store) Path(name string) (string, error) {
	if err := ValidateName(name); err != nil {
		return "", err
	}
	return filepath.Join(s.Dir, name+".yaml"), nil
}

// Load returns the named profile, or ErrNotFound if no such file exists.
func (s *Store) Load(name string) (*Profile, error) {
	path, err := s.Path(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("opening %s: %w", path, err)
	}
	defer f.Close()
	p, err := Unmarshal(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	return p, nil
}

// Save writes p to disk under name. If overwrite is false and the profile
// already exists, Save returns ErrExists without touching the file.
func (s *Store) Save(name string, p *Profile, overwrite bool) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	if !overwrite {
		if _, err := os.Stat(path); err == nil {
			return ErrExists
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return fmt.Errorf("creating %s: %w", s.Dir, err)
	}
	b, err := p.MarshalBytes()
	if err != nil {
		return fmt.Errorf("encoding profile: %w", err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}
	return nil
}

// List returns the names of every profile in the store, alphabetised. A
// missing storage directory yields an empty slice, not an error.
func (s *Store) List() ([]string, error) {
	entries, err := os.ReadDir(s.Dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", s.Dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		names = append(names, strings.TrimSuffix(name, ".yaml"))
	}
	sort.Strings(names)
	return names, nil
}

// Delete removes the named profile. Returns ErrNotFound if it doesn't exist.
func (s *Store) Delete(name string) error {
	path, err := s.Path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil {
		if os.IsNotExist(err) {
			return ErrNotFound
		}
		return err
	}
	return nil
}
