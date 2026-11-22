// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package snapshot

import (
	"fmt"
	"math"
	"time"
)

// Money is an amount in copper (1g = 100s = 10000c).
type Money int64

// String writes the amount as in the game: "1g 23s 45c".
func (m Money) String() string {
	if m < 0 { // a loss or a negative balance: "-1g 2s 3c", not "-1g -2s -3c"
		if m == math.MinInt64 {
			m++ // -MinInt64 does not fit in int64; one copper does not matter at that size
		}
		return "-" + (-m).String()
	}
	copper := int64(m)
	gold, silver, rest := copper/10000, (copper/100)%100, copper%100
	switch {
	case gold > 0:
		return fmt.Sprintf("%dg %ds %dc", gold, silver, rest)
	case silver > 0:
		return fmt.Sprintf("%ds %dc", silver, rest)
	default:
		return fmt.Sprintf("%dc", rest)
	}
}

// Signed writes an amount change with a sign: "+1g 2s 3c", "-50c".
func (m Money) Signed() string {
	if m < 0 {
		return "-" + (-m).String()
	}
	return "+" + m.String()
}

// Round1 rounds to one decimal place, halves away from zero (1.25 -> 1.3, -1.25 -> -1.3). The one rounding used
// for rates, hours and percentages in every answer.
func Round1(v float64) float64 { return math.Round(v*10) / 10 }

// FormatAge writes a duration in a human-friendly way: "45 min", "3 h 20 min", "2 d 4 h".
func FormatAge(d time.Duration) string {
	minutes := int64(d.Minutes())
	switch {
	case minutes < 60:
		return fmt.Sprintf("%d min", minutes)
	case minutes < 48*60:
		if minutes%60 == 0 {
			return fmt.Sprintf("%d h", minutes/60)
		}
		return fmt.Sprintf("%d h %d min", minutes/60, minutes%60)
	default:
		return fmt.Sprintf("%d d %d h", minutes/(24*60), (minutes/60)%24)
	}
}

// FormatSeconds is FormatAge for a number of seconds (played time, auction time).
func FormatSeconds(seconds int64) string {
	return FormatAge(time.Duration(seconds) * time.Second)
}

// FormatTime writes a Unix time in the local time zone: "2006-01-02 15:04".
func FormatTime(unix int64) string {
	return time.Unix(unix, 0).Format("2006-01-02 15:04")
}

// Empty is the label of an empty slot or a missing value.
const Empty = "(empty)"

// OrEmpty replaces an empty string with the Empty label.
func OrEmpty(s string) string {
	if s == "" {
		return Empty
	}
	return s
}
