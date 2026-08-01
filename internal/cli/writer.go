package cli

import "io"

type writeErrorTracker struct {
	writer io.Writer
	err    error
}

func newWriteErrorTracker(writer io.Writer) *writeErrorTracker {
	return &writeErrorTracker{writer: writer}
}

func (w *writeErrorTracker) Write(data []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.writer == nil {
		w.err = io.ErrClosedPipe
		return 0, w.err
	}

	written, err := w.writer.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
	}

	return written, err
}

func (w *writeErrorTracker) Err() error {
	return w.err
}
