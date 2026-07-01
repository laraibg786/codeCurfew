package curfew

import (
	"strings"
	"testing"
	"time"
)

func TestParse_ValidConfigs(t *testing.T) {
	tests := map[string]struct {
		content   string
		wantRules int
	}{
		"default config": {
			content:   DefaultConfig,
			wantRules: 7,
		},
		"comments and blank lines ignored": {
			content: `
# a comment
// another comment

Monday 1000 - *
`,
			wantRules: 1,
		},
		"wildcard start and end": {
			content:   "Tuesday * - *",
			wantRules: 1,
		},
		"deny rule with ! prefix": {
			content:   "! Friday 1400 - *",
			wantRules: 1,
		},
		"case-insensitive weekday": {
			content:   "monday 1000 - 1800",
			wantRules: 1,
		},
		"examples from codecurfew-format.md": {
			content: `
# Disallow on Friday after 04:30 PM UTC
! Friday 1630 - *

# Allow on Monday after 10:00 AM
Monday 1000 - *

# Disallow on Saturday and Sunday
! Saturday * - *
! Sunday * - *

# Allow Tuesday - Thursday till 05:00 PM
Tuesday * - 1700
Wednesday * - 1700
Thursday * - 1700
Friday * - 1700
`,
			wantRules: 8,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			rules, err := Parse(tc.content)
			if err != nil {
				t.Fatalf("Parse() returned unexpected error: %v", err)
			}
			if len(rules) != tc.wantRules {
				t.Fatalf("Parse() returned %d rules, want %d", len(rules), tc.wantRules)
			}
		})
	}
}

func TestParse_SortsRulesByStart(t *testing.T) {
	rules, err := Parse(DefaultConfig)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}
	for i := 1; i < len(rules); i++ {
		if rules[i-1].Start.After(rules[i].Start) {
			t.Fatalf("rules not sorted by Start: rule[%d].Start=%v is after rule[%d].Start=%v",
				i-1, rules[i-1].Start, i, rules[i].Start)
		}
	}
}

func TestParse_InvalidConfigs(t *testing.T) {
	tests := map[string]string{
		"missing dash separator": "Monday 1000 1800",
		"too few fields":         "Monday 1000 -",
		"bad weekday":            "Funday 1000 - 1800",
		"bad start time format":  "Monday abcd - 1800",
		"bad end time format":    "Monday 1000 - abcd",
	}

	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := Parse(content)
			if err == nil {
				t.Fatalf("Parse(%q) expected an error, got nil", content)
			}
		})
	}
}

func TestInCurfew(t *testing.T) {
	rules, err := Parse(`
Monday 1000 - 1800
! Tuesday 0900 - 1700
`)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	monday := nextWeekday(time.Monday)
	tuesday := nextWeekday(time.Tuesday)

	tests := map[string]struct {
		t    time.Time
		want bool
	}{
		"outside any rule window (wednesday) is not in curfew": {
			t:    nextWeekday(time.Wednesday).Add(12 * time.Hour),
			want: false,
		},
		"allow rule window (monday) is not in curfew": {
			t:    monday.Add(11 * time.Hour), // 11:00, within 1000-1800 allow window
			want: false,
		},
		"deny rule window (tuesday) is in curfew": {
			t:    tuesday.Add(10 * time.Hour), // 10:00, within 0900-1700 deny window
			want: true,
		},
		"deny rule boundary start is in curfew": {
			t:    tuesday.Add(9 * time.Hour), // exactly 0900 start boundary
			want: true,
		},
		"deny rule boundary end is in curfew": {
			t:    tuesday.Add(17 * time.Hour), // exactly 1700 end boundary
			want: true,
		},
		"just after deny rule end is not in curfew": {
			t:    tuesday.Add(17*time.Hour + time.Minute),
			want: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := rules.InCurfew(tc.t, nil)
			if got != tc.want {
				t.Fatalf("InCurfew(%v) = %v, want %v", tc.t, got, tc.want)
			}
		})
	}
}

func TestNext(t *testing.T) {
	rules, err := Parse(`
Monday 1000 - 1800
! Tuesday 0900 - 1700
`)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	tuesday := nextWeekday(time.Tuesday)

	t.Run("time not in curfew returns the same time", func(t *testing.T) {
		notInCurfew := nextWeekday(time.Wednesday).Add(12 * time.Hour)
		got, err := rules.Next(notInCurfew, nil)
		if err != nil {
			t.Fatalf("Next() returned unexpected error: %v", err)
		}
		if !got.Equal(notInCurfew) {
			t.Fatalf("Next() = %v, want %v", got, notInCurfew)
		}
	})

	t.Run("time in curfew returns the end of the curfew window", func(t *testing.T) {
		inCurfew := tuesday.Add(10 * time.Hour) // within 0900-1700 deny window
		want := tuesday.Add(17*time.Hour + time.Minute)
		got, err := rules.Next(inCurfew, nil)
		if err != nil {
			t.Fatalf("Next() returned unexpected error: %v", err)
		}
		if !got.Equal(want) {
			t.Fatalf("Next() = %v, want %v", got, want)
		}
	})
}

func TestValidate(t *testing.T) {
	if err := Validate(DefaultConfig); err != nil {
		t.Fatalf("Validate(DefaultConfig) returned unexpected error: %v", err)
	}
	if err := Validate("Monday 1000 - *"); err != nil {
		t.Fatalf("Validate() returned unexpected error for a valid line: %v", err)
	}
	if err := Validate("Monday 1000 1800"); err == nil {
		t.Fatal("Validate() expected an error for a malformed line (missing '-'), got nil")
	}
	if err := Validate("Funday 1000 - 1800"); err == nil {
		t.Fatal("Validate() expected an error for a bad weekday, got nil")
	}
	if err := Validate("Monday abcd - 1800"); err == nil {
		t.Fatal("Validate() expected an error for a bad time format, got nil")
	}
}

// nextWeekday returns the UTC start-of-day for the next occurrence of day,
// matching the semantics of the package-private nextTime helper used by Parse.
func nextWeekday(day time.Weekday) time.Time {
	today := time.Now().UTC()
	daysUntil := (int(day) - int(today.Weekday()) + 7) % 7
	return time.Date(today.Year(), today.Month(), today.Day()+daysUntil, 0, 0, 0, 0, time.UTC)
}

func TestParseWeekday_CaseInsensitive(t *testing.T) {
	for _, name := range []string{"monday", "MONDAY", "Monday", "mOnDaY"} {
		d, err := parseWeekday(name)
		if err != nil {
			t.Fatalf("parseWeekday(%q) returned unexpected error: %v", name, err)
		}
		if d != time.Monday {
			t.Fatalf("parseWeekday(%q) = %v, want %v", name, d, time.Monday)
		}
	}
	if _, err := parseWeekday("notaday"); err == nil {
		t.Fatal("parseWeekday(\"notaday\") expected an error, got nil")
	}
}

func TestErrorMessagesIncludeOriginalLine(t *testing.T) {
	_, err := Parse("  Funday 1000 - 1800  ")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Funday 1000 - 1800") {
		t.Fatalf("error message %q does not include original (untrimmed) line content", err.Error())
	}
}
