package ci

import (
	"os"
	"testing"
	"time"
)

// TestProbe_ReportsTheLatestRunForATag exercises the real gh-arg path against a
// fake gh on PATH: the argv git-release builds is the contract.
func TestProbe_ReportsTheLatestRunForATag(t *testing.T) {
	dir := t.TempDir()
	fake := `#!/bin/sh
	case "$1" in
	run) shift; echo '[{"databaseId":42,"status":"completed","conclusion":"failure","displayTitle":"RC Release","createdAt":"2026-09-20T13:36:00Z","url":"https://example/runs/42"}]' ;;
	esac`
	if err := os.WriteFile(dir+"/gh", []byte(fake), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))

	r, err := Probe(dir, "v0.1.52-rc.5")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if r == nil || r.DatabaseID != 42 || r.Conclusion != "failure" {
		t.Fatalf("unexpected run: %+v", r)
	}
}

// TestProbe_NoGhDegradesToNil: a missing gh is silence, never an error that
// fails the command being decorated.
func TestProbe_NoGhDegradesToNil(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	r, err := Probe(t.TempDir(), "v0.1.0")
	if r != nil {
		t.Fatal("expected nil run when gh is absent")
	}
	if err == nil {
		t.Fatal("expected a logged error, the caller decides how loudly")
	}
}

// TestVerdict: each run state reads the way a waiting release-cutter needs it
// to — failed runs say FAILED and name the failed job, working runs name the
// job and runner, success says when.
func TestVerdict(t *testing.T) {
	now := time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)
	failed := &Run{Status: "completed", Conclusion: "failure", CreatedAt: "2026-09-26T19:40:00Z"}
	if got := Verdict(failed,
		[]Job{{Name: "build-and-push", Status: "completed", Conclusion: "failure"}},
		now); got != "FAILURE 20 minutes ago — build-and-push failed" {
		t.Fatalf("failed verdict: %q", got)
	}
	running := &Run{Status: "in_progress"}
	if got := Verdict(running,
		[]Job{{Name: "build", Status: "in_progress", RunnerName: "github-runner-2", StartedAt: "2026-09-26T19:30:00Z"}},
		now); got != "in progress — build on github-runner-2 (30m elapsed)" {
		t.Fatalf("running verdict: %q", got)
	}
	queued := &Run{Status: "queued"}
	if got := Verdict(queued, []Job{{Name: "build", Status: "queued"}}, now); got != "queued — build (queued)" {
		t.Fatalf("queued verdict: %q", got)
	}
	ok := &Run{Status: "completed", Conclusion: "success", CreatedAt: "2026-09-26T19:40:00Z"}
	if got := Verdict(ok, nil, now); got != "succeeded 20 minutes ago" {
		t.Fatalf("success verdict: %q", got)
	}
	if got := Verdict(nil, nil, now); got != "" {
		t.Fatalf("nil verdict: %q", got)
	}
}
