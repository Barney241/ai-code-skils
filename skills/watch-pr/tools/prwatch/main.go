// Command prwatch is the single source of truth for pull-request state.
//
// It exists because every session used to re-derive "is this PR done?" from raw
// gh calls, and got it wrong the same two ways: trusting the green CodeRabbit
// status check (which is green even when the review never ran), and treating
// "no unresolved threads" as evidence of a review (zero reviews also means zero
// threads). One job directory held eight hand-written pollers, four of them for
// the same PR.
//
//	prwatch status <pr>   one JSON document, one verdict
//	prwatch sweep         the same for every open PR of yours, oldest first
//	prwatch ping <pr>     spend one CodeRabbit review, if the ledger allows it
//	prwatch budget        what the ledger thinks is available, and when
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "status":
		err = cmdStatus(os.Args[2:])
	case "sweep":
		err = cmdSweep()
	case "ping":
		err = cmdPing(os.Args[2:])
	case "budget":
		err = cmdBudget()
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "prwatch:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `prwatch — one source of truth for PR state

  prwatch status <pr>   full state as JSON, including a single verdict
  prwatch sweep         one line per open PR of yours, oldest first
  prwatch ping <pr>     ask CodeRabbit to review, respecting the quota ledger
  prwatch budget        when the next review slot frees up

Verdicts: DRAFT, CONFLICTED, CI_RED, FEEDBACK_PENDING, CI_PENDING,
          RATE_LIMITED, BLOCKED_UNREVIEWED, GREEN
`)
}

func prArg(args []string) (int, error) {
	if len(args) < 1 {
		return 0, fmt.Errorf("need a PR number")
	}
	n, err := strconv.Atoi(strings.TrimPrefix(args[0], "#"))
	if err != nil {
		return 0, fmt.Errorf("%q is not a PR number", args[0])
	}
	return n, nil
}

func cmdStatus(args []string) error {
	pr, err := prArg(args)
	if err != nil {
		return err
	}
	s, err := fetchStatus(pr)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(s)
}

func cmdSweep() error {
	prs, err := openPRs()
	if err != nil {
		return err
	}
	if len(prs) == 0 {
		fmt.Println("no open PRs")
		return nil
	}
	for _, pr := range prs {
		s, err := fetchStatus(pr)
		if err != nil {
			fmt.Printf("#%-5d ERROR            %v\n", pr, err)
			continue
		}
		fmt.Printf("#%-5d %-18s %s\n", s.PR, s.Verdict, s.Reason)
	}
	return nil
}

// gitDir resolves the common git directory, so the ledger is shared by every
// worktree of this clone — the quota is per-account, not per-worktree.
func gitDir() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--path-format=absolute", "--git-common-dir").Output()
	if err != nil {
		return "", fmt.Errorf("locate git dir: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func cmdPing(args []string) error {
	pr, err := prArg(args)
	if err != nil {
		return err
	}
	s, err := fetchStatus(pr)
	if err != nil {
		return err
	}
	dir, err := gitDir()
	if err != nil {
		return err
	}
	l, err := loadLedger(dir)
	if err != nil {
		return err
	}

	now := time.Now()
	ok, reason := l.canPing(now, s)
	if !ok {
		fmt.Printf("not pinging #%d: %s\n", pr, reason)
		return nil
	}

	if err := postComment(pr, pingComment()); err != nil {
		return err
	}
	l.record(Ping{PR: pr, SHA: s.Head, At: now})
	if err := l.save(dir); err != nil {
		return err
	}
	fmt.Printf("pinged #%d for %s\n", pr, s.Head[:9])
	fmt.Println("verify it landed in ~90s: a reply saying \"Action not completed\" means it was refused, not queued")
	return nil
}

// pingComment is what gets posted to request a review. Override with
// $PRWATCH_PING_COMMENT for a bot with different wording.
func pingComment() string {
	if c := strings.TrimSpace(os.Getenv("PRWATCH_PING_COMMENT")); c != "" {
		return c
	}
	return "@coderabbitai review"
}

func cmdBudget() error {
	dir, err := gitDir()
	if err != nil {
		return err
	}
	l, err := loadLedger(dir)
	if err != nil {
		return err
	}
	last := l.lastPing()
	if last == nil {
		fmt.Println("no pings recorded; a review slot is available")
		return nil
	}
	since := time.Since(last.At)
	fmt.Printf("last ping: #%d %s at %s (%s ago)\n",
		last.PR, last.SHA[:min(9, len(last.SHA))], last.At.UTC().Format(time.RFC3339), since.Round(time.Second))
	if wait := MinPingInterval - since; wait > 0 {
		fmt.Printf("next slot in %s\n", wait.Round(time.Second))
	} else {
		fmt.Println("a review slot is available")
	}
	return nil
}
