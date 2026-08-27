package main

import "time"

// Verdict is the single field every caller branches on. Computing it in one
// place is the point of this tool: before it existed, each session re-derived
// "is this PR done?" from raw API calls and got it wrong in the same two ways —
// trusting the CodeRabbit status check, and treating "no unresolved threads" as
// evidence of a review.
type Verdict string

const (
	VerdictGreen             Verdict = "GREEN"
	VerdictCIPending         Verdict = "CI_PENDING"
	VerdictCIRed             Verdict = "CI_RED"
	VerdictConflicted        Verdict = "CONFLICTED"
	VerdictFeedbackPending   Verdict = "FEEDBACK_PENDING"
	VerdictRateLimited       Verdict = "RATE_LIMITED"
	VerdictBlockedUnreviewed Verdict = "BLOCKED_UNREVIEWED"
	VerdictDraft             Verdict = "DRAFT"
)

// CI is the state of the checks on the head commit.
type CI struct {
	State   string  `json:"state"`
	Pending int     `json:"pending"`
	Failing []Check `json:"failing"`
}

// Check is one failing check run.
type Check struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// CodeRabbitState answers the only question that matters about the bot: did it
// review THIS head commit? The `CodeRabbit` status check does not answer it —
// reviews.fail_commit_status defaults to false, so the check is green even when
// the review was skipped for rate limits.
type CodeRabbitState struct {
	ReviewedHead     bool       `json:"reviewed_head"`
	LastReviewAt     *time.Time `json:"last_review_at"`
	RateLimited      bool       `json:"rate_limited"`
	NextReviewAt     *time.Time `json:"next_review_at"`
	UnresolvedThread int        `json:"unresolved_threads"`
}

// HumanState is review feedback from people, which is handled differently:
// never resolve a human's thread on their behalf.
type HumanState struct {
	UnresolvedThread int    `json:"unresolved_threads"`
	Decision         string `json:"decision"`
}

// Status is the whole state of a PR, as one JSON document.
type Status struct {
	PR         int             `json:"pr"`
	URL        string          `json:"url"`
	Head       string          `json:"head"`
	HeadAt     time.Time       `json:"head_at"`
	Draft      bool            `json:"draft"`
	Mergeable  string          `json:"mergeable"`
	CI         CI              `json:"ci"`
	CodeRabbit CodeRabbitState `json:"coderabbit"`
	Human      HumanState      `json:"human"`
	Verdict    Verdict         `json:"verdict"`
	Reason     string          `json:"reason"`
}

// verdictFor ranks the states by what the agent should do next, most blocking
// first. Feedback outranks a still-running CI because findings can be fixed
// while checks finish; a draft outranks everything because nothing is expected
// of an unfinished PR.
func verdictFor(s Status) (Verdict, string) {
	switch {
	case s.Draft:
		return VerdictDraft, "PR is a draft; mark it ready when the local gates pass"
	case s.Mergeable == "CONFLICTING":
		return VerdictConflicted, "branch conflicts with the base; rebase before anything else"
	case len(s.CI.Failing) > 0:
		return VerdictCIRed, "CI failed: " + s.CI.Failing[0].Name
	case s.Human.Decision == "CHANGES_REQUESTED" || s.Human.UnresolvedThread > 0 || s.CodeRabbit.UnresolvedThread > 0:
		return VerdictFeedbackPending, "unresolved review feedback"
	case s.CI.Pending > 0:
		return VerdictCIPending, "checks still running"
	case !s.CodeRabbit.ReviewedHead && s.CodeRabbit.RateLimited:
		return VerdictRateLimited, "head is unreviewed and the review quota is exhausted"
	case !s.CodeRabbit.ReviewedHead:
		return VerdictBlockedUnreviewed, "no CodeRabbit review newer than the head commit"
	default:
		return VerdictGreen, "CI green, head reviewed, no unresolved feedback"
	}
}
