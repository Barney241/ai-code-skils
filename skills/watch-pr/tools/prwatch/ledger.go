package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// MinPingInterval is the floor between two `@coderabbitai review` comments,
// anywhere in the repo. CodeRabbit's included reviews refill per hour and the
// fair-usage ladder drops that to 1-2/hour at a busy repo's PR volume, so the
// quota is a shared serial resource: on 2026-08-18 three PRs were pinged within
// eight seconds and all three pings were refused, spending the retries for one
// slot.
const MinPingInterval = 30 * time.Minute

// InFlightWindow is how long a ping is considered still in flight before a
// follow-up is allowed. CodeRabbit normally answers within a couple of minutes.
const InFlightWindow = 15 * time.Minute

// Ping records one spend of review quota. Landed is set once a review is seen
// for that SHA — an "Action not completed" reply means the ping was refused and
// the slot was NOT consumed by a review, so it must not count as reviewed.
type Ping struct {
	PR     int       `json:"pr"`
	SHA    string    `json:"sha"`
	At     time.Time `json:"at"`
	Landed bool      `json:"landed"`
}

// Ledger is the repo-local record of review-quota spending. It lives in .git/
// so it is per-clone and never committed.
type Ledger struct {
	Pings []Ping `json:"pings"`
}

func ledgerPath(gitDir string) string {
	return filepath.Join(gitDir, "prwatch-ledger.json")
}

func loadLedger(gitDir string) (*Ledger, error) {
	b, err := os.ReadFile(ledgerPath(gitDir))
	if errors.Is(err, os.ErrNotExist) {
		return &Ledger{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read ledger: %w", err)
	}
	var l Ledger
	if err := json.Unmarshal(b, &l); err != nil {
		return nil, fmt.Errorf("parse ledger: %w", err)
	}
	return &l, nil
}

func (l *Ledger) save(gitDir string) error {
	b, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return fmt.Errorf("encode ledger: %w", err)
	}
	if err := os.WriteFile(ledgerPath(gitDir), b, 0o600); err != nil {
		return fmt.Errorf("write ledger: %w", err)
	}
	return nil
}

// lastPing returns the most recent ping, or nil.
func (l *Ledger) lastPing() *Ping {
	if len(l.Pings) == 0 {
		return nil
	}
	last := l.Pings[0]
	for _, p := range l.Pings[1:] {
		if p.At.After(last.At) {
			last = p
		}
	}
	return &last
}

// canPing decides whether spending a review on this PR/SHA is allowed right
// now. It returns the reason either way, because "why not" is what the agent
// has to report instead of silently retrying.
func (l *Ledger) canPing(now time.Time, s Status) (bool, string) {
	if s.Draft {
		return false, "PR is a draft; CodeRabbit does not review drafts"
	}
	if s.CodeRabbit.ReviewedHead {
		return false, "head is already reviewed; CodeRabbit will not re-review a reviewed commit"
	}
	if s.CodeRabbit.NextReviewAt != nil && now.Before(*s.CodeRabbit.NextReviewAt) {
		return false, fmt.Sprintf("rate limited until %s", s.CodeRabbit.NextReviewAt.UTC().Format(time.RFC3339))
	}
	for _, p := range l.Pings {
		if p.SHA == s.Head {
			return false, "this head SHA was already pinged at " + p.At.UTC().Format(time.RFC3339)
		}
	}
	if last := l.lastPing(); last != nil {
		if !last.Landed && now.Sub(last.At) < InFlightWindow {
			return false, fmt.Sprintf("a ping for PR #%d is still in flight", last.PR)
		}
		if wait := MinPingInterval - now.Sub(last.At); wait > 0 {
			return false, fmt.Sprintf("last ping was %s ago; %s to go before the next slot",
				now.Sub(last.At).Round(time.Second), wait.Round(time.Second))
		}
	}
	return true, "quota available"
}

func (l *Ledger) record(p Ping) {
	l.Pings = append(l.Pings, p)
	// Keep the file small; the rules only ever look at recent history plus the
	// per-SHA check, and a SHA that old is long merged.
	if len(l.Pings) > 200 {
		l.Pings = l.Pings[len(l.Pings)-200:]
	}
}
