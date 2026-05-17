package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// Version and RepoURL are overridden at build time via -ldflags (see
// .github/workflows/release.yml). The "dev" sentinel skips the GitHub update
// check for local builds — there's no point asking "is your dev build out of
// date" when there's no upstream to compare to.
var (
	Version = "dev"
	RepoURL = "https://github.com/ohnotnow/claude-skill-profiles"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the csp version and check for updates",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Printf("csp version %s\n", Version)
		if Version == "dev" {
			return
		}
		latest, err := checkLatestRelease()
		if err != nil {
			// Silent on network failure — don't make the user feel like
			// something's broken when their wifi is just being flaky.
			return
		}
		if isNewer(latest, Version) {
			cmd.Printf("A newer version (%s) is available.\n", latest)
			cmd.Printf("Visit %s/releases/latest to update.\n", RepoURL)
		} else {
			cmd.Println("You are running the latest version.")
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// ghRelease is the slice of GitHub's /releases/latest payload that csp cares
// about — just the tag for version comparison.
type ghRelease struct {
	TagName string `json:"tag_name"`
}

// checkLatestRelease asks GitHub for the latest published release of the
// current RepoURL and returns the tag. Five-second timeout so an unreachable
// network doesn't make `csp version` feel hung.
func checkLatestRelease() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(buildAPIURL(RepoURL))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	return rel.TagName, nil
}

// buildAPIURL converts a github.com user-facing URL into the API endpoint for
// its latest release.
func buildAPIURL(repoURL string) string {
	path := strings.TrimPrefix(repoURL, "https://github.com/")
	path = strings.TrimPrefix(path, "http://github.com/")
	path = strings.TrimSuffix(path, "/")
	return "https://api.github.com/repos/" + path + "/releases/latest"
}

// isNewer reports whether latest > current using strict semver comparison.
// Either tag may carry a "v" prefix or not. Non-semver inputs are treated as
// "not newer" so a malformed tag never falsely prompts an upgrade.
func isNewer(latest, current string) bool {
	parse := func(v string) (int, int, int, bool) {
		v = strings.TrimPrefix(v, "v")
		parts := strings.Split(v, ".")
		if len(parts) != 3 {
			return 0, 0, 0, false
		}
		major, err1 := strconv.Atoi(parts[0])
		minor, err2 := strconv.Atoi(parts[1])
		patch, err3 := strconv.Atoi(parts[2])
		if err1 != nil || err2 != nil || err3 != nil {
			return 0, 0, 0, false
		}
		return major, minor, patch, true
	}
	lMaj, lMin, lPat, lok := parse(latest)
	cMaj, cMin, cPat, cok := parse(current)
	if !lok || !cok {
		return false
	}
	if lMaj != cMaj {
		return lMaj > cMaj
	}
	if lMin != cMin {
		return lMin > cMin
	}
	return lPat > cPat
}
