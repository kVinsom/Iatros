package cli

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestWriteErrorTracker(t *testing.T) {
	t.Parallel()

	t.Run("successful write", func(t *testing.T) {
		t.Parallel()

		var output bytes.Buffer
		tracker := newWriteErrorTracker(&output)
		written, err := tracker.Write([]byte("output"))

		if written != len("output") || err != nil || tracker.Err() != nil {
			t.Fatalf("Write() = (%d, %v), tracker error = %v", written, err, tracker.Err())
		}
		if output.String() != "output" {
			t.Fatalf("output = %q, want %q", output.String(), "output")
		}
	})

	t.Run("short write", func(t *testing.T) {
		t.Parallel()

		tracker := newWriteErrorTracker(shortWriter{})
		written, err := tracker.Write([]byte("output"))

		if written != len("output")-1 || !errors.Is(err, io.ErrShortWrite) {
			t.Fatalf("Write() = (%d, %v), want short write", written, err)
		}
		if !errors.Is(tracker.Err(), io.ErrShortWrite) {
			t.Fatalf("Err() = %v, want io.ErrShortWrite", tracker.Err())
		}
	})

	t.Run("failing write is sticky", func(t *testing.T) {
		t.Parallel()

		writeErr := errors.New("write failed")
		tracker := newWriteErrorTracker(failingWriter{err: writeErr})
		if _, err := tracker.Write([]byte("first")); !errors.Is(err, writeErr) {
			t.Fatalf("first Write() error = %v, want %v", err, writeErr)
		}
		if _, err := tracker.Write([]byte("second")); !errors.Is(err, writeErr) {
			t.Fatalf("second Write() error = %v, want sticky %v", err, writeErr)
		}
	})

	t.Run("nil writer", func(t *testing.T) {
		t.Parallel()

		tracker := newWriteErrorTracker(nil)
		if _, err := tracker.Write([]byte("output")); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("Write() error = %v, want io.ErrClosedPipe", err)
		}
	})
}

type shortWriter struct{}

func (shortWriter) Write(data []byte) (int, error) {
	return len(data) - 1, nil
}
