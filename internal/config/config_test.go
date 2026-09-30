package config_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gronitab/git-release/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeRepo creates a temporary git repo for testing.
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

func setGitConfig(t *testing.T, dir, key, value string) {
	t.Helper()
	c := exec.Command("git", "config", key, value)
	c.Dir = dir
	out, err := c.CombinedOutput()
	require.NoError(t, err, string(out))
}

func TestDefaults(t *testing.T) {
	dir := makeRepo(t)
	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, "v", cfg.TagPrefix)
	assert.Equal(t, "origin", cfg.Remote)
	assert.Equal(t, "", cfg.ReleaseBranch)
	assert.Equal(t, true, cfg.RequireClean)
	assert.Equal(t, false, cfg.SignTags)
}

func TestCustomValues(t *testing.T) {
	dir := makeRepo(t)
	setGitConfig(t, dir, "release.tagPrefix", "rel")
	setGitConfig(t, dir, "release.remote", "upstream")
	setGitConfig(t, dir, "release.releaseBranch", "main")
	setGitConfig(t, dir, "release.requireClean", "false")
	setGitConfig(t, dir, "release.signTags", "true")

	cfg, err := config.Load(dir)
	require.NoError(t, err)

	assert.Equal(t, "rel", cfg.TagPrefix)
	assert.Equal(t, "upstream", cfg.Remote)
	assert.Equal(t, "main", cfg.ReleaseBranch)
	assert.Equal(t, false, cfg.RequireClean)
	assert.Equal(t, true, cfg.SignTags)
}

func TestLoadFromSubdir(t *testing.T) {
	dir := makeRepo(t)
	sub := filepath.Join(dir, "pkg", "foo")
	require.NoError(t, os.MkdirAll(sub, 0755))

	// Should still find config by walking up
	cfg, err := config.Load(sub)
	require.NoError(t, err)
	assert.Equal(t, "v", cfg.TagPrefix) // default
}

func TestNotInRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := config.Load(dir)
	assert.ErrorIs(t, err, config.ErrNotInRepo)
}
