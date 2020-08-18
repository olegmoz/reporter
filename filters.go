package main

import (
	"fmt"
	"github.com/google/go-github/v31/github"
	"time"
)

type TicketFilter interface {
	CheckTicket(i *github.Issue) bool
}

type PrFilter interface {
	CheckPr(pr *github.PullRequest) bool
}

type TimeFilter struct {
	Range DateRange
}

func (f *TimeFilter) CheckTicket(i *github.Issue) bool {
	return f.Range.Include(i.CreatedAt)
}

func (f *TimeFilter) CheckPr(pr *github.PullRequest) bool {
	return f.Range.Include(pr.CreatedAt)
}

// DateRange - checks if time is in range
type DateRange interface {
	// Include - check if date range includes the time
	Include(t *time.Time) bool
}

// ParseRange from string
func ParseRange(name string) (DateRange, error) {
	now := time.Now()
	if name == "daily" {
		return &DailyRange{&now}, nil
	}
	if name == "weekly" {
		return &WeeklyRange{t: &now}, nil
	}
	return nil, fmt.Errorf("Unkown range period: %s", name)
}

type FixedRange struct {
}

func (r *FixedRange) Include(t *time.Time) bool {
	if t == nil {
		return false
	}

	ry, rm, rd := t.Date()
	if ry != 2020 {
		return false
	}
	if rm != time.June {
		return false
	}
	if rd < 7 || rd > 13 {
		return false
	}
	return true
}

// DailyRange for one day
type DailyRange struct {
	t *time.Time
}

func (r *DailyRange) Include(t *time.Time) bool {
	if t == nil {
		return false
	}
	ly, lm, ld := r.t.Date()
	ry, rm, rd := t.Date()
	if ly != ry {
		return false
	}
	if lm != rm {
		return false
	}
	if ld != rd {
		return false
	}
	return true
}

// WeeklyRange for one week
type WeeklyRange struct {
	t *time.Time
}

func (r *WeeklyRange) Include(t *time.Time) bool {
	if t == nil {
		return false
	}
	return r.t.Add(-time.Hour * 24 * 7).Before(*t)
}
