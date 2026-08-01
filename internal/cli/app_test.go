package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/kVinsom/Iatros/internal/analysis"
)

func TestHelpCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "root", args: nil},
		{name: "subcommand", args: []string{"help"}},
		{name: "flag", args: []string{"--help"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCLI(t, analysis.NewLocalStub(), test.args...)
			if result.exitCode != ExitSuccess {
				t.Fatalf("exit code = %d, want %d", result.exitCode, ExitSuccess)
			}
			if !strings.Contains(result.stdout, "Usage:") ||
				!strings.Contains(result.stdout, "analyze") {
				t.Fatalf("stdout does not contain command help:\n%s", result.stdout)
			}
			if result.stderr != "" {
				t.Fatalf("stderr = %q, want empty", result.stderr)
			}
		})
	}
}

func TestVersionCommands(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{{"version"}, {"--version"}} {
		result := runCLI(t, analysis.NewLocalStub(), args...)
		if result.exitCode != ExitSuccess {
			t.Fatalf("args %v: exit code = %d, want %d", args, result.exitCode, ExitSuccess)
		}
		if result.stdout != "iatros test-version\n" {
			t.Fatalf("args %v: stdout = %q, want version line", args, result.stdout)
		}
		if result.stderr != "" {
			t.Fatalf("args %v: stderr = %q, want empty", args, result.stderr)
		}
	}
}

func TestVersionNormalization(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "trimmed", version: " 1.2.3 \n", want: "iatros 1.2.3\n"},
		{name: "default", version: " \t ", want: "iatros dev\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := Run(
				t.Context(),
				test.version,
				analysis.NewLocalStub(),
				[]string{"version"},
				&stdout,
				&stderr,
			)

			if exitCode != ExitSuccess || stdout.String() != test.want || stderr.Len() != 0 {
				t.Fatalf(
					"Run() = (%d, %q, %q), want (%d, %q, empty)",
					exitCode,
					stdout.String(),
					stderr.String(),
					ExitSuccess,
					test.want,
				)
			}
		})
	}
}

func TestUsageErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		args []string
	}{
		{name: "unsupported format", args: []string{"analyze", "--format", "yaml", "."}},
		{name: "too many targets", args: []string{"analyze", ".", "."}},
		{name: "unknown command", args: []string{"unknown"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result := runCLI(t, analysis.NewLocalStub(), test.args...)
			if result.exitCode != ExitUsage {
				t.Fatalf("exit code = %d, want %d", result.exitCode, ExitUsage)
			}
			if result.stdout != "" {
				t.Fatalf("stdout = %q, want empty", result.stdout)
			}
			if !strings.Contains(result.stderr, "Error:") ||
				!strings.Contains(result.stderr, "Usage:") {
				t.Fatalf("stderr does not contain error and usage:\n%s", result.stderr)
			}
		})
	}
}

func TestRunDetectsCommandOutputFailures(t *testing.T) {
	t.Parallel()

	writeErr := errors.New("write failed")
	for _, args := range [][]string{nil, {"--help"}, {"--version"}, {"version"}} {
		var stderr bytes.Buffer
		exitCode := Run(
			t.Context(),
			"test-version",
			analysis.NewLocalStub(),
			args,
			failingWriter{err: writeErr},
			&stderr,
		)

		if exitCode != ExitFailure {
			t.Fatalf("args %v: exit code = %d, want %d", args, exitCode, ExitFailure)
		}
		if !strings.Contains(stderr.String(), "could not write") {
			t.Fatalf("args %v: stderr does not describe output failure: %q", args, stderr.String())
		}
	}
}

func TestRunDetectsUsageOutputFailure(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	exitCode := Run(
		t.Context(),
		"test-version",
		analysis.NewLocalStub(),
		[]string{"unknown"},
		&stdout,
		failingWriter{err: errors.New("write failed")},
	)

	if exitCode != ExitFailure {
		t.Fatalf("exit code = %d, want %d", exitCode, ExitFailure)
	}
}

func TestRunHandlesNilWriters(t *testing.T) {
	t.Parallel()

	for _, args := range [][]string{nil, {"unknown"}} {
		exitCode := Run(
			t.Context(),
			"test-version",
			analysis.NewLocalStub(),
			args,
			nil,
			nil,
		)
		if exitCode != ExitFailure {
			t.Fatalf("args %v: exit code = %d, want %d", args, exitCode, ExitFailure)
		}
	}
}

type commandResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runCLI(t *testing.T, analyzer Analyzer, args ...string) commandResult {
	t.Helper()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	exitCode := Run(
		t.Context(),
		"test-version",
		analyzer,
		args,
		&stdout,
		&stderr,
	)

	return commandResult{
		exitCode: exitCode,
		stdout:   stdout.String(),
		stderr:   stderr.String(),
	}
}

type failingWriter struct {
	err error
}

func (w failingWriter) Write([]byte) (int, error) {
	return 0, w.err
}
