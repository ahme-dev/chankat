package storage

import (
	"fmt"
	"time"
)

type PeriodKind string

const (
	Day   PeriodKind = "day"
	Week  PeriodKind = "week"
	Month PeriodKind = "month"
	All   PeriodKind = "all"
)

type Period struct {
	Kind  PeriodKind
	Start time.Time
	End   time.Time
}

func CurrentPeriod(kind PeriodKind, now time.Time) (Period, error) {
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch kind {
	case Day:
		return Period{Kind: kind, Start: day, End: day.AddDate(0, 0, 1)}, nil
	case Week:
		daysSinceMonday := (int(day.Weekday()) + 6) % 7
		start := day.AddDate(0, 0, -daysSinceMonday)
		return Period{Kind: kind, Start: start, End: start.AddDate(0, 0, 7)}, nil
	case Month:
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, day.Location())
		return Period{Kind: kind, Start: start, End: start.AddDate(0, 1, 0)}, nil
	case All:
		return Period{Kind: kind}, nil
	default:
		return Period{}, fmt.Errorf("unknown period %q", kind)
	}
}

func CustomPeriod(start, end time.Time) (Period, error) {
	if end.Before(start) {
		return Period{}, fmt.Errorf("end date must not precede start date")
	}
	return Period{
		Start: start,
		End:   end.AddDate(0, 0, 1),
	}, nil
}

func MovePeriod(period Period, offset int) Period {
	if offset == 0 || period.Kind == All || period.Kind == "" {
		return period
	}
	switch period.Kind {
	case Day:
		period.Start = period.Start.AddDate(0, 0, offset)
		period.End = period.End.AddDate(0, 0, offset)
	case Week:
		period.Start = period.Start.AddDate(0, 0, 7*offset)
		period.End = period.End.AddDate(0, 0, 7*offset)
	case Month:
		period.Start = period.Start.AddDate(0, offset, 0)
		period.End = period.End.AddDate(0, offset, 0)
	}
	return period
}

func StepPeriodKind(period Period, offset int, now time.Time) Period {
	if offset == 0 {
		return period
	}
	kinds := [...]PeriodKind{Day, Week, Month, All}
	index := len(kinds) - 1
	for i, kind := range kinds {
		if period.Kind == kind {
			index = i
			break
		}
	}
	if offset < 0 {
		offset = -1
	} else {
		offset = 1
	}
	index += offset
	if index < 0 {
		index = 0
	}
	if index >= len(kinds) {
		index = len(kinds) - 1
	}
	result, err := CurrentPeriod(kinds[index], now)
	if err != nil {
		return period
	}
	return result
}

func (p Period) Label() string {
	if p.Kind == All {
		return "All time"
	}
	if p.Kind == Day {
		return p.Start.Format("Mon, 02 Jan 2006")
	}
	return p.Start.Format("02 Jan 2006") + " – " +
		p.End.AddDate(0, 0, -1).Format("02 Jan 2006")
}

// EntriesInPeriod returns entries clipped to the half-open period interval.
// Active entries remain active when the period includes now.
func EntriesInPeriod(entries []Entry, period Period, now time.Time) []Entry {
	result := make([]Entry, 0, len(entries))
	for _, entry := range entries {
		endedAt := now
		if entry.EndedAt != nil && entry.EndedAt.Before(endedAt) {
			endedAt = *entry.EndedAt
		}
		if !period.End.IsZero() && period.End.Before(endedAt) {
			endedAt = period.End
		}
		startedAt := entry.StartedAt
		if !period.Start.IsZero() && period.Start.After(startedAt) {
			startedAt = period.Start
		}
		if !endedAt.After(startedAt) {
			continue
		}

		clipped := entry
		clipped.StartedAt = startedAt
		periodIncludesNow := period.End.IsZero() || period.End.After(now)
		if entry.EndedAt == nil && periodIncludesNow {
			clipped.EndedAt = nil
		} else {
			end := endedAt
			clipped.EndedAt = &end
		}
		result = append(result, clipped)
	}
	return result
}
