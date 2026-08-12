// Package semstat reports facts about semantic version strings. It parses,
// classifies and orders versions; it never produces a new one.
package semstat

import (
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/Masterminds/semver/v3"
)

// ReleaseType is the kind of release a version represents.
type ReleaseType string

// The prerelease flavors the vCluster release dispatcher can route, not the set
// semver allows. Anything outside it is an error rather than a sixth category, so
// an unroutable release cannot be misrouted.
const (
	Alpha        ReleaseType = "alpha"
	Beta         ReleaseType = "beta"
	RC           ReleaseType = "rc"
	Next         ReleaseType = "next"
	NextInternal ReleaseType = "next-internal"
	Stable       ReleaseType = "stable"
)

// supportedSuffixes is the operator-facing description of the vocabulary above.
// It is part of the error message for an unrecognised suffix, so it has to name
// every accepted shape.
const supportedSuffixes = `-alpha.N, -beta.N, -rc.N, -next.N or -next.internal.N, or no suffix at all for a stable release`

// Version is a parsed, valid semantic version, plus the string it was parsed
// from. The raw form is kept because callers pass v-prefixed tags and expect to
// see them echoed back unchanged.
type Version struct {
	sv  *semver.Version
	raw string
}

// Parsed is the flat view of a Version, and the shape of `semstat parse`
// output. Prerelease and Build are pointers so that their absence marshals to
// JSON null rather than an empty string.
type Parsed struct {
	Major      uint64  `json:"major"`
	Minor      uint64  `json:"minor"`
	Patch      uint64  `json:"patch"`
	Prerelease *string `json:"prerelease"`
	Build      *string `json:"build"`
	Raw        string  `json:"raw"`
}

// Limits carried over from the npm `semver` package, which decided validity for
// every caller before this tool existed.
const (
	// maxLength is npm's cap, measured in UTF-16 code units and applied to the
	// untrimmed input.
	maxLength = 256
	// maxSafeInteger is JavaScript's Number.MAX_SAFE_INTEGER. npm rejects a
	// major, minor or patch above it because it could not represent one.
	maxSafeInteger = 1<<53 - 1
)

// Parse reads a semantic version, with or without a leading "v".
//
// Surrounding whitespace is trimmed before parsing, and the version must be
// complete: "1.2" is rejected where "1.2.3" is accepted. Validity matches the
// npm `semver` package this tool replaces, down to its length and integer
// ceilings, so a version that validated before still validates now.
func Parse(s string) (*Version, error) {
	if utf16Len(s) > maxLength {
		return nil, fmt.Errorf("version string is longer than %d characters", maxLength)
	}

	trimmed := trimECMAScript(s)
	if trimmed == "" {
		return nil, errors.New("version string is empty")
	}

	sv, err := semver.StrictNewVersion(strings.TrimPrefix(trimmed, "v"))
	if err != nil {
		return nil, fmt.Errorf("%q is not a valid semantic version: %w", trimmed, err)
	}

	if sv.Major() > maxSafeInteger || sv.Minor() > maxSafeInteger || sv.Patch() > maxSafeInteger {
		return nil, fmt.Errorf("%q has a version number above the maximum of %d", trimmed, maxSafeInteger)
	}

	// Untrimmed: callers expect their own string echoed back verbatim.
	return &Version{sv: sv, raw: s}, nil
}

// trimECMAScript trims what JavaScript's String.prototype.trim removes, which
// is not what strings.TrimSpace removes: JavaScript strips the byte order mark
// and leaves the NEL control character, and Go does the opposite.
func trimECMAScript(s string) string {
	return strings.TrimFunc(s, func(r rune) bool {
		switch r {
		case '\u0085': // NEL, whitespace to Go but not to JavaScript
			return false
		case '\uFEFF': // BOM, whitespace to JavaScript but not to Go
			return true
		}
		return unicode.IsSpace(r)
	})
}

// utf16Len counts UTF-16 code units, which is what a JavaScript string length
// counts and therefore what npm's limit is measured in.
func utf16Len(s string) int {
	n := 0
	for _, r := range s {
		n++
		if r > 0xFFFF {
			n++ // encoded as a surrogate pair
		}
	}
	return n
}

