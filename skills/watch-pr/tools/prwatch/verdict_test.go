package main

import (
	"testing"
	"time"
)

func TestVerdictFor(t *testing.T) {
	head := time.Date(2026, 8, 18, 10, 0, 0, 0, time.UTC)
	reviewed := head.Add(5 * time.Minute)
	stale := head.Add(-5 * time.Minute)

	green := func() Status {
		return Status{
			Head:       "abc123456",
			HeadAt:     head,
			Mergeable:  "MERGEABLE",
			CI:         CI{State: "SUCCESS"},
			CodeRabbit: CodeRabbitState{ReviewedHead: true, LastReviewAt: &reviewed},
			Human:      HumanState{Decision: "APPROVED"},
		}
	}

	tests := []struct {
		name  string
		mutid func(*Status)
		want  Verdict
	}{
		{
			name:  "everything clear",
			mutid: func(*Status) {},
			want:  VerdictGreen,
		},
		{
			name:  "draft outranks everything",
			mutid: func(s *Status) { s.Draft = true; s.CI.Failing = []Check{{Name: "test"}} },
			want:  VerdictDraft,
		},
		{
			name:  "conflict outranks a red CI",
			mutid: func(s *Status) { s.Mergeable = "CONFLICTING"; s.CI.Failing = []Check{{Name: "test"}} },
			want:  VerdictConflicted,
		},
		{
			name:  "red CI outranks feedback",
			mutid: func(s *Status) { s.CI.Failing = []Check{{Name: "test"}}; s.CodeRabbit.UnresolvedThread = 2 },
			want:  VerdictCIRed,
		},
		{
			name:  "unresolved coderabbit thread is feedback",
			mutid: func(s *Status) { s.CodeRabbit.UnresolvedThread = 1 },
			want:  VerdictFeedbackPending,
		},
		{
			name:  "unresolved human thread is feedback",
			mutid: func(s *Status) { s.Human.UnresolvedThread = 1 },
			want:  VerdictFeedbackPending,
		},
		{
			name:  "changes requested is feedback even with no threads",
			mutid: func(s *Status) { s.Human.Decision = "CHANGES_REQUESTED" },
			want:  VerdictFeedbackPending,
		},
		{
			name:  "feedback outranks pending checks",
			mutid: func(s *Status) { s.CI.Pending = 3; s.CodeRabbit.UnresolvedThread = 1 },
			want:  VerdictFeedbackPending,
		},
		{
			name:  "pending checks",
			mutid: func(s *Status) { s.CI.Pending = 2 },
			want:  VerdictCIPending,
		},
		{
			// The regression this whole tool exists for: green checks, no
			// threads, and CodeRabbit never reviewed this head.
			name: "green checks but head unreviewed is not green",
			mutid: func(s *Status) {
				s.CodeRabbit = CodeRabbitState{ReviewedHead: false, LastReviewAt: &stale}
			},
			want: VerdictBlockedUnreviewed,
		},
		{
			name: "unreviewed and rate limited waits for the slot",
			mutid: func(s *Status) {
				s.CodeRabbit = CodeRabbitState{ReviewedHead: false, RateLimited: true}
			},
			want: VerdictRateLimited,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := green()
			tc.mutid(&s)
			got, reason := verdictFor(s)
			if got != tc.want {
				t.Fatalf("verdictFor() = %q (%s), want %q", got, reason, tc.want)
			}
			if reason == "" {
				t.Fatal("verdictFor() returned an empty reason")
			}
		})
	}
}

