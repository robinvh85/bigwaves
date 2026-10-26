package app_test

import (
	"strconv"

	m "bigwaves/internal/platform/money"
)

func itoa(i int64) string { return strconv.FormatInt(i, 10) }

func money(s string) m.Dec { return m.MustParse(s) }
