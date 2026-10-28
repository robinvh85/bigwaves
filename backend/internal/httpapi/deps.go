package httpapi

import (
	"bigwaves/internal/account"
	"bigwaves/internal/config"
	"bigwaves/internal/dca"
	"bigwaves/internal/ema1d"
	"bigwaves/internal/emacross"
	"bigwaves/internal/emasettings"
	"bigwaves/internal/marketdata"
	"bigwaves/internal/platform/configstore"
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
	EMA1D     *ema1d.Service
	DCA       *dca.Service
	EMACross  *emacross.Service
	EMASets   *emasettings.Service
	Configs   *configstore.Store
}
