package port

import "time"

// Clock supplies the observation time (a hashed business fact of the observation
// plane: when S4rCiv observed the resource). Injected so tests are deterministic.
type Clock interface {
	Now() time.Time
}

// IDGenerator mints external event identifiers (uuidv7).
type IDGenerator interface {
	NewID() string
}
