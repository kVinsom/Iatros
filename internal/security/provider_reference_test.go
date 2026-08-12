package security

import "testing"

func TestSafeProviderReference(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference string
		safe      bool
	}{
		{name: "repository identity", reference: "repository:acme/checkout", safe: true},
		{name: "catalog identity", reference: "catalog:checkout-system", safe: true},
		{name: "descriptive metadata", reference: "github repository acme/checkout at abc123", safe: true},
		{name: "empty"},
		{name: "surrounding whitespace", reference: " repository:acme/checkout"},
		{name: "control character", reference: "repository:acme/\x00checkout"},
		{name: "invalid UTF-8", reference: string([]byte{0xff})},
		{name: "URL", reference: "https://github.com/acme/checkout"},
		{name: "user information", reference: "user@github.com/acme/checkout"},
		{name: "query", reference: "repository:acme/checkout?token"},
		{name: "fragment", reference: "repository:acme/checkout#main"},
		{name: "backslash", reference: `repository:acme\checkout`},
		{name: "parameter", reference: "credential=secret"},
		{name: "authorization header", reference: "Authorization: secret"},
		{name: "proxy authorization header", reference: "Proxy-Authorization: secret"},
		{name: "bearer credential", reference: "Bearer secret"},
		{name: "basic credential", reference: "Basic secret"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if actual := IsSafeProviderReference(test.reference); actual != test.safe {
				t.Fatalf("IsSafeProviderReference(%q) = %t, want %t", test.reference, actual, test.safe)
			}
		})
	}
}
