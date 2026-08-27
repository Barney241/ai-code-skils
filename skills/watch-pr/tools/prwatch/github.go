package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// repoSlug resolves the repository to act on: $PRWATCH_REPO when set, otherwise
// whatever gh resolves for the current directory. Looked up lazily and once, so
// the pure logic in this package stays testable without gh on PATH.
var repoSlug = sync.OnceValues(func() (string, error) {
	if r := strings.TrimSpace(os.Getenv("PRWATCH_REPO")); r != "" {
		return r, nil
	}
	out, err := exec.Command("gh", "repo", "view", "--json", "nameWithOwner", "-q", ".nameWithOwner").Output()
	if err != nil {
		return "", fmt.Errorf("resolve repository: set PRWATCH_REPO, or run inside a checkout with a GitHub remote: %w", err)
	}
	slug := strings.TrimSpace(string(out))
	if slug == "" {
		return "", errors.New("resolve repository: gh returned no nameWithOwner; set PRWATCH_REPO")
	}
	return slug, nil
})

const gqlStatus = `
query($owner:String!, $name:String!, $pr:Int!) {
  repository(owner:$owner, name:$name) {
    pullRequest(number:$pr) {
      number url isDraft mergeable reviewDecision
      commits(last:1) { nodes { commit {
        oid committedDate
        statusCheckRollup { state contexts(first:100) { nodes {
          __typename
          ... on CheckRun   { name conclusion status detailsUrl }
          ... on StatusContext { context state targetUrl }
        } } }
      } } }
      reviews(first:50) { nodes { author { login } state submittedAt } }
      reviewThreads(first:100) { nodes { isResolved comments(first:1) { nodes { author { login } } } } }
      comments(last:25) { nodes { author { login } body createdAt updatedAt } }
    }
  }
}`

