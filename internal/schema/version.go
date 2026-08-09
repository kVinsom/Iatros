// Package schema defines compatibility rules for versioned internal contracts.
package schema

import (
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidVersion indicates that a schema version is not a canonical major.minor value.
var ErrInvalidVersion = errors.New("schema version is invalid")

// Version identifies a schema using a canonical major.minor value.
type Version string

// Parse validates raw and returns its canonical schema version.
func Parse(raw string) (Version, error) {
	version := Version(raw)
	if _, _, ok := components(version); !ok {
		return "", ErrInvalidVersion
	}

	return version, nil
}

// String returns the canonical major.minor representation.
func (v Version) String() string {
	return string(v)
}

// Validate checks that the version uses the canonical major.minor representation.
func (v Version) Validate() error {
	if _, _, ok := components(v); !ok {
		return ErrInvalidVersion
	}

	return nil
}

// Supports reports whether a reader using v can consume a document version.
//
// Stable major versions accept documents at the same or an older minor version.
// Before version 1.0, compatibility requires an exact major.minor match.
func (v Version) Supports(document Version) bool {
	readerMajor, readerMinor, readerOK := components(v)
	documentMajor, documentMinor, documentOK := components(document)
	if !readerOK || !documentOK || readerMajor != documentMajor {
		return false
	}
	if readerMajor == 0 {
		return readerMinor == documentMinor
	}

	return readerMinor >= documentMinor
}

func components(version Version) (uint16, uint16, bool) {
	raw := string(version)
	majorText, minorText, found := strings.Cut(raw, ".")
	if !found || strings.Contains(minorText, ".") ||
		!canonicalNumber(majorText) || !canonicalNumber(minorText) {
		return 0, 0, false
	}

	major, majorErr := strconv.ParseUint(majorText, 10, 16)
	minor, minorErr := strconv.ParseUint(minorText, 10, 16)
	if majorErr != nil || minorErr != nil {
		return 0, 0, false
	}

	return uint16(major), uint16(minor), true
}

func canonicalNumber(numberText string) bool {
	if numberText == "" || (len(numberText) > 1 && numberText[0] == '0') {
		return false
	}
	for index := range len(numberText) {
		if numberText[index] < '0' || numberText[index] > '9' {
			return false
		}
	}

	return true
}
