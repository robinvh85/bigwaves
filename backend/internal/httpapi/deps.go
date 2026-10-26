package httpapi

import (
	"bigwaves/internal/platform/db"
)

// Deps are the services the handlers need. Fields are added as domains are ported.
type Deps struct {
	DB *db.DB
}