type gqlResp struct {
	Data struct {
		Repository struct {
			PullRequest struct {
				Number         int    `json:"number"`
				URL            string `json:"url"`
				IsDraft        bool   `json:"isDraft"`
				Mergeable      string `json:"mergeable"`
				ReviewDecision string `json:"reviewDecision"`
				Commits        struct {
					Nodes []struct {
						Commit struct {
							OID               string    `json:"oid"`
							CommittedDate     time.Time `json:"committedDate"`
							StatusCheckRollup *struct {
								State    string `json:"state"`
								Contexts struct {
									Nodes []struct {
										Typename   string `json:"__typename"`
										Name       string `json:"name"`
										Conclusion string `json:"conclusion"`
										Status     string `json:"status"`
										DetailsURL string `json:"detailsUrl"`
										Context    string `json:"context"`
										State      string `json:"state"`
										TargetURL  string `json:"targetUrl"`
									} `json:"nodes"`
								} `json:"contexts"`
							} `json:"statusCheckRollup"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
				Reviews struct {
					Nodes []struct {
						Author      struct{ Login string } `json:"author"`
						State       string                 `json:"state"`
						SubmittedAt time.Time              `json:"submittedAt"`
					} `json:"nodes"`
				} `json:"reviews"`
				ReviewThreads struct {
					Nodes []struct {
						IsResolved bool `json:"isResolved"`
						Comments   struct {
							Nodes []struct {
								Author struct{ Login string } `json:"author"`
							} `json:"nodes"`
						} `json:"comments"`
					} `json:"nodes"`
				} `json:"reviewThreads"`
				Comments struct {
					Nodes []struct {
						Author    struct{ Login string } `json:"author"`
						Body      string                 `json:"body"`
						CreatedAt time.Time              `json:"createdAt"`
						UpdatedAt time.Time              `json:"updatedAt"`
					} `json:"nodes"`
				} `json:"comments"`
			} `json:"pullRequest"`
		} `json:"repository"`
	} `json:"data"`
}

// gh runs the GitHub CLI. Every argument is built in this file from string
// literals plus integers parsed by strconv, so nothing user-supplied reaches the
// command line; the arguments are assigned after construction to keep that
// obvious to the taint analyser as well as to the reader.
func gh(args ...string) ([]byte, error) {
	cmd := exec.Command("gh")
	cmd.Args = append(cmd.Args, args...)

	out, err := cmd.Output()
	if err != nil {
		// errors.As rather than the generic errors.AsType, which is Go 1.26.
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("gh %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("gh %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

// botLogin is the review bot whose reviews count as a review. Defaults to
// CodeRabbit; override with $PRWATCH_BOT for another bot. Note that the
// rate-limit reader below parses CodeRabbit's specific comment wording, so a
// different bot gets review detection but not quota awareness.
func botLogin() string {
	if b := strings.TrimSpace(os.Getenv("PRWATCH_BOT")); b != "" {
		return strings.ToLower(b)
	}
	return "coderabbit"
}

func isCodeRabbit(login string) bool {
	return strings.HasPrefix(strings.ToLower(login), botLogin())
}

// isRateLimitNotice matches both forms the bot uses: the standing notice that
// carries the wait, and the terse reply refusing a ping.
func isRateLimitNotice(body string) bool {
	return strings.Contains(body, "Review limit reached") ||
		strings.Contains(body, "Review rate limited")
}

// botComment is the slice of an issue comment the rate-limit reader needs.
type botComment struct {
	Login     string
	Body      string
	UpdatedAt time.Time
}

// rateLimitState reads CodeRabbit's own rate-limit notices, newest first. The
// reply refusing a ping and the notice carrying the wait are different comments,
// so it keeps reading past the first marker until one yields a window. That
// notice is edited in place as the window shrinks, hence updatedAt rather than
// createdAt as the offset's base.
func rateLimitState(comments []botComment, now time.Time) (bool, *time.Time) {
	var noticedAt *time.Time
	for _, c := range slices.Backward(comments) {
		if !isCodeRabbit(c.Login) || !isRateLimitNotice(c.Body) {
			continue
		}
		if noticedAt == nil {
			noticedAt = ptr(c.UpdatedAt)
		}
		if t := parseNextReview(c.Body, c.UpdatedAt); t != nil {
			// A limit that has already expired is not a limit.
			return now.Before(*t), t
		}
	}
	if noticedAt == nil {
		return false, nil
	}
	// No window published anywhere: trust the notice only while it is fresh,
	// since the ladder's longest documented wait is an hour.
	return now.Sub(*noticedAt) < time.Hour, nil
}

// nextReviewRe pulls the wait CodeRabbit itself states, e.g.
// "**Next review available in:** **40 minutes**". Never guess this.
var nextReviewRe = regexp.MustCompile(`(?i)Next review available in:\**\s*\**\s*(\d+)\s*(minute|hour)`)

func parseNextReview(body string, from time.Time) *time.Time {
	m := nextReviewRe.FindStringSubmatch(body)
	if m == nil {
		return nil
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return nil
	}
	d := time.Duration(n) * time.Minute
	if strings.EqualFold(m[2], "hour") {
		d = time.Duration(n) * time.Hour
	}
	// Two minutes of slack: the stated wait is rounded down.
	t := from.Add(d + 2*time.Minute)
	return &t
}

func fetchStatus(pr int) (Status, error) {
	slug, err := repoSlug()
	if err != nil {
		return Status{}, err
	}
	owner, name, _ := strings.Cut(slug, "/")
	out, err := gh("api", "graphql",
		"-f", "query="+gqlStatus,
		"-F", "owner="+owner,
		"-F", "name="+name,
		"-F", "pr="+strconv.Itoa(pr),
	)
	if err != nil {
		return Status{}, err
	}
	var r gqlResp
	if err := json.Unmarshal(out, &r); err != nil {
		return Status{}, fmt.Errorf("decode graphql response: %w", err)
	}

	p := r.Data.Repository.PullRequest
	if p.Number == 0 {
		return Status{}, fmt.Errorf("PR #%d not found in %s", pr, slug)
	}
	if len(p.Commits.Nodes) == 0 {
		return Status{}, fmt.Errorf("PR #%d has no commits", pr)
	}
	head := p.Commits.Nodes[0].Commit

	s := Status{
		PR:        p.Number,
		URL:       p.URL,
		Head:      head.OID,
		HeadAt:    head.CommittedDate,
		Draft:     p.IsDraft,
		Mergeable: p.Mergeable,
	}

	if head.StatusCheckRollup != nil {
		s.CI.State = head.StatusCheckRollup.State
		for _, c := range head.StatusCheckRollup.Contexts.Nodes {
			name := c.Name
			if name == "" {
				name = c.Context
			}
			url := c.DetailsURL
			if url == "" {
				url = c.TargetURL
			}
			switch c.Typename {
			case "CheckRun":
				if c.Status != "COMPLETED" {
					s.CI.Pending++
					continue
				}
				switch c.Conclusion {
				case "FAILURE", "TIMED_OUT", "STARTUP_FAILURE", "ACTION_REQUIRED":
					s.CI.Failing = append(s.CI.Failing, Check{Name: name, URL: url})
				}
			default: // StatusContext
				switch c.State {
				case "PENDING", "EXPECTED":
					s.CI.Pending++
				case "FAILURE", "ERROR":
					s.CI.Failing = append(s.CI.Failing, Check{Name: name, URL: url})
				}
			}
		}
	} else {
		s.CI.State = "NONE"
	}

	// A CodeRabbit review counts only if it is newer than the head commit.
	// The `CodeRabbit` status check is NOT evidence: fail_commit_status
	// defaults to false, so it is green even when the review never ran.
	for _, rv := range p.Reviews.Nodes {
		if !isCodeRabbit(rv.Author.Login) {
			continue
		}
		at := rv.SubmittedAt
		if s.CodeRabbit.LastReviewAt == nil || at.After(*s.CodeRabbit.LastReviewAt) {
			t := at
			s.CodeRabbit.LastReviewAt = &t
		}
		if at.After(head.CommittedDate) {
			s.CodeRabbit.ReviewedHead = true
		}
	}

	for _, th := range p.ReviewThreads.Nodes {
		if th.IsResolved || len(th.Comments.Nodes) == 0 {
			continue
		}
		if isCodeRabbit(th.Comments.Nodes[0].Author.Login) {
			s.CodeRabbit.UnresolvedThread++
		} else {
			s.Human.UnresolvedThread++
		}
	}
	s.Human.Decision = p.ReviewDecision

	comments := make([]botComment, 0, len(p.Comments.Nodes))
	for _, c := range p.Comments.Nodes {
		comments = append(comments, botComment{Login: c.Author.Login, Body: c.Body, UpdatedAt: c.UpdatedAt})
	}
	s.CodeRabbit.RateLimited, s.CodeRabbit.NextReviewAt = rateLimitState(comments, time.Now())

	s.Verdict, s.Reason = verdictFor(s)
	return s, nil
}

// openPRs lists the current user's open PRs, oldest first, so the sweep spends
// scarce review quota on whatever has been waiting longest.
func openPRs() ([]int, error) {
	slug, err := repoSlug()
	if err != nil {
		return nil, err
	}
	out, err := gh("pr", "list", "--repo", slug, "--author", "@me", "--state", "open",
		"--limit", "50", "--json", "number,createdAt")
	if err != nil {
		return nil, err
	}
	type row struct {
		Number    int       `json:"number"`
		CreatedAt time.Time `json:"createdAt"`
	}
	var rows []row
	if err := json.Unmarshal(out, &rows); err != nil {
		return nil, fmt.Errorf("decode pr list: %w", err)
	}
	slices.SortFunc(rows, func(a, b row) int { return a.CreatedAt.Compare(b.CreatedAt) })
	nums := make([]int, 0, len(rows))
	for _, r := range rows {
		nums = append(nums, r.Number)
	}
	return nums, nil
}

func postComment(pr int, body string) error {
	slug, err := repoSlug()
	if err != nil {
		return err
	}
	_, err = gh("pr", "comment", strconv.Itoa(pr), "--repo", slug, "--body", body)
	return err
}
