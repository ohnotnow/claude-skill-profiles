package profile

import (
	"errors"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	return &Store{Dir: filepath.Join(t.TempDir(), "profiles")}
}

func TestValidateName(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"laravel", true},
		{"go-lang", true},
		{"sys_admin", true},
		{"profile1", true},
		{"", false},
		{"-leading-dash", false},
		{"has space", false},
		{"has/slash", false},
		{".dotfile", false},
		{"weird!chars", false},
	}
	for _, tc := range cases {
		err := ValidateName(tc.in)
		got := err == nil
		if got != tc.want {
			t.Errorf("ValidateName(%q): want valid=%v, got valid=%v (err=%v)", tc.in, tc.want, got, err)
		}
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	s := newTestStore(t)
	p := New()
	p.Set("flux-ui", StateNameOnly)
	p.Set("docker-fleet", StateOff)

	if err := s.Save("laravel", p, false); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := s.Load("laravel")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Get("flux-ui") != StateNameOnly {
		t.Errorf("flux-ui: got %q", got.Get("flux-ui"))
	}
	if got.Get("docker-fleet") != StateOff {
		t.Errorf("docker-fleet: got %q", got.Get("docker-fleet"))
	}
}

func TestLoadMissingReturnsErrNotFound(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Load("nonesuch")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestSaveRefusesOverwriteWithoutFlag(t *testing.T) {
	s := newTestStore(t)
	p := New()
	p.Set("ait", StateOff)

	if err := s.Save("existing", p, false); err != nil {
		t.Fatal(err)
	}
	err := s.Save("existing", p, false)
	if !errors.Is(err, ErrExists) {
		t.Errorf("want ErrExists, got %v", err)
	}
	// Overwrite=true should succeed.
	if err := s.Save("existing", p, true); err != nil {
		t.Errorf("overwrite=true should succeed, got %v", err)
	}
}

func TestListAlphabetisesAndStripsExtension(t *testing.T) {
	s := newTestStore(t)
	p := New()
	for _, name := range []string{"zebra", "alpha", "mango"} {
		if err := s.Save(name, p, false); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alpha", "mango", "zebra"}
	if len(got) != len(want) {
		t.Fatalf("want %v, got %v", want, got)
	}
	for i, n := range want {
		if got[i] != n {
			t.Errorf("position %d: want %q, got %q", i, n, got[i])
		}
	}
}

func TestListMissingDirIsEmpty(t *testing.T) {
	s := &Store{Dir: filepath.Join(t.TempDir(), "does-not-exist")}
	got, err := s.List()
	if err != nil {
		t.Fatalf("missing dir should not error, got %v", err)
	}
	if len(got) != 0 {
		t.Errorf("want empty list, got %v", got)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)
	p := New()
	if err := s.Save("tmp", p, false); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("tmp"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := s.Delete("tmp"); !errors.Is(err, ErrNotFound) {
		t.Errorf("second delete: want ErrNotFound, got %v", err)
	}
}

func TestSaveRejectsInvalidName(t *testing.T) {
	s := newTestStore(t)
	if err := s.Save("../etc/passwd", New(), false); err == nil {
		t.Error("want error for invalid name")
	}
}
