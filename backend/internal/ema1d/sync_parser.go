package ema1d

import (
	"regexp"
	"slices"
	"strings"

	"bigwaves/internal/platform/httpx"
	"bigwaves/internal/platform/money"
)

// SyncRequest is the query of GET /api/ema1d/sync. S is the wanted state of the whole basket:
// COIN:STATE:ENTRY_DATE:REF_PRICE:RULE joined by ",", e.g. "SUN:S:2026-10-09:0.0213:XO,GRT:F:-:0.081:-".
type SyncRequest struct {
	Date   string
	RunID  string
	S      string
	DryRun string // "1", "true"...
}

// SyncItem is one coin of S. JSON keys match what the Rails app stored in ema1d_syncs.parsed.
type SyncItem struct {
	Coin      string     `json:"coin"`
	State     string     `json:"state"` // L, S or F
	EntryDate *string    `json:"entry_date"`
	RefPrice  *money.Dec `json:"ref_price"`
	Rule      *string    `json:"rule"`
}

func (i SyncItem) Flat() bool { return i.State == "F" }

func (i SyncItem) Side() string {
	switch i.State {
	case "L":
		return SideLong
	case "S":
		return SideShort
	}
	return ""
}

type parsedSync struct {
	Date     string
	RunID    string
	DryRun   bool
	Items    []SyncItem // in request order, one per coin
	Warnings []string
}

var (
	coinFormat  = regexp.MustCompile(`^[A-Z0-9]{1,20}$`)
	runIDFormat = regexp.MustCompile(`^[\w.:-]{1,64}$`)
)

func invalidRequest(msg string) error { return httpx.Validation(msg, nil) }

// truthy follows ActiveModel::Type::Boolean: "", "0", "f", "false", "off" are false.
func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "", "0", "f", "false", "off":
		return false
	}
	return true
}

// parseSync validates a sync request. A malformed request fails; a malformed item only drops that
// item with a warning, so one bad coin never blocks the rest of the basket.
func parseSync(req SyncRequest) (parsedSync, error) {
	var p parsedSync
	if req.Date == "" {
		return p, invalidRequest("date is required (YYYY-MM-DD)")
	}
	if _, err := parseDate(req.Date); err != nil {
		return p, invalidRequest("date must be YYYY-MM-DD")
	}
	p.Date = req.Date
	if !runIDFormat.MatchString(req.RunID) {
		return p, invalidRequest("run_id is required ([A-Za-z0-9_.:-], max 64)")
	}
	p.RunID = req.RunID
	if strings.TrimSpace(req.S) == "" {
		return p, invalidRequest("s is required")
	}
	p.DryRun = truthy(req.DryRun)

	seen := map[string]bool{}
	for _, token := range strings.Split(req.S, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		item, reason := parseItem(token, p.Date)
		if reason != "" {
			p.Warnings = append(p.Warnings, "invalid_item: "+token+" ("+reason+")")
			continue
		}
		if seen[item.Coin] {
			p.Warnings = append(p.Warnings, "duplicate_coin: "+item.Coin+" (kept the first)")
			continue
		}
		seen[item.Coin] = true
		p.Items = append(p.Items, item)
	}
	return p, nil
}

func parseItem(token, date string) (SyncItem, string) {
	f := strings.Split(token, ":")
	get := func(i int) string {
		if i < len(f) {
			return f[i]
		}
		return ""
	}
	if len(f) > 5 {
		return SyncItem{}, "too many fields"
	}
	coin, state, entry, ref, rule := strings.ToUpper(get(0)), strings.ToUpper(get(1)), get(2), get(3), strings.ToUpper(get(4))
	if !coinFormat.MatchString(coin) {
		return SyncItem{}, "bad coin"
	}
	if !slices.Contains([]string{"L", "S", "F"}, state) {
		return SyncItem{}, "state must be L, S or F"
	}
	item := SyncItem{Coin: coin, State: state}
	if strings.TrimSpace(entry) != "" && entry != "-" {
		if _, err := parseDate(entry); err != nil {
			return SyncItem{}, "bad entry date"
		}
		item.EntryDate = &entry
	}
	if state != "F" && item.EntryDate == nil {
		return SyncItem{}, "entry date is required for " + state
	}
	if item.EntryDate != nil && *item.EntryDate > date {
		return SyncItem{}, "entry date after " + date
	}
	if strings.TrimSpace(ref) != "" && ref != "-" {
		px, err := money.Parse(ref)
		if err != nil || !px.IsPositive() {
			return SyncItem{}, "bad ref price"
		}
		item.RefPrice = &px
	}
	if rule != "" && rule != "-" {
		if rule != "DIV4H" && rule != "XO" {
			return SyncItem{}, "rule must be DIV4H, XO or -"
		}
		item.Rule = &rule
	}
	return item, ""
}
