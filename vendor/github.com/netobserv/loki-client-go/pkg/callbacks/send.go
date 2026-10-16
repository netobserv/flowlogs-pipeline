package callbacks

// Send is a set of callbacks invoked during the sending job.
type Send interface {
	// OnRetry is called during retry backoff. First argument is the error, second is the attempt number.
	OnRetry(error, int)
	// OnSuccess is called when send succeeded.
	OnSuccess()
	// OnError is called when send failed with a non-retryable error.
	OnError(error)
}
