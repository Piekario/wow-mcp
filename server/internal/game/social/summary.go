// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright (C) 2026 Piekasz (GitHub: Piekario). Author attribution must be kept: see NOTICE.

package social

// Summary is the guild roster counted: classes and level bands of everyone saved and of those online.
type Summary struct {
	Listed, Online               int
	Classes, OnlineClasses       map[string]int
	Bands, OnlineBands           map[int64]int // key = first level of the band of ten (20 = levels 20-29)
	OnlineAverageLevel           float64
	OnlineLevelKnown             int
	Unlevelled, OnlineUnlevelled int
}

// Summarize counts the guild members the dump lists. A class the dump has no text for is counted as "unknown".
func Summarize(members []Person) Summary {
	s := Summary{Classes: map[string]int{}, OnlineClasses: map[string]int{}, Bands: map[int64]int{}, OnlineBands: map[int64]int{}}
	var sum int64
	for _, m := range members {
		class := m.Class
		if class == "" {
			class = m.ClassToken
		}
		if class == "" {
			class = "unknown"
		}
		s.Listed++
		s.Classes[class]++
		if m.HasLevel {
			s.Bands[bandOf(m.Level)]++
		} else {
			s.Unlevelled++
		}
		if !m.Online {
			continue
		}
		s.Online++
		s.OnlineClasses[class]++
		if !m.HasLevel {
			s.OnlineUnlevelled++
			continue
		}
		s.OnlineBands[bandOf(m.Level)]++
		sum += m.Level
		s.OnlineLevelKnown++
	}
	if s.OnlineLevelKnown > 0 {
		s.OnlineAverageLevel = float64(sum*10/int64(s.OnlineLevelKnown)) / 10
	}
	return s
}

func bandOf(level int64) int64 { return level / 10 * 10 }
