package analysis

import (
	"testing"

	"github.com/kVinsom/Iatros/internal/finding"
)

func TestCompareFindings(t *testing.T) {
	t.Parallel()

	base := testFinding()
	critical := base
	critical.Severity = finding.SeverityCritical
	delivery := base
	delivery.ID = "delivery.ci.missing.local"

	tests := []struct {
		name     string
		left     Finding
		right    Finding
		wantSign int
	}{
		{name: "severity", left: critical, right: base, wantSign: -1},
		{name: "id", left: delivery, right: base, wantSign: -1},
		{name: "equal key", left: base, right: base, wantSign: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			comparison := compareFindings(test.left, test.right)
			if sign(comparison) != test.wantSign {
				t.Fatalf("compareFindings() = %d, want sign %d", comparison, test.wantSign)
			}
		})
	}
}

func sign(value int) int {
	switch {
	case value < 0:
		return -1
	case value > 0:
		return 1
	default:
		return 0
	}
}
