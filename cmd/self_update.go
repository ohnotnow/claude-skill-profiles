package cmd

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var (
	selfUpdateCheck bool
	selfUpdateYes   bool
)

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Download the latest release and replace the running binary",
	Long: `Fetch the latest GitHub release for the current platform, verify it against
the published SHA256SUMS, and atomically replace the running csp binary.

Use --check to report whether an update is available without downloading
(exit 1 silently when newer is available, 0 when up to date).

Use --yes / -y to skip the confirmation prompt.

Dev builds skip the network entirely — there's no upstream to compare against,
and replacing a hand-built binary with a release one is almost never wanted.

Self-update detects Homebrew and 'go install' installations and redirects to
the right package-manager command instead of overwriting them silently.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("resolve executable: %w", err)
		}
		if real, lerr := filepath.EvalSymlinks(exe); lerr == nil {
			exe = real
		}
		home, _ := os.UserHomeDir()

		err = runSelfUpdate(selfUpdateConfig{
			apiURL:         buildAPIURL(RepoURL),
			httpClient:     &http.Client{Timeout: 60 * time.Second},
			targetPath:     exe,
			osName:         runtime.GOOS,
			arch:           runtime.GOARCH,
			stdin:          os.Stdin,
			stdout:         cmd.OutOrStdout(),
			stderr:         cmd.ErrOrStderr(),
			currentVersion: Version,
			gopath:         os.Getenv("GOPATH"),
			home:           home,
			checkOnly:      selfUpdateCheck,
			assumeYes:      selfUpdateYes,
		})
		// In --check mode "update available" is signalled with a silent exit
		// 1 so scripts can react to it. The user-facing message has already
		// been printed by runSelfUpdate; we just want the exit code now.
		if errors.Is(err, errUpdateAvailable) {
			os.Exit(1)
		}
		return err
	},
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheck, "check", false,
		"report whether an update is available without downloading")
	selfUpdateCmd.Flags().BoolVarP(&selfUpdateYes, "yes", "y", false,
		"skip the confirmation prompt")
	rootCmd.AddCommand(selfUpdateCmd)
}

// errUpdateAvailable is a sentinel error: returned by --check mode when a
// newer version exists. The RunE wrapper translates it into os.Exit(1).
var errUpdateAvailable = errors.New("update available")

// selfUpdateConfig bundles the moving parts of a self-update run so tests
// can substitute an httptest server, a temp file standing in for the running
// binary, and a known platform tuple.
type selfUpdateConfig struct {
	apiURL         string
	httpClient     *http.Client
	targetPath     string
	osName         string
	arch           string
	stdin          io.Reader
	stdout         io.Writer
	stderr         io.Writer
	currentVersion string
	gopath         string
	home           string
	checkOnly      bool
	assumeYes      bool
}

func runSelfUpdate(cfg selfUpdateConfig) error {
	// Dev builds short-circuit before any network call.
	if cfg.currentVersion == "dev" {
		if cfg.checkOnly {
			fmt.Fprintln(cfg.stdout, "csp is a dev build — version check skipped.")
			return nil
		}
		fmt.Fprintln(cfg.stdout, "csp is a dev build — self-update is disabled.")
		fmt.Fprintln(cfg.stdout, "Rebuild from source, or install a release binary.")
		return nil
	}

	rel, err := fetchLatestRelease(cfg.httpClient, cfg.apiURL)
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	if !isNewer(rel.TagName, cfg.currentVersion) {
		fmt.Fprintf(cfg.stdout, "Already up to date (%s).\n", cfg.currentVersion)
		return nil
	}

	// --check stops here: report and return the sentinel error.
	if cfg.checkOnly {
		fmt.Fprintf(cfg.stdout, "csp %s is installed; %s is available.\n",
			cfg.currentVersion, rel.TagName)
		return errUpdateAvailable
	}

	// Before we touch anything, ask whether this binary even belongs to us.
	// Homebrew and 'go install' manage their own copies, and a self-update
	// would silently sidestep them.
	pmHint, pmAct := detectPackageManager(cfg.targetPath, cfg.osName, cfg.gopath, cfg.home)
	if pmAct == pmRedirect {
		fmt.Fprintf(cfg.stdout, "csp was installed via %s.\n", pmHint.manager)
		fmt.Fprintf(cfg.stdout, "A newer version (%s) is available — update with:\n  %s\n",
			rel.TagName, pmHint.command)
		return nil
	}

	assetName := assetNameFor(cfg.osName, cfg.arch)
	if assetName == "" {
		return fmt.Errorf("no release asset for platform %s/%s", cfg.osName, cfg.arch)
	}

	binAsset, ok := findAsset(rel.Assets, assetName)
	if !ok {
		return fmt.Errorf("release %s does not include asset %q", rel.TagName, assetName)
	}
	sumsAsset, ok := findAsset(rel.Assets, "SHA256SUMS")
	if !ok {
		return fmt.Errorf("release %s does not include SHA256SUMS", rel.TagName)
	}

	// Probe write permission now, before download. Saves multi-MB pain if
	// the install location needs sudo.
	dir := filepath.Dir(cfg.targetPath)
	if !canWrite(dir) {
		return fmt.Errorf("cannot write to %s — re-run with sudo, or visit %s/releases/latest",
			dir, RepoURL)
	}

	if !cfg.assumeYes {
		ok, err := confirmUpdate(cfg, rel, assetName)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(cfg.stdout, "Aborted.")
			return nil
		}
	}

	binBytes, err := downloadAsset(cfg.httpClient, binAsset.URL)
	if err != nil {
		return fmt.Errorf("download %s: %w", assetName, err)
	}
	sumsBytes, err := downloadAsset(cfg.httpClient, sumsAsset.URL)
	if err != nil {
		return fmt.Errorf("download SHA256SUMS: %w", err)
	}

	expected, err := lookupChecksum(string(sumsBytes), assetName)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(binBytes)
	actual := hex.EncodeToString(sum[:])
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("checksum mismatch for %s: expected %s, got %s",
			assetName, expected, actual)
	}

	if pmAct == pmWarn && cfg.stderr != nil {
		fmt.Fprintf(cfg.stderr,
			"warning: csp lives in %s, which is normally managed by your system package manager.\n",
			filepath.Dir(cfg.targetPath))
		fmt.Fprintln(cfg.stderr, "self-update will sidestep that — proceeding anyway.")
	}

	if err := atomicSwap(cfg.targetPath, binBytes); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	fmt.Fprintf(cfg.stdout, "Updated to %s.\n", rel.TagName)
	return nil
}

// confirmUpdate prints a summary, the release notes, and a y/N prompt, then
// reads a single line from stdin. Default (empty input or anything not
// 'y'/'yes') is no.
func confirmUpdate(cfg selfUpdateConfig, rel *ghRelease, assetName string) (bool, error) {
	fmt.Fprintf(cfg.stdout, "csp %s -> %s\n", cfg.currentVersion, rel.TagName)
	fmt.Fprintf(cfg.stdout, "Asset: %s\n", assetName)

	body := strings.TrimSpace(rel.Body)
	if body != "" {
		fmt.Fprintln(cfg.stdout)
		fmt.Fprintln(cfg.stdout, "Release notes:")
		fmt.Fprintln(cfg.stdout, body)
	}

	fmt.Fprintln(cfg.stdout)
	fmt.Fprint(cfg.stdout, "Proceed? [y/N]: ")

	if cfg.stdin == nil {
		return false, nil
	}
	reader := bufio.NewReader(cfg.stdin)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("read confirmation: %w", err)
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// pmAction classifies what self-update should do when the running binary
// looks like it was installed by a package manager.
type pmAction int

const (
	pmNone     pmAction = iota // self-installed; proceed normally
	pmRedirect                 // package manager owns this; print hint and exit
	pmWarn                     // system bin we'll touch anyway, but warn
)

// pmHint is the human-facing pair of strings emitted alongside a redirect.
type pmHint struct {
	manager string
	command string
}

// detectPackageManager classifies path against well-known install locations.
// goos, gopath and home are passed in so tests can drive the function with
// known values without touching real environment variables.
//
// Heuristics, not gates — false positives just route users to a tool that
// already does the right thing; false negatives fall through to a normal
// self-update.
func detectPackageManager(path, goos, gopath, home string) (pmHint, pmAction) {
	p := filepath.ToSlash(path)

	if strings.HasPrefix(p, "/opt/homebrew/") ||
		strings.Contains(p, "/Cellar/") ||
		strings.Contains(p, "/homebrew/") ||
		strings.Contains(p, "/linuxbrew/") {
		return pmHint{manager: "Homebrew", command: "brew upgrade csp"}, pmRedirect
	}

	goBin := ""
	switch {
	case gopath != "":
		goBin = filepath.ToSlash(filepath.Join(gopath, "bin"))
	case home != "":
		goBin = filepath.ToSlash(filepath.Join(home, "go", "bin"))
	}
	if goBin != "" && (p == goBin || strings.HasPrefix(p, goBin+"/")) {
		return pmHint{
			manager: "'go install'",
			command: "go install github.com/ohnotnow/claude-skill-profiles@latest",
		}, pmRedirect
	}

	if goos == "linux" {
		dir := filepath.ToSlash(filepath.Dir(p))
		if dir == "/usr/bin" || dir == "/usr/local/bin" {
			return pmHint{}, pmWarn
		}
	}

	return pmHint{}, pmNone
}

// canWrite reports whether the calling process can create files in dir.
// Implemented by trying to create (and immediately remove) a tempfile — the
// only portable answer that works across POSIX and Windows.
func canWrite(dir string) bool {
	f, err := os.CreateTemp(dir, ".csp-write-test-*")
	if err != nil {
		return false
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return true
}

// assetNameFor returns the release asset filename csp publishes for a given
// GOOS/GOARCH, or "" if the platform isn't built. Mirrors the workflow's
// build matrix in .github/workflows/release.yml — keep the two in sync.
func assetNameFor(goos, goarch string) string {
	switch goos {
	case "windows":
		if goarch == "amd64" || goarch == "arm64" {
			return "csp-windows-" + goarch + ".exe"
		}
	case "darwin":
		if goarch == "amd64" || goarch == "arm64" {
			return "csp-darwin-" + goarch
		}
	case "linux":
		if goarch == "amd64" || goarch == "arm64" {
			return "csp-linux-" + goarch
		}
	}
	return ""
}

func findAsset(assets []ghAsset, name string) (ghAsset, bool) {
	for _, a := range assets {
		if a.Name == name {
			return a, true
		}
	}
	return ghAsset{}, false
}

func downloadAsset(client *http.Client, url string) ([]byte, error) {
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// lookupChecksum scans a SHA256SUMS file for the line matching name and
// returns its hex digest. Each line is '<hex><space><space-or-asterisk><name>';
// binary mode uses two spaces, text mode uses ' *'. strings.Fields flattens
// both into the same two-field split.
func lookupChecksum(sums, name string) (string, error) {
	for _, line := range strings.Split(sums, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		fname := strings.TrimPrefix(fields[1], "*")
		if fname == name {
			return fields[0], nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %q in SHA256SUMS", name)
}

// atomicSwap replaces the file at target with content. On POSIX it writes a
// sibling temp file and renames over the target — atomic at the filesystem
// level. On Windows the running .exe can't be overwritten in place, so the
// current binary is renamed to <target>.old first; that file is left behind
// for the OS or the next self-update invocation to clean up.
func atomicSwap(target string, content []byte) error {
	dir := filepath.Dir(target)
	base := filepath.Base(target)

	tmp, err := os.CreateTemp(dir, base+".new-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}

	if runtime.GOOS == "windows" {
		oldPath := target + ".old"
		_ = os.Remove(oldPath)
		if err := os.Rename(target, oldPath); err != nil {
			return err
		}
	}

	if err := os.Rename(tmpPath, target); err != nil {
		return err
	}
	cleanup = false
	return nil
}
