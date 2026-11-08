package domain

// Failure is a transport-independent, safe-to-display application failure.
// Backend response bodies and process output must never become its Message.
type Failure struct {
	Code    string
	Message string
}

func (e *Failure) Error() string { return e.Code + ": " + e.Message }
