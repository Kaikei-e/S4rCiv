// Package sys provides the real Clock and uuidv7 IDGenerator implementations.
package sys

import (
	"time"

	"github.com/google/uuid"
)

type Clock struct{}

func (Clock) Now() time.Time { return time.Now().UTC() }

type IDGen struct{}

func (IDGen) NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.NewString() // v4 fallback; still a valid unique external id
	}
	return id.String()
}
