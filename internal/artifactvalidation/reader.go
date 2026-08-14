package artifactvalidation

import "bytes"

func newReadOnlyReader(content []byte) *bytes.Reader {
	return bytes.NewReader(content)
}
