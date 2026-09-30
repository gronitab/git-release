package commands

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/gronitab/git-release/internal/build"
	"github.com/gronitab/git-release/internal/semver"
	"github.com/gronitab/git-release/internal/ui"
)

const (
	githubRepo  = "skaramicke/git-release"
	releasesAPI = "https://api.github.com/repos/" + githubRepo + "/releases/latest"
)

func newUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update",
		Short: "Check for a newer version and install it",
		Long: `Checks GitHub for the latest release of git-release.
If a newer version is available, shows what changed and asks for confirmation
before downloading and installing.`,
		Args: cobra.NoArgs,
		RunE: runUpdate,
	}
}

type githubRelease struct {
	TagName    string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	HTMLURL    string `json:"html_url"`
	Body       string `json:"body"`
}

func runUpdate(cmd *cobra.Command, args []string) error {
	p := &ui.Printer{Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()}

	current := build.Version
	if current == "dev" {
		p.Errorf("running a dev build — cannot update")
		return fmt.Errorf("dev build")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Current version: %s\n", ui.StyleBold("v"+current))
	fmt.Fprintf(cmd.OutOrStdout(), "Checking for updates...\n")

	latest, err := fetchLatestRelease()
	if err != nil {
		p.Errorf("could not fetch latest release: %v", err)
		return err
	}

	latestVer, err := semver.ParseTag(latest.TagName, "v")
	if err != nil {
		return fmt.Errorf("could not parse latest tag %q: %w", latest.TagName, err)
	}

	currentVer, err := semver.ParseTag("v"+current, "v")
	if err != nil {
		return fmt.Errorf("could not parse current version %q: %w", current, err)
	}

	if !latestVer.GreaterThan(currentVer) {
		p.Success(fmt.Sprintf("already up to date (%s)", latest.TagName))
		return nil
	}

	// Show what's new
	fmt.Fprintf(cmd.OutOrStdout(), "\nNew version available: %s\n", ui.StyleBold(latest.TagName))
	if latest.Body != "" {
		fmt.Fprintf(cmd.OutOrStdout(), "\n%s\n", latest.Body)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "\n")

	// Confirm unless --yes
	ctx := &runContext{yes: flagYes, print: p}
	confirmed, err := confirm(ctx,
		fmt.Sprintf("Update from v%s to %s?", current, latest.TagName),
		latest.TagName,
	)
	if err != nil {
		return err
	}
	if !confirmed {
		p.Error("aborted")
		return errAbort
	}

	// Install via the bundled install script
	installURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/%s/install.sh",
		githubRepo, latest.TagName,
	)
	fmt.Fprintf(cmd.OutOrStdout(), "Installing...\n\n")

	c := exec.Command("sh", "-c", fmt.Sprintf("curl -fsSL %s | sh", installURL))
	c.Stdout = cmd.OutOrStdout()
	c.Stderr = cmd.ErrOrStderr()
	if err := c.Run(); err != nil {
		p.Errorf("install failed: %v", err)
		return err
	}

	p.Success(fmt.Sprintf("updated to %s — restart your shell if needed", latest.TagName))
	return nil
}

func fetchLatestRelease() (*githubRelease, error) {
	req, err := http.NewRequest("GET", releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}
	return &rel, nil
}
