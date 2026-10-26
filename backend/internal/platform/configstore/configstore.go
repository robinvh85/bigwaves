// Package configstore persists each strategy's configuration as one JSON document with a version
// number, and records every change in config_changes. The JSON shape belongs to the strategy.
package configstore

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"bigwaves/internal/platform/db"
	"bigwaves/internal/platform/db/dbq"
	"bigwaves/internal/platform/dbtype"
)

// ErrVersionConflict means someone saved the config since it was read.
var ErrVersionConflict = errors.New("config was changed by someone else; reload and try again")

// Validator is implemented by config types that can check themselves.
type Validator interface {
	Validate() error
}

type Store struct {
	db *db.DB

	mu    sync.RWMutex
	cache map[string]cached
}

type cached struct {
	raw     []byte
	version int64
}

func New(d *db.DB) *Store { return &Store{db: d, cache: map[string]cached{}} }

// Load reads strategy's config into out. When no row exists, out keeps its defaults and they are
// saved as version 1. Fields missing from a stored document keep their defaults too, so adding a
// field to a Config type needs no migration.
func (s *Store) Load(ctx context.Context, strategy string, out any) (version int64, err error) {
	s.mu.RLock()
	c, ok := s.cache[strategy]
	s.mu.RUnlock()
	if !ok {
		c, err = s.read(ctx, strategy, out)
		if err != nil {
			return 0, err
		}
	}
	if err := json.Unmarshal(c.raw, out); err != nil {
		return 0, fmt.Errorf("decode %s config: %w", strategy, err)
	}
	return c.version, nil
}

func (s *Store) read(ctx context.Context, strategy string, defaults any) (cached, error) {
	q := dbq.New(s.db.Write)
	row, err := q.GetStrategyConfig(ctx, strategy)
	if errors.Is(err, sql.ErrNoRows) {
		raw, mErr := json.Marshal(defaults)
		if mErr != nil {
			return cached{}, mErr
		}
		if err := q.InsertStrategyConfig(ctx, dbq.InsertStrategyConfigParams{Strategy: strategy, Config: dbtype.JSON(raw)}); err != nil {
			return cached{}, err
		}
		row, err = q.GetStrategyConfig(ctx, strategy)
	}
	if err != nil {
		return cached{}, err
	}
	c := cached{raw: row.Config, version: row.Version}
	s.mu.Lock()
	s.cache[strategy] = c
	s.mu.Unlock()
	return c, nil
}

// Save stores next if the stored version still equals version, and records the change.
func (s *Store) Save(ctx context.Context, strategy string, version int64, next any, userEmail string) (int64, error) {
	if v, ok := next.(Validator); ok {
		if err := v.Validate(); err != nil {
			return 0, err
		}
	}
	raw, err := json.Marshal(next)
	if err != nil {
		return 0, err
	}
	var newVersion int64
	err = s.db.Tx(ctx, func(tx *sql.Tx) error {
		q := dbq.New(tx)
		before, err := q.GetStrategyConfig(ctx, strategy)
		if err != nil {
			return err
		}
		n, err := q.UpdateStrategyConfig(ctx, dbq.UpdateStrategyConfigParams{Config: dbtype.JSON(raw), Strategy: strategy, Version: version})
		if err != nil {
			return err
		}
		if n == 0 {
			return ErrVersionConflict
		}
		newVersion = version + 1
		return q.InsertConfigChange(ctx, dbq.InsertConfigChangeParams{
			Strategy: strategy, Before: before.Config, After: dbtype.JSON(raw), UserEmail: userEmail,
		})
	})
	if err != nil {
		return 0, err
	}
	s.mu.Lock()
	s.cache[strategy] = cached{raw: raw, version: newVersion}
	s.mu.Unlock()
	return newVersion, nil
}

// History returns the latest changes, newest first.
func (s *Store) History(ctx context.Context, strategy string, limit int64) ([]dbq.ConfigChange, error) {
	return dbq.New(s.db.Read).ListConfigChanges(ctx, dbq.ListConfigChangesParams{Strategy: strategy, Limit: limit})
}

// Invalidate drops the cache (after an import that wrote the table directly).
func (s *Store) Invalidate() {
	s.mu.Lock()
	s.cache = map[string]cached{}
	s.mu.Unlock()
}
