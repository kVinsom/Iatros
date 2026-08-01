package cli

const (
	// ExitSuccess indicates successful command completion.
	ExitSuccess = 0
	// ExitFailure indicates an operational or internal failure.
	ExitFailure = 1
	// ExitUsage indicates invalid command syntax or arguments.
	ExitUsage = 2
	// ExitInvalidTarget indicates a missing, inaccessible, or unsupported analysis root.
	ExitInvalidTarget = 3
	// ExitNotImplemented indicates that the requested analysis contract has no implementation yet.
	ExitNotImplemented = 4
)

type exitError struct {
	code          int
	message       string
	reportWritten bool
}

func newExitError(code int, message string) *exitError {
	return &exitError{
		code:    code,
		message: message,
	}
}

func newReportedExitError(code int, message string) *exitError {
	err := newExitError(code, message)
	err.reportWritten = true
	return err
}

func (e *exitError) Error() string {
	return e.message
}
