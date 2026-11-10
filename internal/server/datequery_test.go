package server

import "testing"

// The album's search reads dates as people write them, in English and
// German; a month or a day without a year is found in every year.
func TestASearchIsReadAsADate(t *testing.T) {
	days := []string{"2026-11-10", "2026-11-09", "2026-10-10", "2025-11-10", "2024-02-29", "2024-05-01"}
	cases := map[string][]string{
		"2026":              {"2026-11-10", "2026-11-09", "2026-10-10"},
		"2026-11":           {"2026-11-10", "2026-11-09"},
		"2026/11":           {"2026-11-10", "2026-11-09"},
		"11.2026":           {"2026-11-10", "2026-11-09"},
		"11/2026":           {"2026-11-10", "2026-11-09"},
		"2026-11-10":        {"2026-11-10"},
		"10.11.2026":        {"2026-11-10"},
		"10.11.26":          {"2026-11-10"},
		"1.5.2024":          {"2024-05-01"},
		"10.11.":            {"2026-11-10", "2025-11-10"},
		"10.11":             {"2026-11-10", "2025-11-10"},
		"29.2.":             {"2024-02-29"},
		"November 2026":     {"2026-11-10", "2026-11-09"},
		"  nov.  2026 ":     {"2026-11-10", "2026-11-09"},
		"Mai 2024":          {"2024-05-01"},
		"März":              nil,
		"november":          {"2026-11-10", "2026-11-09", "2025-11-10"},
		"10. November 2026": {"2026-11-10"},
		"10 Nov 2026":       {"2026-11-10"},
		"November 10, 2026": {"2026-11-10"},
		"10. Oktober":       {"2026-10-10"},
	}
	for q, want := range cases {
		match := dateQuery(q)
		if match == nil {
			t.Errorf("%q is no date", q)
			continue
		}
		var got []string
		for _, d := range days {
			if match(d) {
				got = append(got, d)
			}
		}
		if len(got) != len(want) {
			t.Errorf("%q matches %v, want %v", q, got, want)
			continue
		}
		for i := range got {
			if got[i] != want[i] {
				t.Errorf("%q matches %v, want %v", q, got, want)
				break
			}
		}
	}
	for _, q := range []string{"", "lake", "IMG_2026", "31.4.2026", "13.2026", "2026-13", "30.2.", "0.1.2026", "99", "12345", "nov 99999", "foo 2026", "1.2.3.4"} {
		if dateQuery(q) != nil {
			t.Errorf("%q was read as a date", q)
		}
	}
}
