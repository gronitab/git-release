// Package ci answers what GitHub Actions is doing with a release tag.
//
// WHY THIS EXISTS (GRIT-6872). `git release stage` pushes a tag and reports
// success, which is true about the tag and says nothing about the release: the
// job the tag triggers can fail twenty minutes later while the caller waits for
// a rollout that is never coming. The two durable answers belong to the two
// commands that hold the question:
//
//   - `git release stage` hands back the run it just triggered, so the log is
//     one copy-paste away.
//   - `git release status` shows the in-flight RC's run verdict next to the tag:
//     an in-flight RC whose job failed is not in flight, it is finished and lost.
//
// BEST-EFFORT BY DESIGN. This shells out to `gh` and NEVER fails the command
// it decorates: the tool's contract is git (tags in, tags out), and a missing
// `gh`, an unauthenticated account, a rate limit or a network hiccup all
// degrade to "no CI information" rather than an error. Every caller treats a
// nil RunInfo as "say nothing beyond the fallback hint".
package ci

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Run is the latest workflow run GitHub has for a tag.
type Run struct {
	DatabaseID   int64  `json:"databaseId"`
	Status       string `json:"status"`     // queued, in_progress, completed
	Conclusion   string `json:"conclusion"` // success, failure, cancelled, …
	DisplayTitle string `json:"displayTitle"`
	CreatedAt    string `json:"createdAt"`
	HTMLURL      string `json:"url"`
}

// Job is one job of a run, with the fields the waiting caller actually asks
// for: which job, what state, and where it has been running.
type Job struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	StartedAt  string `json:"startedAt"`
	RunnerName string `json:"runnerName"`
}

// Probe returns the latest run for the tag, or nil when nothing (or no gh) is
// reachable. The error is for the log, not for the caller's exit code.
func Probe(dir, tag string) (*Run, error) {
	out, err := run(dir, "run", "list", "--branch", tag, "--limit", "1",
		"--json", "databaseId,status,conclusion,displayTitle,createdAt,url")
	if err != nil {
		return nil, fmt.Errorf("gh run list: %w", err)
	}
	var runs []Run
	if err := json.Unmarshal(out, &runs); err != nil {
		return nil, fmt.Errorf("gh run list: %w", err)
	}
	if len(runs) == 0 {
		return nil, nil
	}
	return &runs[0], nil
}

// Jobs returns the jobs of a run, or nil when they cannot be read.
func Jobs(dir string, runID int64) ([]Job, error) {
	out, err := run(dir, "run", "view", fmt.Sprint(runID), "--json", "jobs")
	if err != nil {
		return nil, fmt.Errorf("gh run view: %w", err)
	}
	var parsed struct {
		Jobs []Job `json:"jobs"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("gh run view: %w", err)
	}
	return parsed.Jobs, nil
}

// run executes gh inside the repo directory and answers its stdout.
func run(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// WaitForRun polls briefly for the run a just-pushed tag triggers, because the
// tag arriving at GitHub and the run appearing are seconds apart. Bounded, and
// a timeout answers (nil, nil): the caller says "not seen yet" and moves on.
func WaitForRun(dir, tag string, tries int, gap time.Duration) (*Run, error) {
	for i := 0; i < tries; i++ {
		r, err := Probe(dir, tag)
		if err == nil && r != nil {
			return r, nil
		}
		if i < tries-1 {
			time.Sleep(gap)
		}
	}
	return nil, nil
}

// Verdict renders the run's state the way a waiting release-cutter reads it.
// An in-flight RC whose job failed reads "finished and lost", not "in flight".
func Verdict(r *Run, jobs []Job, now time.Time) string {
	switch {
	case r == nil:
		return ""
	case r.Status != "completed":
		v := "in progress"
		if r.Status == "queued" {
			v = "queued"
		}
		if j := runningJob(jobs, now); j != "" {
			return v + " — " + j
		}
		return v
	case r.Conclusion == "success":
		return "succeeded " + age(r.CreatedAt, now)
	default:
		v := strings.ToUpper(r.Conclusion)
		if v == "" {
			v = "FINISHED"
		}
		return v + " " + age(r.CreatedAt, now) + failedJob(jobs)
	}
}

// runningJob names the job currently occupying the run, with its runner and
// how long it has been there — the three things someone waiting on a release
// actually asks for (GRIT-6872 comment: report the JOB, not just the run).
func runningJob(jobs []Job, now time.Time) string {
	for _, j := range jobs {
		if j.Status == "in_progress" {
			s := fmt.Sprintf("%s on %s", j.Name, runner(j))
			if d, ok := elapsed(j.StartedAt, now); ok {
				s = fmt.Sprintf("%s (%s elapsed)", s, d)
			}
			return s
		}
	}
	for _, j := range jobs {
		if j.Status == "queued" {
			return j.Name + " (queued)"
		}
	}
	return ""
}

// failedJob names the first genuinely failed job, so a red run says WHY it is
// red rather than making the caller open the log to find out.
func failedJob(jobs []Job) string {
	for _, j := range jobs {
		if j.Conclusion == "failure" {
			return " — " + j.Name + " failed"
		}
	}
	return ""
}

func runner(j Job) string {
	if j.RunnerName != "" {
		return j.RunnerName
	}
	return "a runner"
}

// age renders how long ago a run finished, from its createdAt as a coarse
// proxy — the API's updatedAt is not worth a second call's failure modes.
func age(iso string, now time.Time) string {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return ""
	}
	d := now.Sub(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%d minutes ago", int(d.Minutes()))
	default:
		return fmt.Sprintf("%d hours ago", int(d.Hours()))
	}
}

func elapsed(iso string, now time.Time) (string, bool) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return "", false
	}
	d := now.Sub(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds())), true
	}
	return fmt.Sprintf("%dm", int(d.Minutes())), true
}
