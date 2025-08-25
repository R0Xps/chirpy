package api

import (
	"sync/atomic"

	"github.com/R0Xps/chirpy/internal/database"
)

// This struct is used for handling file server hits, secrets, and anything else needed by multiple endpoints
type Config struct {
	FileServerHits atomic.Int32
	DB             *database.Queries
	Platform       string
	Secret         string
	PolkaKey       string
}
