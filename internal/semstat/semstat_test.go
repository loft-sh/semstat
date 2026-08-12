package semstat

import (
	"strconv"
	"strings"
	"testing"
)

func TestParseAccepts(t *testing.T) {
	// Every case here is one npm `semver` also accepts. The action this tool
	// replaces used that package, so anything that validated before has to
	// validate now.
	tests := []struct {
		name string
		in   string
	}{
		{"bare", "1.2.3"},
		{"v prefixed", "v1.2.3"},
		{"zero major", "0.4.1"},
		{"prerelease", "1.2.3-rc.1"},
		{"prerelease and build", "v1.2.3-rc.1+meta"},
		{"build only", "1.2.3+build.5"},
		{"numeric prerelease identifier", "1.2.3-0.1"},
		{"large numbers", "10.20.30"},
		{"surrounding whitespace", "  v1.2.3\t"},

		// npm parity at the boundaries. Its limits are the ones every caller
		// was already living with, so they are the ones that apply.
		{"largest safe version number", "9007199254740991.0.0"},
		{"longest accepted string", "1.2.3+" + strings.Repeat("a", 250)},
		{"byte order mark is whitespace to javascript", "\uFEFF1.2.3\uFEFF"},
		// The spec puts no ceiling on a numeric prerelease identifier and npm
		// accepts this, so it parses. Ordering it correctly is Compare's job.
		{"numeric prerelease identifier past uint64", "1.0.0-99999999999999999999"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Parse(tt.in); err != nil {
				t.Fatalf("Parse(%q) = %v, want no error", tt.in, err)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	tests := []struct {
		name string
		in   string
	}{
		{"empty", ""},
		{"whitespace only", "   "},
		{"missing patch", "1.2"},
		{"missing patch with prefix", "v1.2"},
		{"major only", "1"},
		{"four segments", "1.2.3.4"},
		{"leading zero in minor", "1.02.3"},
		{"leading zero in prerelease", "1.2.3-01"},
		{"empty prerelease", "1.2.3-"},
		{"equals prefix", "=v1.2.3"},
		{"not a version", "bogus"},
		{"range expression", ">=1.2.3"},
		// A four-segment build number. Valid to some release tooling, never
		// valid semver, so it belongs to whoever cuts the tag rather than here.
		{"four-segment build number", "v1.10.1.2"},

		// npm parity at the boundaries, matching TestParseAccepts.
		{"version number above the safe integer ceiling", "9007199254740992.0.0"},
		{"one character too long", "1.2.3+" + strings.Repeat("a", 251)},
		{"NEL is whitespace to go but not to javascript", "\u00851.2.3\u0085"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if v, err := Parse(tt.in); err == nil {
				t.Fatalf("Parse(%q) = %v, want an error", tt.in, v)
			}
		})
	}
}

func TestParseErrorNamesTheInput(t *testing.T) {
	_, err := Parse("v1.2")
	if err == nil {
		t.Fatal("Parse(\"v1.2\") = nil error, want an error")
	}

	if !strings.Contains(err.Error(), `"v1.2"`) {
		t.Errorf("error %q does not quote the offending input", err)
	}
}

func TestParsed(t *testing.T) {
	str := func(s string) *string { return &s }

	tests := []struct {
		name string
		in   string
		want Parsed
	}{
		{
			name: "stable keeps the v prefix in raw",
			in:   "v1.10.1",
			want: Parsed{Major: 1, Minor: 10, Patch: 1, Raw: "v1.10.1"},
		},
		{
			name: "bare version",
			in:   "1.2.3",
			want: Parsed{Major: 1, Minor: 2, Patch: 3, Raw: "1.2.3"},
		},
		{
			name: "prerelease and build",
			in:   "v2.0.0-rc.2+abc123",
			want: Parsed{
				Major: 2, Minor: 0, Patch: 0,
				Prerelease: str("rc.2"),
				Build:      str("abc123"),
				Raw:        "v2.0.0-rc.2+abc123",
			},
		},
		{
			name: "multi-identifier prerelease is joined with dots",
			in:   "v2.0.0-next.internal.3",
			want: Parsed{
				Major: 2, Minor: 0, Patch: 0,
				Prerelease: str("next.internal.3"),
				Raw:        "v2.0.0-next.internal.3",
			},
		},
		{
			// npm echoed the caller's string back untouched, and callers read
			// raw expecting exactly what they passed in.
			name: "raw keeps surrounding whitespace",
			in:   " v1.2.3\t",
			want: Parsed{Major: 1, Minor: 2, Patch: 3, Raw: " v1.2.3\t"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.in, err)
			}

			got := v.Parsed()
			if got.Major != tt.want.Major || got.Minor != tt.want.Minor || got.Patch != tt.want.Patch {
				t.Errorf("got %d.%d.%d, want %d.%d.%d",
					got.Major, got.Minor, got.Patch, tt.want.Major, tt.want.Minor, tt.want.Patch)
			}
			if !equalStr(got.Prerelease, tt.want.Prerelease) {
				t.Errorf("prerelease = %v, want %v", show(got.Prerelease), show(tt.want.Prerelease))
			}
			if !equalStr(got.Build, tt.want.Build) {
				t.Errorf("build = %v, want %v", show(got.Build), show(tt.want.Build))
			}
			if got.Raw != tt.want.Raw {
				t.Errorf("raw = %q, want %q", got.Raw, tt.want.Raw)
			}
		})
	}
}

