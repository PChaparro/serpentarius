package infrastructure

import (
	"github.com/rs/xid"
)

// GenerateXID generates a random unique identifier using the XID library.
func GenerateXID() string {
	return xid.New().String()
}
