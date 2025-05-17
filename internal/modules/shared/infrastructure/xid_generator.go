package infrastructure

import (
	"github.com/rs/xid"
)

func GenerateXID() string {
	return xid.New().String()
}