func TestType(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ReleaseType
	}{
		{"stable", "v1.2.3", Stable},
		{"stable with build metadata", "v1.2.3+abc", Stable},
		{"alpha", "v1.2.3-alpha.1", Alpha},
		{"beta", "v1.2.3-beta.4", Beta},
		{"rc", "v2.0.0-rc.2", RC},
		{"next", "v2.0.0-next.7", Next},
		{"next internal", "v2.0.0-next.internal.3", NextInternal},
		{"next internal wins over next", "v2.0.0-next.internal.1", NextInternal},
		{"rc with build metadata", "v2.0.0-rc.2+abc123", RC},
		// The counter does not have to be numeric to route. The vocabulary is
		// about the flavor, and a non-numeric counter is a shape real tag
		// histories contain.
		{"non-numeric alpha counter", "v1.2.3-alpha.abc123", Alpha},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.in, err)
			}

			got, err := v.Type()
			if err != nil {
				t.Fatalf("Type() = %v, want %q", err, tt.want)
			}
			if got != tt.want {
				t.Errorf("Type() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeFailsClosed(t *testing.T) {
	// Legal semver, outside the routable vocabulary. Each of these has to be an
	// error rather than a guess, or an unhandled release type gets misrouted.
	tests := []struct {
		name string
		in   string
	}{
		{"unknown flavor", "v1.2.3-preview.1"},
		{"prefixed flavor", "v1.2.3-vendor.alpha.1"},
		{"known flavor without a counter", "v1.2.3-rc"},
		{"known flavor as a bare word", "v1.2.3-next"},
		{"hyphenated rather than dotted", "v1.2.3-rc-2"},
		{"snapshot", "v1.2.3-SNAPSHOT"},

		// Suffix shapes that turn up in real tag histories. They are the reason
		// the separating dot is required rather than optional: an undotted rc1
		// is not a shape anything routes.
		{"undotted counter", "v1.2.3-rc1"},
		{"undotted counter, later line", "v1.2.3-rc2"},
		{"hyphenated test flavor", "v1.2.3-rc-test.1"},
		{"patch flavor", "v1.2.3-patch.1"},
		{"kubernetes flavor", "v1.2.3-kubernetes.115"},
		{"ci flavor", "v1.2.3-ci.6"},

		// The counter is mandatory, including for the sub-flavor. A bare
		// next.internal is next-internal with its counter left off far more
		// often than it is next counted by the word "internal", and guessing
		// either way silently is worse than stopping.
		{"sub-flavor without a counter", "v1.2.3-next.internal"},

		// The identifier count is exact. A trailing extra identifier is a shape
		// nobody defined, so reporting the flavor would route a malformed tag
		// as though it were well formed.
		{"trailing identifier after the counter", "v1.2.3-rc.1.extra"},
		{"two identifiers after the flavor", "v1.2.3-alpha.foo.bar"},
		{"trailing identifier after the sub-flavor counter", "v1.2.3-next.internal.1.extra"},
		// Only the whole identifier "internal" selects the sub-flavor, so this
		// is next with two identifiers after it, which is not a defined shape.
		{"counter resembling the sub-flavor", "v1.2.3-next.internalX.1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v, want it to parse as valid semver", tt.in, err)
			}

			got, err := v.Type()
			if err == nil {
				t.Fatalf("Type() = %q, want an error", got)
			}

			// The error is what an operator sees when a release stops, so it
			// has to name both the offending input and the way out.
			for _, want := range []string{tt.in, "-alpha.N", "-beta.N", "-rc.N", "-next.N", "-next.internal.N"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
		})
	}
}

