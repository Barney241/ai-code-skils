package main

import (
	"testing"
	"time"
)

func TestLedgerCanPing(t *testing.T) {
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	unreviewed := Status{
		PR:         3430,
		Head:       "abc123456",
		CodeRabbit: CodeRabbitState{ReviewedHead: false},
	}

	tests := []struct {
		name   string
		ledger Ledger
		status func(Status) Status
		want   bool
	}{
		{
			name:   "empty ledger, head unreviewed",
			ledger: Ledger{},
			status: func(s Status) Status { return s },
			want:   true,
		},
		{
			name:   "draft is never pinged",
			ledger: Ledger{},
			status: func(s Status) Status { s.Draft = true; return s },
			want:   false,
		},
		{
			// CodeRabbit refuses: "does not re-review already reviewed commits".
			name:   "head already reviewed",
			ledger: Ledger{},
			status: func(s Status) Status { s.CodeRabbit.ReviewedHead = true; return s },
			want:   false,
		},
		{
			name:   "same SHA pinged before",
			ledger: Ledger{Pings: []Ping{{PR: 3430, SHA: "abc123456", At: now.Add(-2 * time.Hour), Landed: true}}},
			status: func(s Status) Status { return s },
			want:   false,
		},
		{
			// The 2026-08-18 failure: three PRs pinged within eight seconds,
			// all three refused for one slot.
			name:   "another PR pinged five minutes ago",
			ledger: Ledger{Pings: []Ping{{PR: 3421, SHA: "other1234", At: now.Add(-5 * time.Minute), Landed: true}}},
			status: func(s Status) Status { return s },
			want:   false,
		},
		{
			name:   "another PR pinged an hour ago",
			ledger: Ledger{Pings: []Ping{{PR: 3421, SHA: "other1234", At: now.Add(-time.Hour), Landed: true}}},
			status: func(s Status) Status { return s },
			want:   true,
		},
		{
			name:   "a ping is still in flight",
			ledger: Ledger{Pings: []Ping{{PR: 3421, SHA: "other1234", At: now.Add(-3 * time.Minute), Landed: false}}},
			status: func(s Status) Status { return s },
			want:   false,
		},
		{
			name:   "stated rate limit has not expired",
			ledger: Ledger{},
			status: func(s Status) Status {
				s.CodeRabbit.NextReviewAt = ptr(now.Add(20 * time.Minute))
				return s
			},
			want: false,
		},
		{
			name:   "stated rate limit has expired",
			ledger: Ledger{},
			status: func(s Status) Status {
				s.CodeRabbit.NextReviewAt = ptr(now.Add(-1 * time.Minute))
				return s
			},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := tc.ledger
			got, reason := l.canPing(now, tc.status(unreviewed))
			if got != tc.want {
				t.Fatalf("canPing() = %v (%s), want %v", got, reason, tc.want)
			}
			if reason == "" {
				t.Fatal("canPing() returned an empty reason")
			}
		})
	}
}

func TestLedgerRoundTrip(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)

	l, err := loadLedger(dir)
	if err != nil {
		t.Fatalf("loadLedger() on a fresh dir: %v", err)
	}
	l.record(Ping{PR: 3430, SHA: "abc123456", At: now})
	if err := l.save(dir); err != nil {
		t.Fatalf("save(): %v", err)
	}

	got, err := loadLedger(dir)
	if err != nil {
		t.Fatalf("loadLedger() after save: %v", err)
	}
	want := Ledger{Pings: []Ping{{PR: 3430, SHA: "abc123456", At: now}}}
	if len(got.Pings) != 1 || got.Pings[0].PR != want.Pings[0].PR ||
		got.Pings[0].SHA != want.Pings[0].SHA || !got.Pings[0].At.Equal(want.Pings[0].At) {
		t.Fatalf("round trip = %+v, want %+v", *got, want)
	}
}

func TestLedgerRecordIsBounded(t *testing.T) {
	var l Ledger
	base := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	for i := range 250 {
		l.record(Ping{PR: i, SHA: "sha", At: base.Add(time.Duration(i) * time.Minute)})
	}
	if len(l.Pings) != 200 {
		t.Fatalf("len(Pings) = %d, want 200", len(l.Pings))
	}
	if l.Pings[len(l.Pings)-1].PR != 249 {
		t.Fatalf("newest ping = %d, want 249", l.Pings[len(l.Pings)-1].PR)
	}
}
