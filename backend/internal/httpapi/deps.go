package httpapi

import (
	"bigwaves/internal/account"
	"bigwaves/internal/config"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/scheduler"
)

// Deps are the services the handlers need. Fields are added as domains are ported.
type Deps struct {
	Cfg       config.Config
	DB        *db.DB
	Pairs     *marketdata.Service
	Account   *account.Service
	Scheduler *scheduler.Scheduler
}