func TestParseNextReview(t *testing.T) {
	from := time.Date(2026, 8, 17, 15, 14, 0, 0, time.UTC)

	tests := []struct {
		name string
		body string
		want *time.Time
	}{
		{
			name: "the real comment CodeRabbit posts",
			body: "> **Next review available in:** **40 minutes**\n> Limit details: ...",
			want: ptr(from.Add(42 * time.Minute)),
		},
		{
			name: "plain wording without markdown",
			body: "Next review available in: 15 minutes",
			want: ptr(from.Add(17 * time.Minute)),
		},
		{
			name: "hours",
			body: "Next review available in: **2 hours**",
			want: ptr(from.Add(2*time.Hour + 2*time.Minute)),
		},
		{
			name: "no statement means do not guess",
			body: "Review limit reached. Please wait.",
			want: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseNextReview(tc.body, from)
			switch {
			case tc.want == nil && got != nil:
				t.Fatalf("parseNextReview() = %v, want nil", got)
			case tc.want != nil && got == nil:
				t.Fatalf("parseNextReview() = nil, want %v", tc.want)
			case tc.want != nil && !got.Equal(*tc.want):
				t.Fatalf("parseNextReview() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestIsCodeRabbit(t *testing.T) {
	tests := []struct {
		login string
		want  bool
	}{
		{"coderabbitai", true},
		{"coderabbitai[bot]", true},
		{"CodeRabbit[bot]", true},
		{"Barney241", false},
		{"github-actions", false},
	}
	for _, tc := range tests {
		t.Run(tc.login, func(t *testing.T) {
			if got := isCodeRabbit(tc.login); got != tc.want {
				t.Fatalf("isCodeRabbit(%q) = %v, want %v", tc.login, got, tc.want)
			}
		})
	}
}

func TestRateLimitState(t *testing.T) {
	now := time.Date(2026, 8, 18, 11, 15, 0, 0, time.UTC)
	// The shape PR #3430 actually produced: a standing notice edited in place to
	// carry the current wait, and a newer reply refusing a ping that carries none.
	notice := func(updated time.Time, wait string) botComment {
		return botComment{
			Login:     "coderabbitai",
			Body:      "> ## Review limit reached\n> **Next review available in:** **" + wait + "**\n",
			UpdatedAt: updated,
		}
	}
	refusal := func(updated time.Time) botComment {
		return botComment{Login: "coderabbitai", Body: "Review rate limited.", UpdatedAt: updated}
	}

	tests := []struct {
		name        string
		comments    []botComment
		wantLimited bool
		wantNext    *time.Time
	}{
		{
			name:        "no bot comments at all",
			comments:    []botComment{{Login: "Barney241", Body: "looks good", UpdatedAt: now}},
			wantLimited: false,
		},
		{
			name:        "a human quoting the notice is not the bot",
			comments:    []botComment{{Login: "Barney241", Body: "Review limit reached again?", UpdatedAt: now}},
			wantLimited: false,
		},
		{
			name: "refusal is newest, the wait lives in the older notice",
			comments: []botComment{
				notice(now.Add(-2*time.Minute), "41 minutes"),
				refusal(now.Add(-1 * time.Minute)),
			},
			wantLimited: true,
			wantNext:    ptr(now.Add(-2*time.Minute + 41*time.Minute + 2*time.Minute)),
		},
		{
			name:        "an expired window is not a limit",
			comments:    []botComment{notice(now.Add(-2*time.Hour), "41 minutes")},
			wantLimited: false,
			wantNext:    ptr(now.Add(-2*time.Hour + 41*time.Minute + 2*time.Minute)),
		},
		{
			name:        "fresh refusal with no window anywhere is still a limit",
			comments:    []botComment{refusal(now.Add(-5 * time.Minute))},
			wantLimited: true,
		},
		{
			name:        "stale refusal with no window has lapsed",
			comments:    []botComment{refusal(now.Add(-3 * time.Hour))},
			wantLimited: false,
		},
		{
			name: "hours parse too",
			comments: []botComment{
				notice(now.Add(-1*time.Minute), "2 hours"),
			},
			wantLimited: true,
			wantNext:    ptr(now.Add(-1*time.Minute + 2*time.Hour + 2*time.Minute)),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotLimited, gotNext := rateLimitState(tc.comments, now)
			switch {
			case gotLimited != tc.wantLimited:
				t.Fatalf("rateLimitState() limited = %v, want %v", gotLimited, tc.wantLimited)
			case tc.wantNext == nil && gotNext != nil:
				t.Fatalf("rateLimitState() next = %v, want nil", gotNext)
			case tc.wantNext != nil && gotNext == nil:
				t.Fatalf("rateLimitState() next = nil, want %v", tc.wantNext)
			case tc.wantNext != nil && !gotNext.Equal(*tc.wantNext):
				t.Fatalf("rateLimitState() next = %v, want %v", gotNext, tc.wantNext)
			}
		})
	}
}
