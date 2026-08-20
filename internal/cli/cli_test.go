package cli

import (
	"bytes"
	"strings"
	"testing"
)

// run executes one invocation against buffers and returns the exit code plus
// what the command wrote.
func run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()

	var out, errOut bytes.Buffer
	code = Run(args, "test", &out, &errOut)
	return code, out.String(), errOut.String()
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"valid bare", "1.2.3", ExitOK},
		{"valid v prefixed", "v1.10.1", ExitOK},
		{"valid prerelease", "v2.0.0-next.internal.3", ExitOK},
		{"valid with build metadata", "1.2.3+abc", ExitOK},
		// An unroutable suffix is still valid semver. validate answers the
		// semver question only; type answers the routing one.
		{"valid but unroutable suffix", "1.2.3-preview.1", ExitOK},
		{"missing patch", "1.2", ExitNo},
		{"leading zero", "1.02.3", ExitNo},
		{"not a version", "bogus", ExitNo},
		{"empty", "", ExitNo},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, "validate", tt.in)

			if code != tt.want {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, tt.want, stderr)
			}

			// validate reports through the exit code alone so it reads as a
			// shell condition. Anything on stdout would break `x=$(...)` use.
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}

			switch tt.want {
			case ExitOK:
				if stderr != "" {
					t.Errorf("stderr = %q, want empty on a valid version", stderr)
				}
			case ExitNo:
				if stderr == "" {
					t.Error("stderr is empty, want an explanation of what is wrong")
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	// Callers index into this JSON, so key order, spacing and null-versus-empty are
	// contract. Asserting the exact bytes is the only way to pin them: unmarshalling
	// first would accept reordered keys and added fields.
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "stable",
			in:   "v1.10.1",
			want: `{"major":1,"minor":10,"patch":1,"prerelease":null,"build":null,"raw":"v1.10.1"}`,
		},
		{
			name: "prerelease and build",
			in:   "v2.0.0-rc.2+abc123",
			want: `{"major":2,"minor":0,"patch":0,"prerelease":"rc.2","build":"abc123","raw":"v2.0.0-rc.2+abc123"}`,
		},
		{
			name: "prerelease only",
			in:   "2.0.0-next.internal.3",
			want: `{"major":2,"minor":0,"patch":0,"prerelease":"next.internal.3","build":null,"raw":"2.0.0-next.internal.3"}`,
		},
		{
			name: "build only",
			in:   "1.2.3+build.5",
			want: `{"major":1,"minor":2,"patch":3,"prerelease":null,"build":"build.5","raw":"1.2.3+build.5"}`,
		},
		{
			name: "raw echoes surrounding whitespace",
			in:   " v1.2.3\t",
			want: `{"major":1,"minor":2,"patch":3,"prerelease":null,"build":null,"raw":" v1.2.3\t"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, "parse", tt.in)
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr)
			}

			if got := strings.TrimSuffix(stdout, "\n"); got != tt.want {
				t.Errorf("stdout = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseEmitsNullNotEmptyString(t *testing.T) {
	// The npm implementation emitted null for an absent prerelease or build.
	// An empty string here would be truthy in some callers and silently change
	// their branching.
	code, stdout, _ := run(t, "parse", "v1.10.1")
	if code != ExitOK {
		t.Fatalf("exit = %d, want %d", code, ExitOK)
	}

	for _, want := range []string{`"prerelease":null`, `"build":null`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout %q does not contain %s", strings.TrimSpace(stdout), want)
		}
	}
}

func TestParseRejectsInvalid(t *testing.T) {
	// parse has no answer that means "not a version", so an unreadable input is
	// ExitError rather than ExitNo, and not ExitUsage either: see
	// TestMisuseIsDistinctFromUnreadableInput.
	code, stdout, stderr := run(t, "parse", "1.2")

	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty so a caller never parses half an answer", stdout)
	}
	if stderr == "" {
		t.Error("stderr is empty, want an explanation")
	}
}

func TestType(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"v1.2.3", "stable"},
		{"v1.2.3-alpha.1", "alpha"},
		{"v1.2.3-beta.1", "beta"},
		{"v2.0.0-rc.2", "rc"},
		{"v2.0.0-next.7", "next"},
		{"v2.0.0-next.internal.3", "next-internal"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			code, stdout, stderr := run(t, "type", tt.in)

			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr)
			}
			// Printed bare and newline-terminated so `$(semstat type ...)`
			// feeds a case statement directly.
			if got := strings.TrimSuffix(stdout, "\n"); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTypeFailsClosed(t *testing.T) {
	for _, in := range []string{"v1.2.3-preview.1", "v1.2.3-vendor.alpha.1", "v1.2.3-rc"} {
		t.Run(in, func(t *testing.T) {
			code, stdout, stderr := run(t, "type", in)

			if code != ExitError {
				t.Errorf("exit = %d, want %d", code, ExitError)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty rather than a guessed type", stdout)
			}
			if !strings.Contains(stderr, "-next.internal.N") {
				t.Errorf("stderr %q does not name the supported suffixes", stderr)
			}
		})
	}
}

func TestTypeRejectsInvalidVersion(t *testing.T) {
	code, _, stderr := run(t, "type", "1.2")

	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stderr == "" {
		t.Error("stderr is empty, want an explanation")
	}
}

func TestCompare(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want string
	}{
		// The prerelease ordering case, through the CLI surface callers use.
		{"release candidate below its final release", "v2.0.0-rc.2", "v2.0.0", "-1"},
		{"final release above its release candidate", "v2.0.0", "v2.0.0-rc.2", "1"},
		{"build metadata does not affect precedence", "1.2.3+a", "1.2.3+b", "0"},
		{"equal", "v1.2.3", "1.2.3", "0"},
		{"minor compares numerically", "1.10.1", "1.9.1", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, "compare", tt.a, tt.b)

			// compare answers a question that always has an answer for two
			// valid versions, so it exits 0 whatever the ordering.
			if code != ExitOK {
				t.Fatalf("exit = %d, want %d (stderr: %s)", code, ExitOK, stderr)
			}
			if got := strings.TrimSuffix(stdout, "\n"); got != tt.want {
				t.Errorf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGreaterThan(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		{"final release beats its release candidate", "v2.0.0", "v2.0.0-rc.2", ExitOK},
		{"release candidate does not beat its final release", "v2.0.0-rc.2", "v2.0.0", ExitNo},
		{"equal is not greater", "v1.2.3", "v1.2.3", ExitNo},
		{"build metadata does not make it greater", "1.2.3+zzz", "1.2.3", ExitNo},
		{"newer patch", "v1.10.2", "v1.10.1", ExitOK},
		{"numeric minor ordering", "1.10.1", "1.9.1", ExitOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, "gt", tt.a, tt.b)

			if code != tt.want {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, tt.want, stderr)
			}
			// gt is a condition, not a reporter.
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
		})
	}
}

func TestGreaterThanSeparatesFalseFromUnreadable(t *testing.T) {
	// A caller writing `if semstat gt "$a" "$b"` takes the else branch on ExitNo, so
	// a typo'd version returning ExitNo too would read as a real ordering result.
	for _, args := range [][]string{
		{"gt", "bogus", "v2.0.0"},
		{"gt", "v2.0.0", "bogus"},
		{"gt", "1.2", "1.2.3"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			code, _, stderr := run(t, args...)

			if code != ExitError {
				t.Errorf("exit = %d, want %d so it is distinguishable from 'not greater'", code, ExitError)
			}
			if stderr == "" {
				t.Error("stderr is empty, want an explanation")
			}
		})
	}
}

func TestCompareRejectsInvalid(t *testing.T) {
	code, stdout, stderr := run(t, "compare", "v1.2.3", "bogus")

	if code != ExitError {
		t.Errorf("exit = %d, want %d", code, ExitError)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty rather than a bogus ordering", stdout)
	}
	if stderr == "" {
		t.Error("stderr is empty, want an explanation")
	}
}

func TestArgumentCounts(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"help with an argument", []string{"help", "garbage"}},
		{"version with an argument", []string{"version", "garbage"}},
		{"--version with an argument", []string{"--version", "garbage"}},
		{"validate with none", []string{"validate"}},
		{"validate with two", []string{"validate", "1.2.3", "1.2.4"}},
		{"parse with none", []string{"parse"}},
		{"type with two", []string{"type", "1.2.3", "1.2.4"}},
		{"compare with one", []string{"compare", "1.2.3"}},
		{"compare with three", []string{"compare", "1.2.3", "1.2.4", "1.2.5"}},
		{"gt with none", []string{"gt"}},
		{"gt with one", []string{"gt", "1.2.3"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.args...)

			if code != ExitUsage {
				t.Errorf("exit = %d, want %d", code, ExitUsage)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "takes ") {
				t.Errorf("stderr = %q, want it to state the expected argument count", stderr)
			}
		})
	}
}

// TestOptionsInVersionPositions covers the common shape of the miscall: an
// unquoted "$tag" that expanded to nothing, leaving semstat one argument that is
// the caller's own flag. No valid version starts with a dash, so the only useful
// answer is that the call was wrong, not that the version was unreadable.
func TestOptionsInVersionPositions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"validate", []string{"validate", "--strict"}},
		{"parse", []string{"parse", "-h"}},
		{"type", []string{"type", "--json"}},
		{"compare first", []string{"compare", "-v", "1.2.3"}},
		{"compare second", []string{"compare", "1.2.3", "-v"}},
		{"gt first", []string{"gt", "--quiet", "1.2.3"}},
		{"gt second", []string{"gt", "1.2.3", "--quiet"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.args...)

			if code != ExitUsage {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, ExitUsage, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if !strings.Contains(stderr, "option") {
				t.Errorf("stderr = %q, want it to name the offending option", stderr)
			}
		})
	}
}

func TestHelp(t *testing.T) {
	for _, arg := range []string{"-h", "--help", "help"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, stderr := run(t, arg)

			if code != ExitOK {
				t.Errorf("exit = %d, want %d", code, ExitOK)
			}
			if stderr != "" {
				t.Errorf("stderr = %q, want the help on stdout when it was asked for", stderr)
			}
			// Every subcommand has to appear, or the binary is its own stale
			// documentation.
			for _, command := range []string{"validate", "parse", "type", "compare", "gt"} {
				if !strings.Contains(stdout, command) {
					t.Errorf("help does not mention %q", command)
				}
			}
		})
	}
}

func TestVersion(t *testing.T) {
	for _, arg := range []string{"-v", "--version", "version"} {
		t.Run(arg, func(t *testing.T) {
			code, stdout, _ := run(t, arg)

			if code != ExitOK {
				t.Errorf("exit = %d, want %d", code, ExitOK)
			}
			if got := strings.TrimSuffix(stdout, "\n"); got != "test" {
				t.Errorf("stdout = %q, want the injected build version", got)
			}
		})
	}
}

func TestNoArguments(t *testing.T) {
	code, stdout, stderr := run(t)

	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	// Unasked-for help goes to stderr, so a bare `semstat` in a pipeline does
	// not feed the usage text downstream.
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr = %q, want the usage text", stderr)
	}
}

// TestMisuseIsDistinctFromUnreadableInput pins the split callers depend on.
// A caller that accepts ExitError from parse or type as "not a version" and
// stays green has to be sure that code cannot also mean "I do not have that
// subcommand": otherwise a bad pin turns a valid tag into a false verdict on a
// green step.
func TestMisuseIsDistinctFromUnreadableInput(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"parse reads an invalid version", []string{"parse", "1.2"}, ExitError},
		{"parse with no version", []string{"parse"}, ExitUsage},
		{"parse with two versions", []string{"parse", "1.2.3", "1.2.4"}, ExitUsage},
		{"parse renamed away", []string{"parse-version", "1.2.3"}, ExitUsage},
		{"parse given an option", []string{"parse", "-h"}, ExitUsage},

		{"type reads an invalid version", []string{"type", "1.2"}, ExitError},
		{"type reads an unroutable suffix", []string{"type", "1.2.3-preview.1"}, ExitError},
		{"type with no version", []string{"type"}, ExitUsage},
		{"type with two versions", []string{"type", "1.2.3", "1.2.4"}, ExitUsage},
		{"type renamed away", []string{"release-type", "1.2.3"}, ExitUsage},
		{"type given an option", []string{"type", "--json"}, ExitUsage},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := run(t, tt.args...)

			if code != tt.want {
				t.Errorf("exit = %d, want %d (stderr: %s)", code, tt.want, stderr)
			}
			if stdout != "" {
				t.Errorf("stdout = %q, want empty", stdout)
			}
			if stderr == "" {
				t.Error("stderr is empty, want an explanation")
			}
		})
	}
}

// The codes are a published contract, so their values are asserted rather than
// left to whatever order the constants happen to be declared in.
func TestExitCodeValues(t *testing.T) {
	for _, tt := range []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitNo", ExitNo, 1},
		{"ExitError", ExitError, 2},
		{"ExitUsage", ExitUsage, 64},
	} {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestUnknownCommand(t *testing.T) {
	code, stdout, stderr := run(t, "frobnicate", "1.2.3")

	if code != ExitUsage {
		t.Errorf("exit = %d, want %d", code, ExitUsage)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "frobnicate") {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
	}
}
