package schema

import (
	"errors"
	"testing"
)

func TestParseAcceptsCanonicalVersions(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{"0.1", "1.0", "12.34", "65535.65535"} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			version, err := Parse(raw)
			if err != nil {
				t.Fatalf("Parse(%q) error = %v", raw, err)
			}
			if version.String() != raw {
				t.Fatalf("String() = %q, want %q", version.String(), raw)
			}
			if err := version.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestParseRejectsNonCanonicalVersions(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"", "1", "1.", ".1", "1.2.3", "01.2", "1.02", "-1.0", "1.-1",
		"v1.0", "1.0 ", "65536.0", "0.65536",
	} {
		raw := raw
		t.Run(raw, func(t *testing.T) {
			t.Parallel()

			if _, err := Parse(raw); !errors.Is(err, ErrInvalidVersion) {
				t.Fatalf("Parse(%q) error = %v, want ErrInvalidVersion", raw, err)
			}
		})
	}
}

func TestVersionSupportsStableSchemas(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		reader   Version
		document Version
		want     bool
	}{
		{name: "same", reader: "1.2", document: "1.2", want: true},
		{name: "older minor", reader: "1.2", document: "1.1", want: true},
		{name: "newer minor", reader: "1.1", document: "1.2", want: false},
		{name: "different major", reader: "2.0", document: "1.9", want: false},
		{name: "pre-release exact", reader: "0.2", document: "0.2", want: true},
		{name: "pre-release minor differs", reader: "0.2", document: "0.1", want: false},
		{name: "invalid reader", reader: "latest", document: "1.0", want: false},
		{name: "invalid document", reader: "1.0", document: "", want: false},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := test.reader.Supports(test.document); got != test.want {
				t.Fatalf("Supports(%q) = %t, want %t", test.document, got, test.want)
			}
		})
	}
}
