package registry

import "testing"

func TestParseSemver(t *testing.T) {
	tests := []struct {
		input   string
		want    Semver
		wantErr bool
	}{
		{"1.2.3", Semver{1, 2, 3, ""}, false},
		{"v1.2.3", Semver{1, 2, 3, ""}, false},
		{"0.0.1", Semver{0, 0, 1, ""}, false},
		{"1.2.3-beta", Semver{1, 2, 3, "beta"}, false},
		{"v10.20.30-rc.1", Semver{10, 20, 30, "rc.1"}, false},
		{"1.2", Semver{}, true},
		{"abc", Semver{}, true},
		{"1.x.3", Semver{}, true},
		{"1.2.x", Semver{}, true},
		{"x.2.3", Semver{}, true},
	}
	for _, tt := range tests {
		got, err := ParseSemver(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSemver(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseSemver(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestSemverString(t *testing.T) {
	tests := []struct {
		v    Semver
		want string
	}{
		{Semver{1, 2, 3, ""}, "1.2.3"},
		{Semver{0, 0, 0, ""}, "0.0.0"},
		{Semver{1, 0, 0, "beta"}, "1.0.0-beta"},
	}
	for _, tt := range tests {
		if got := tt.v.String(); got != tt.want {
			t.Errorf("%v.String() = %q, want %q", tt.v, got, tt.want)
		}
	}
}

func TestSemverCompare(t *testing.T) {
	tests := []struct {
		a, b Semver
		want int
	}{
		{Semver{1, 0, 0, ""}, Semver{1, 0, 0, ""}, 0},
		{Semver{2, 0, 0, ""}, Semver{1, 0, 0, ""}, 1},
		{Semver{1, 0, 0, ""}, Semver{2, 0, 0, ""}, -1},
		{Semver{1, 2, 0, ""}, Semver{1, 1, 0, ""}, 1},
		{Semver{1, 1, 0, ""}, Semver{1, 2, 0, ""}, -1},
		{Semver{1, 0, 2, ""}, Semver{1, 0, 1, ""}, 1},
		{Semver{1, 0, 1, ""}, Semver{1, 0, 2, ""}, -1},
		// Pre-release has lower precedence
		{Semver{1, 0, 0, ""}, Semver{1, 0, 0, "beta"}, 1},
		{Semver{1, 0, 0, "beta"}, Semver{1, 0, 0, ""}, -1},
		{Semver{1, 0, 0, "alpha"}, Semver{1, 0, 0, "beta"}, -1},
		{Semver{1, 0, 0, "beta"}, Semver{1, 0, 0, "alpha"}, 1},
		{Semver{1, 0, 0, "rc"}, Semver{1, 0, 0, "rc"}, 0},
	}
	for _, tt := range tests {
		if got := tt.a.Compare(tt.b); got != tt.want {
			t.Errorf("%v.Compare(%v) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestMatchesConstraint(t *testing.T) {
	v := func(s string) Semver { v, _ := ParseSemver(s); return v }

	tests := []struct {
		version    string
		constraint string
		want       bool
	}{
		// Empty / wildcard
		{"1.0.0", "", true},
		{"1.0.0", "*", true},
		{"1.0.0", "  ", true},
		// Exact
		{"1.2.3", "1.2.3", true},
		{"1.2.3", "1.2.4", false},
		// >=
		{"1.2.3", ">=1.0.0", true},
		{"1.0.0", ">=1.0.0", true},
		{"0.9.0", ">=1.0.0", false},
		// <=
		{"1.0.0", "<=1.0.0", true},
		{"0.9.0", "<=1.0.0", true},
		{"1.0.1", "<=1.0.0", false},
		// >
		{"1.0.1", ">1.0.0", true},
		{"1.0.0", ">1.0.0", false},
		// <
		{"0.9.0", "<1.0.0", true},
		{"1.0.0", "<1.0.0", false},
		// ^ (caret — same major)
		{"1.5.0", "^1.2.0", true},
		{"1.2.0", "^1.2.0", true},
		{"1.1.0", "^1.2.0", false},
		{"2.0.0", "^1.2.0", false},
		// ~ (tilde — same major+minor)
		{"1.2.5", "~1.2.0", true},
		{"1.2.0", "~1.2.0", true},
		{"1.3.0", "~1.2.0", false},
		{"1.1.0", "~1.2.0", false},
		// Invalid constraint
		{"1.0.0", ">=invalid", false},
		{"1.0.0", "<=invalid", false},
		{"1.0.0", ">invalid", false},
		{"1.0.0", "<invalid", false},
		{"1.0.0", "^invalid", false},
		{"1.0.0", "~invalid", false},
		{"1.0.0", "invalid", false},
	}
	for _, tt := range tests {
		got := MatchesConstraint(v(tt.version), tt.constraint)
		if got != tt.want {
			t.Errorf("MatchesConstraint(%s, %q) = %v, want %v", tt.version, tt.constraint, got, tt.want)
		}
	}
}

func TestFindBestTag(t *testing.T) {
	tags := []string{"v1.0.0", "v1.1.0", "v2.0.0", "v0.9.0", "not-semver", "v1.2.0-beta"}

	// No constraint — latest
	tag, _, _ := FindBestTag(tags, "")
	if tag != "v2.0.0" {
		t.Errorf("got %q, want v2.0.0", tag)
	}

	// Caret constraint — 1.2.0-beta > 1.1.0 (higher minor)
	tag, _, _ = FindBestTag(tags, "^1.0.0")
	if tag != "v1.2.0-beta" {
		t.Errorf("got %q, want v1.2.0-beta", tag)
	}

	// Tilde constraint — only 1.1.x
	tag, _, _ = FindBestTag(tags, "~1.1.0")
	if tag != "v1.1.0" {
		t.Errorf("got %q, want v1.1.0", tag)
	}

	// Exact
	tag, _, _ = FindBestTag(tags, "0.9.0")
	if tag != "v0.9.0" {
		t.Errorf("got %q, want v0.9.0", tag)
	}

	// No match
	tag, _, _ = FindBestTag(tags, ">=3.0.0")
	if tag != "" {
		t.Errorf("got %q, want empty", tag)
	}

	// Empty list
	tag, _, _ = FindBestTag(nil, "")
	if tag != "" {
		t.Errorf("got %q, want empty", tag)
	}

	// All invalid
	tag, _, _ = FindBestTag([]string{"abc", "def"}, "")
	if tag != "" {
		t.Errorf("got %q, want empty", tag)
	}
}
