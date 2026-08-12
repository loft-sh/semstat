// Package cli implements the semstat command line.
package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/loft-sh/semstat/internal/semstat"
)

// ExitNo and ExitError are deliberately distinct: a caller writing
// `if semstat gt "$a" "$b"` has to tell "b is newer" apart from "I could not read
// $a". Only validate and gt have an answer that means no; for the others an
// unreadable version is a failure to answer, so it is ExitError.
const (
	ExitOK    = 0
	ExitNo    = 1
	ExitError = 2
)

const usage = `semstat reports facts about semantic versions. It never produces a new one.

Usage:
  semstat <command> [arguments]

Commands:
  validate <version>   report whether version is valid semver
  parse <version>      print major, minor, patch, prerelease, build and raw as JSON
  type <version>       print the release type: alpha, beta, rc, next, next-internal or stable
  compare <a> <b>      print -1, 0 or 1 as a sorts below, equal to, or above b
  gt <a> <b>           report whether a sorts above b
  version              print the semstat version

Exit codes:
  0   the command succeeded, or the answer is yes
  1   the answer is no: the version is invalid, or a does not sort above b
  2   the input could not be understood, or the command was misused

Versions may be given with or without a leading "v". Ordering follows semver
precedence, so build metadata never affects it and a prerelease always sorts
below its final release.

Examples, assigning first so that exit 2 is not swallowed by $(...):
  if semstat gt "$candidate" "$current"; then echo newer; elif [ $? -ne 1 ]; then exit 1; fi
  release_type="$(semstat type "$tag")" || exit 1
  parsed="$(semstat parse "$tag")" || exit 1
`

// Run executes one semstat invocation and returns its exit code. version is the
// semstat build version reported by the version subcommand.
func Run(args []string, version string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return ExitError
	}

	command, rest := args[0], args[1:]

	switch command {
	case "-h", "--help", "help":
		if code := none(command, rest, stderr); code != ExitOK {
			return code
		}
		fmt.Fprint(stdout, usage)
		return ExitOK

	case "-v", "--version", "version":
		if code := none(command, rest, stderr); code != ExitOK {
			return code
		}
		fmt.Fprintln(stdout, version)
		return ExitOK

	case "validate":
		return validate(rest, stderr)

	case "parse":
		return parse(rest, stdout, stderr)

	case "type":
		return releaseType(rest, stdout, stderr)

	case "compare":
		return compare(rest, stdout, stderr)

	case "gt":
		return greaterThan(rest, stderr)
	}

	fmt.Fprintf(stderr, "semstat: unknown command %q\n\n%s", command, usage)
	return ExitError
}

// validate reports through the exit code alone, so that `semstat validate "$v"`
// reads as a condition. Nothing is written on success.
func validate(args []string, stderr io.Writer) int {
	arg, code := one("validate", args, stderr)
	if code != ExitOK {
		return code
	}

	if _, err := semstat.Parse(arg); err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return ExitNo
	}

	return ExitOK
}

func parse(args []string, stdout, stderr io.Writer) int {
	arg, code := one("parse", args, stderr)
	if code != ExitOK {
		return code
	}

	v, err := semstat.Parse(arg)
	if err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return ExitError
	}

	encoded, err := json.Marshal(v.Parsed())
	if err != nil {
		// Unreachable: Parsed holds only strings and integers.
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return ExitError
	}

	fmt.Fprintln(stdout, string(encoded))
	return ExitOK
}

func releaseType(args []string, stdout, stderr io.Writer) int {
	arg, code := one("type", args, stderr)
	if code != ExitOK {
		return code
	}

	v, err := semstat.Parse(arg)
	if err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return ExitError
	}

	t, err := v.Type()
	if err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return ExitError
	}

	fmt.Fprintln(stdout, t)
	return ExitOK
}

func compare(args []string, stdout, stderr io.Writer) int {
	a, b, code := two("compare", args, stderr)
	if code != ExitOK {
		return code
	}

	fmt.Fprintln(stdout, a.Compare(b))
	return ExitOK
}

func greaterThan(args []string, stderr io.Writer) int {
	a, b, code := two("gt", args, stderr)
	if code != ExitOK {
		return code
	}

	if a.Compare(b) > 0 {
		return ExitOK
	}

	return ExitNo
}

// none rejects arguments to a command that takes none, so that a misspelled
// invocation is not silently answered as though it were correct.
func none(command string, args []string, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintf(stderr, "semstat: %s takes no arguments, got %d\n", command, len(args))
		return ExitError
	}

	return ExitOK
}

// one returns the single version argument a command was given.
func one(command string, args []string, stderr io.Writer) (string, int) {
	if len(args) != 1 {
		fmt.Fprintf(stderr, "semstat: %s takes exactly one version, got %d\n", command, len(args))
		return "", ExitError
	}

	return args[0], ExitOK
}

// two parses the pair of versions compare and gt take. Either one being
// unreadable is ExitError, never ExitNo, so an ordering answer is never
// confused with a parse failure.
func two(command string, args []string, stderr io.Writer) (*semstat.Version, *semstat.Version, int) {
	if len(args) != 2 {
		fmt.Fprintf(stderr, "semstat: %s takes exactly two versions, got %d\n", command, len(args))
		return nil, nil, ExitError
	}

	a, err := semstat.Parse(args[0])
	if err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return nil, nil, ExitError
	}

	b, err := semstat.Parse(args[1])
	if err != nil {
		fmt.Fprintf(stderr, "semstat: %v\n", err)
		return nil, nil, ExitError
	}

	return a, b, ExitOK
}
