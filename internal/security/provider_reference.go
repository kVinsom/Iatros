package security

import (
	"strings"
	"unicode/utf8"
)

// IsSafeProviderReference reports whether reference is sanitized metadata rather than a URL or credential.
func IsSafeProviderReference(reference string) bool {
	if !isSafeText(reference) || strings.Contains(reference, "://") ||
		strings.ContainsAny(reference, "@?#\\=") {
		return false
	}

	lowerReference := strings.ToLower(reference)
	return !strings.Contains(lowerReference, "authorization:") &&
		!strings.Contains(lowerReference, "proxy-authorization:") &&
		!strings.Contains(lowerReference, "bearer ") &&
		!strings.Contains(lowerReference, "basic ")
}

func isSafeText(text string) bool {
	if text == "" || !utf8.ValidString(text) || strings.TrimSpace(text) != text {
		return false
	}
	for _, character := range text {
		if character < 0x20 || (character >= 0x7f && character <= 0x9f) {
			return false
		}
	}
	return true
}
