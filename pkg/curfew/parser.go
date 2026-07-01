// Package curfew implements the .codecurfew DSL: a small, weekday-based
// format describing when commits are allowed or denied. It is deliberately
// standalone (no dependency on this module's internal packages) so it can be
// imported and reused outside the webhook server, e.g. by a future
// standalone validator.
package curfew

import (
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"
)

const (
	militaryTimeLayout = "1504"
	// DefaultConfig is the built-in fallback DSL text used when a repository
	// does not define its own .codecurfew file (or it cannot be fetched/parsed).
	DefaultConfig = `
Monday 1000 - *
Tuesday * - *
Wednesday * - *
Thursday * - *
! Friday 1400 - *
! Saturday * - *
! Sunday * - *
`
)

type (
	// Rule represents a single parsed line of the .codecurfew DSL.
	Rule struct {
		IsAllowed bool
		Start     time.Time
		End       time.Time
	}

	// Rules is a parsed, sorted collection of Rule.
	Rules []Rule
)

// InCurfew reports whether t falls within a curfew (disallowed) window
// according to the rules.
func (r Rules) InCurfew(t time.Time, l *slog.Logger) bool {
	var inCurfew bool
	if l == nil {
		l = slog.Default()
		l.Warn("logger not found in context, using default logger")
	}
	for _, rule := range r {
		if t.Weekday() != rule.Start.Weekday() {
			continue
		}
		if !t.Before(rule.Start) && !t.After(rule.End) {
			inCurfew = !rule.IsAllowed
			break
		}
	}
	l.Debug("checking curfew status", "time", t, "rules", r, "status", inCurfew)
	return inCurfew
}

// Next returns the next time after t at which commits are allowed again, per
// the rules. If t is not currently in curfew, it returns t unchanged.
func (r Rules) Next(t time.Time, l *slog.Logger) (time.Time, error) {
	if l == nil {
		l = slog.Default()
		l.Warn("logger not found in context, using default logger")
	}
	t = t.UTC()
	if !r.InCurfew(t, l) {
		return t, nil
	}

	for _, rule := range r {
		// ignore the rules that have already ended
		end := rule.End
		if t.After(end) {
			continue
		}
		// rule slice is sorted so the first matching is the closest to the given time.
		if rule.IsAllowed {
			if rule.Start.After(t) {
				return rule.Start, nil
			}
		} else if !r.InCurfew(end.Add(time.Minute), l) {
			// check for checking the consecutive curfew rules.
			return end.Add(time.Minute), nil
		}
	}
	return time.Time{}, fmt.Errorf("could not find the next allowed time after %v", t)
}

// Parse parses the .codecurfew DSL content into a sorted set of Rules.
func Parse(content string) (Rules, error) {
	rules := Rules{}
	for _, originalLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(originalLine)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "//") {
			// ignore comments and blank lines.
			continue
		}
		isAllowed := !strings.HasPrefix(line, "!")
		if !isAllowed {
			line = strings.TrimSpace(line[1:])
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[2] != "-" {
			return rules, fmt.Errorf("cannot parse the fields in the invalid line: %q", originalLine)
		}

		if fields[1] == "*" {
			fields[1] = "0000" // treat * as start of day
		}
		if fields[3] == "*" {
			fields[3] = "2359" // treat * as end of day
		}

		day, err := parseWeekday(fields[0])
		if err != nil {
			return rules, fmt.Errorf("invalid weekday name in the line: %q", originalLine)
		}

		startTime, err := time.Parse(militaryTimeLayout, fields[1])
		if err != nil {
			return rules, fmt.Errorf("cannot parse start time in line: %q", originalLine)
		}
		endTime, err := time.Parse(militaryTimeLayout, fields[3])
		if err != nil {
			return rules, fmt.Errorf("cannot parse end time in line: %q", originalLine)
		}
		rule := Rule{
			IsAllowed: isAllowed,
			Start:     nextTime(startTime, day),
			End:       nextTime(endTime, day),
		}
		rules = append(rules, rule)
		slog.Debug("parsed rule from config", "rule", rule, "text", originalLine)
	}
	slices.SortFunc(rules, func(r1, r2 Rule) int {
		if r1.Start.Before(r2.Start) {
			return -1
		}
		if r1.Start.After(r2.Start) {
			return 1
		}
		return 0
	})
	return rules, nil
}

func nextTime(t time.Time, day time.Weekday) time.Time {
	today := time.Now().UTC()
	daysUntil := (int(day) - int(today.Weekday()) + 7) % 7
	return time.Date(today.Year(), today.Month(), today.Day()+daysUntil,
		t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), time.UTC)
}

func parseWeekday(s string) (time.Weekday, error) {
	for d := time.Sunday; d <= time.Saturday; d++ {
		if strings.EqualFold(d.String(), s) {
			return d, nil
		}
	}
	return 0, fmt.Errorf("invalid weekday: %s", s)
}
