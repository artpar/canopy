package registry

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSemver parses a version string like "1.2.3" or "v1.2.3" into a Semver.
func ParseSemver(s string) (Semver, error) {
	s = strings.TrimPrefix(s, "v")
	// Split off pre-release
	pre := ""
	if i := strings.Index(s, "-"); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Semver{}, fmt.Errorf("invalid semver: %q", s)
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid major: %q", parts[0])
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid minor: %q", parts[1])
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Semver{}, fmt.Errorf("invalid patch: %q", parts[2])
	}
	return Semver{Major: major, Minor: minor, Patch: patch, Pre: pre}, nil
}

// String returns the semver as "major.minor.patch[-pre]".
func (v Semver) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Compare returns -1, 0, or 1.
func (v Semver) Compare(other Semver) int {
	if v.Major != other.Major {
		return cmpInt(v.Major, other.Major)
	}
	if v.Minor != other.Minor {
		return cmpInt(v.Minor, other.Minor)
	}
	if v.Patch != other.Patch {
		return cmpInt(v.Patch, other.Patch)
	}
	// Pre-release versions have lower precedence than release
	if v.Pre == "" && other.Pre != "" {
		return 1
	}
	if v.Pre != "" && other.Pre == "" {
		return -1
	}
	if v.Pre < other.Pre {
		return -1
	}
	if v.Pre > other.Pre {
		return 1
	}
	return 0
}

func cmpInt(a, b int) int {
	if a < b {
		return -1
	}
	return 1
}

// MatchesConstraint checks if a version satisfies a constraint string.
// Supported formats: ">=1.0.0", "^1.2.0", "~1.2.0", "1.2.3" (exact), "*".
func MatchesConstraint(version Semver, constraint string) bool {
	constraint = strings.TrimSpace(constraint)
	if constraint == "" || constraint == "*" {
		return true
	}

	if strings.HasPrefix(constraint, ">=") {
		c, err := ParseSemver(constraint[2:])
		if err != nil {
			return false
		}
		return version.Compare(c) >= 0
	}
	if strings.HasPrefix(constraint, "<=") {
		c, err := ParseSemver(constraint[2:])
		if err != nil {
			return false
		}
		return version.Compare(c) <= 0
	}
	if strings.HasPrefix(constraint, ">") {
		c, err := ParseSemver(constraint[1:])
		if err != nil {
			return false
		}
		return version.Compare(c) > 0
	}
	if strings.HasPrefix(constraint, "<") {
		c, err := ParseSemver(constraint[1:])
		if err != nil {
			return false
		}
		return version.Compare(c) < 0
	}
	if strings.HasPrefix(constraint, "^") {
		// Caret: >=version, <next major
		c, err := ParseSemver(constraint[1:])
		if err != nil {
			return false
		}
		if version.Compare(c) < 0 {
			return false
		}
		return version.Major == c.Major
	}
	if strings.HasPrefix(constraint, "~") {
		// Tilde: >=version, <next minor
		c, err := ParseSemver(constraint[1:])
		if err != nil {
			return false
		}
		if version.Compare(c) < 0 {
			return false
		}
		return version.Major == c.Major && version.Minor == c.Minor
	}

	// Exact match
	c, err := ParseSemver(constraint)
	if err != nil {
		return false
	}
	return version.Compare(c) == 0
}

// FindBestTag finds the highest semver tag that matches a constraint from a list of tag names.
// If constraint is empty, returns the highest version. Returns ("", "") if no match.
func FindBestTag(tagNames []string, constraint string) (tag string, commit string, tagList []TagVersion) {
	for _, name := range tagNames {
		v, err := ParseSemver(name)
		if err != nil {
			continue
		}
		tagList = append(tagList, TagVersion{Tag: name, Version: v})
	}

	var best *TagVersion
	for i := range tagList {
		tv := &tagList[i]
		if constraint != "" && !MatchesConstraint(tv.Version, constraint) {
			continue
		}
		if best == nil || tv.Version.Compare(best.Version) > 0 {
			best = tv
		}
	}
	if best == nil {
		return "", "", tagList
	}
	return best.Tag, "", tagList
}

// TagVersion pairs a tag name with its parsed semver.
type TagVersion struct {
	Tag     string
	Version Semver
}