func TestIsStable(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"v1.2.3", true},
		{"v1.2.3+build", true},
		{"v1.2.3-rc.1", false},
		{"v1.2.3-next.internal.3", false},
		// Unroutable, but still unambiguously not stable. IsStable answers for
		// versions Type refuses.
		{"v1.2.3-preview.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			v, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.in, err)
			}

			if got := v.IsStable(); got != tt.want {
				t.Errorf("IsStable() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		// `sort -V` ranks a release candidate above its own final release,
		// which is how a promotion gate ends up refusing to promote the final.
		// This is the case that has to stay fixed.
		{"release candidate sorts below its final release", "v2.0.0-rc.2", "v2.0.0", -1},
		{"final release sorts above its release candidate", "v2.0.0", "v2.0.0-rc.2", 1},
		{"alpha sorts below rc", "v2.0.0-alpha.1", "v2.0.0-rc.1", -1},
		{"beta sorts below rc", "v2.0.0-beta.9", "v2.0.0-rc.1", -1},
		{"prerelease counters compare numerically not lexically", "v2.0.0-rc.10", "v2.0.0-rc.9", 1},

		// Build metadata is excluded from precedence by the spec. `sort -V`
		// ranks on it, which is the other half of the same fragility.
		{"build metadata does not affect precedence", "1.2.3+a", "1.2.3+b", 0},
		{"build metadata does not outrank its absence", "1.2.3+build.99", "1.2.3", 0},
		{"build metadata does not rescue a prerelease", "v2.0.0-rc.2+zzz", "v2.0.0", -1},

		{"equal", "1.2.3", "1.2.3", 0},
		{"v prefix is not significant", "v1.2.3", "1.2.3", 0},
		{"patch", "1.2.4", "1.2.3", 1},
		{"minor outranks patch", "1.3.0", "1.2.99", 1},
		{"major outranks minor", "2.0.0", "1.99.99", 1},
		{"minor compares numerically not lexically", "1.10.1", "1.9.1", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.a, err)
			}
			b, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.b, err)
			}

			if got := a.Compare(b); got != tt.want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}

			// Ordering has to be symmetric, or a caller that swaps its
			// arguments gets a different answer to the same question.
			if got, want := b.Compare(a), -tt.want; got != want {
				t.Errorf("Compare(%q, %q) = %d, want %d", tt.b, tt.a, got, want)
			}
		})
	}
}

// Counters past uint64, which the dependency orders as text. Comparing digit
// counts has no ceiling, so these come out in numeric order.
func TestCompareOversizedNumericCounters(t *testing.T) {
	const maxUint64 = uint64(1<<64 - 1)
	overflow := strconv.FormatUint(maxUint64, 10) + "0"

	tests := []struct {
		name string
		a, b string
		want int
	}{
		{
			name: "more digits is the larger number",
			a:    "1.0.0-99999999999999999999",
			b:    "1.0.0-100000000000000000000",
			want: -1,
		},
		{
			name: "one past the uint64 ceiling still outranks a small counter",
			a:    "1.0.0-" + overflow,
			b:    "1.0.0-2",
			want: 1,
		},
		{
			name: "equal oversized counters",
			a:    "1.0.0-" + overflow,
			b:    "1.0.0-" + overflow,
			want: 0,
		},
		{
			name: "at the uint64 boundary",
			a:    "1.0.0-" + strconv.FormatUint(maxUint64, 10),
			b:    "1.0.0-" + strconv.FormatUint(maxUint64-1, 10),
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, err := Parse(tt.a)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.a, err)
			}
			b, err := Parse(tt.b)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", tt.b, err)
			}

			if got := a.Compare(b); got != tt.want {
				t.Errorf("Compare() = %d, want %d", got, tt.want)
			}
			if got, want := b.Compare(a), -tt.want; got != want {
				t.Errorf("reversed Compare() = %d, want %d", got, want)
			}
		})
	}
}

// Compare is our own implementation, so it has to agree with the dependency on
// everything the dependency gets right. Every combination of these components
// is checked in both directions; the only permitted divergence is a numeric
// identifier past uint64, which is what motivated writing it in the first place.
func TestCompareMatchesTheDependency(t *testing.T) {
	cores := []string{"1.2.3", "1.2.4", "1.3.0", "2.0.0", "0.0.1"}
	suffixes := []string{
		"", "-alpha", "-alpha.1", "-alpha.2", "-alpha.10", "-alpha.beta",
		"-beta", "-beta.2", "-beta.11", "-rc.1", "-rc.1.1", "-0", "-1", "-2",
		"-11", "-a", "-A", "-0a", "-x.7.z.92", "-alpha.1.2.3",
	}

	var versions []string
	for _, core := range cores {
		for _, suffix := range suffixes {
			versions = append(versions, core+suffix)
		}
	}

	compared := 0
	for _, a := range versions {
		for _, b := range versions {
			va, err := Parse(a)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", a, err)
			}
			vb, err := Parse(b)
			if err != nil {
				t.Fatalf("Parse(%q) = %v", b, err)
			}

			if got, want := va.Compare(vb), va.sv.Compare(vb.sv); got != want {
				t.Errorf("Compare(%q, %q) = %d, dependency says %d", a, b, got, want)
			}
			compared++
		}
	}

	if compared == 0 {
		t.Fatal("compared nothing")
	}
	t.Logf("agreed with the dependency on %d pairs", compared)
}

func TestString(t *testing.T) {
	v, err := Parse(" v2.0.0-rc.2 ")
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}

	// The caller's string, unaltered, same as Parsed().Raw.
	if got, want := v.String(), " v2.0.0-rc.2 "; got != want {
		t.Errorf("String() = %q, want %q", got, want)
	}
}

func equalStr(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func show(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}
