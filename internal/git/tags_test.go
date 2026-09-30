package git_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/gronitab/git-release/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "init"},
	}
	for _, args := range cmds {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		require.NoError(t, err, string(out))
	}
	return dir
}

func addTag(t *testing.T, dir, tag string) {
	t.Helper()
	c := exec.Command("git", "tag", tag)
	c.Dir = dir
	out, err := c.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestListLocalTags_Empty(t *testing.T) {
	dir := makeRepo(t)
	tags, err := git.ListLocalTags(dir, "v")
	require.NoError(t, err)
	assert.Empty(t, tags)
}

func TestListLocalTags(t *testing.T) {
	dir := makeRepo(t)
	addTag(t, dir, "v1.2.3")
	addTag(t, dir, "v1.2.3-rc")
	addTag(t, dir, "v1.3.0-rc.2")
	addTag(t, dir, "not-a-release") // should be filtered out

	tags, err := git.ListLocalTags(dir, "v")
	require.NoError(t, err)

	tagStrings := make([]string, len(tags))
	for i, tag := range tags {
		tagStrings[i] = tag.String("v")
	}

	assert.ElementsMatch(t, []string{"v1.2.3", "v1.2.3-rc", "v1.3.0-rc.2"}, tagStrings)
}

func TestListLocalTags_CustomPrefix(t *testing.T) {
	dir := makeRepo(t)
	addTag(t, dir, "rel1.0.0")
	addTag(t, dir, "v1.0.0") // wrong prefix, should be filtered

	tags, err := git.ListLocalTags(dir, "rel")
	require.NoError(t, err)
	require.Len(t, tags, 1)
	assert.Equal(t, "rel1.0.0", tags[0].String("rel"))
}

func TestTagCommitInfo(t *testing.T) {
	dir := makeRepo(t)
	addTag(t, dir, "v1.0.0")

	tags, err := git.ListLocalTagsWithInfo(dir, "v")
	require.NoError(t, err)
	require.Len(t, tags, 1)

	info := tags[0]
	assert.Equal(t, "v1.0.0", info.Tag.String("v"))
	assert.NotEmpty(t, info.Hash)
	assert.Len(t, info.Hash, 7)
	assert.False(t, info.Date.IsZero())
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	c := exec.Command("git", args...)
	c.Dir = dir
	out, err := c.CombinedOutput()
	require.NoError(t, err, string(out))
	return strings.TrimSpace(string(out))
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	gitOut(t, dir, args...)
}

// TestPushBranch_FastForwardLandsCommits pins grit-4h2uo: PushBranch must land
// local-only commits on the remote branch so a later tag never points at
// commits missing from origin.
func TestPushBranch_FastForwardLandsCommits(t *testing.T) {
	remote := t.TempDir()
	gitRun(t, remote, "init", "--bare")
	dir := makeRepo(t)
	gitRun(t, dir, "remote", "add", "origin", remote)
	branch, err := git.CurrentBranch(dir)
	require.NoError(t, err)

	// A local commit that isn't on the remote yet.
	gitRun(t, dir, "commit", "--allow-empty", "-m", "feat: local only")
	head := gitOut(t, dir, "rev-parse", "HEAD")

	require.NoError(t, git.PushBranch(dir, "origin", branch, false))

	remoteHead := gitOut(t, remote, "rev-parse", "refs/heads/"+branch)
	assert.Equal(t, head, remoteHead, "PushBranch must land HEAD on the remote branch")
}

// TestPushBranch_NonFastForwardHardFails: a diverged branch must error, never
// force — the tool never rewrites remote history.
func TestPushBranch_NonFastForwardHardFails(t *testing.T) {
	remote := t.TempDir()
	gitRun(t, remote, "init", "--bare")
	dir := makeRepo(t)
	gitRun(t, dir, "remote", "add", "origin", remote)
	branch, err := git.CurrentBranch(dir)
	require.NoError(t, err)
	require.NoError(t, git.PushBranch(dir, "origin", branch, false))

	// A second clone advances the remote branch.
	other := t.TempDir()
	gitRun(t, other, "clone", remote, ".")
	gitRun(t, other, "config", "user.email", "o@o.com")
	gitRun(t, other, "config", "user.name", "O")
	gitRun(t, other, "commit", "--allow-empty", "-m", "remote moved")
	gitRun(t, other, "push", "origin", "HEAD:"+branch)

	// The original repo commits on top of its now-stale base → non-ff.
	gitRun(t, dir, "commit", "--allow-empty", "-m", "diverging local")
	err = git.PushBranch(dir, "origin", branch, false)
	require.Error(t, err, "a non-fast-forward push must hard-fail, not force")
}

func TestPushBranch_DryRunNoOp(t *testing.T) {
	dir := makeRepo(t) // no remote configured
	require.NoError(t, git.PushBranch(dir, "origin", "main", true), "dry-run must not touch the network")
}