// Parsed returns the flat view of v.
func (v *Version) Parsed() Parsed {
	p := Parsed{
		Major: v.sv.Major(),
		Minor: v.sv.Minor(),
		Patch: v.sv.Patch(),
		Raw:   v.raw,
	}
	if pre := v.sv.Prerelease(); pre != "" {
		p.Prerelease = &pre
	}
	if build := v.sv.Metadata(); build != "" {
		p.Build = &build
	}
	return p
}

// Type classifies v's prerelease suffix. It fails closed: a suffix that is
// legal semver but outside the vocabulary is an error, not a guess.
//
// Matching is on whole dot-separated identifiers, not prefixes, which is what
// makes the counter mandatory. A prefix match would read the "internal" in a bare
// "next.internal" as next's counter, when it is far more likely next-internal with
// the counter left off.
func (v *Version) Type() (ReleaseType, error) {
	pre := v.sv.Prerelease()
	if pre == "" {
		return Stable, nil
	}

	unsupported := fmt.Errorf("version %q has an unsupported prerelease suffix %q; supported are %s",
		v.raw, pre, supportedSuffixes)

	// The identifier count is exact, not a minimum. A trailing extra identifier
	// means the tag was cut to a shape nobody defined, and reporting the flavor
	// anyway would route it as though it were well formed.
	ids := strings.Split(pre, ".")

	// Order matters: next.internal is a sub-flavor of next and has to be
	// matched before it.
	switch {
	case ids[0] == "next" && len(ids) > 1 && ids[1] == "internal":
		if len(ids) != 3 {
			return "", unsupported
		}
		return NextInternal, nil
	case ids[0] == "next", ids[0] == "alpha", ids[0] == "beta", ids[0] == "rc":
		if len(ids) != 2 {
			return "", unsupported
		}
		return ReleaseType(ids[0]), nil
	}

	return "", unsupported
}

// IsStable reports whether v carries no prerelease suffix. Unlike Type it does
// not care whether the suffix is one we recognise, so it answers for any valid
// version.
func (v *Version) IsStable() bool {
	return v.sv.Prerelease() == ""
}

// Compare returns -1, 0 or 1 as v sorts below, equal to, or above other.
// Build metadata is ignored, as the semver spec requires.
//
// Precedence is implemented here rather than delegated because the dependency
// parses numeric prerelease identifiers as uint64 and compares them as text once
// one overflows, ranking a 20-digit counter above a 21-digit one. Comparing digit
// counts has no ceiling. TestCompareMatchesTheDependency pins the rest to agree.
func (v *Version) Compare(other *Version) int {
	if c := compareUint(v.sv.Major(), other.sv.Major()); c != 0 {
		return c
	}
	if c := compareUint(v.sv.Minor(), other.sv.Minor()); c != 0 {
		return c
	}
	if c := compareUint(v.sv.Patch(), other.sv.Patch()); c != 0 {
		return c
	}

	return comparePrerelease(v.sv.Prerelease(), other.sv.Prerelease())
}

// comparePrerelease implements the precedence rules for the prerelease field.
func comparePrerelease(a, b string) int {
	switch {
	case a == "" && b == "":
		return 0
	case a == "":
		return 1 // a version without a prerelease outranks one with it
	case b == "":
		return -1
	}

	aIDs, bIDs := strings.Split(a, "."), strings.Split(b, ".")

	for i := 0; i < len(aIDs) && i < len(bIDs); i++ {
		if c := compareIdentifier(aIDs[i], bIDs[i]); c != 0 {
			return c
		}
	}

	// Equal as far as the shorter one goes, so the longer one wins.
	return compareInt(len(aIDs), len(bIDs))
}

func compareIdentifier(a, b string) int {
	aNumeric, bNumeric := isNumeric(a), isNumeric(b)

	switch {
	case aNumeric && bNumeric:
		// Compared as numbers of any size. A leading zero is already invalid
		// semver, so the identifier with more digits is the larger number and
		// equal-length identifiers compare bytewise.
		if c := compareInt(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case aNumeric:
		return -1 // numeric identifiers rank below alphanumeric ones
	case bNumeric:
		return 1
	}

	return strings.Compare(a, b)
}

func isNumeric(s string) bool {
	return s != "" && strings.TrimLeft(s, "0123456789") == ""
}

func compareUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// String returns the version as it was given, including any "v" prefix.
func (v *Version) String() string { return v.raw }
