//go:build integration

package integration_test

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

var (
	gitBinary       string
	clonepathBinary string
	testBinaryDir   string
)

func TestMain(m *testing.M) {
	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "integration tests require macOS or Linux")
		os.Exit(0)
	}

	var err error
	gitBinary, err = exec.LookPath("git")
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration tests require Git:", err)
		os.Exit(1)
	}

	testBinaryDir, err = os.MkdirTemp("", "git-clonepath-integration-bin-*")
	if err != nil {
		fmt.Fprintln(os.Stderr, "create integration binary directory:", err)
		os.Exit(1)
	}

	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		fmt.Fprintln(os.Stderr, "locate integration test source")
		_ = os.RemoveAll(testBinaryDir)
		os.Exit(1)
	}
	repositoryRoot := filepath.Dir(filepath.Dir(filename))
	clonepathBinary = filepath.Join(testBinaryDir, "git-clonepath")

	build := exec.Command("go", "build", "-trimpath", "-o", clonepathBinary, "./cmd/git-clonepath")
	build.Dir = repositoryRoot
	if output, buildErr := build.CombinedOutput(); buildErr != nil {
		fmt.Fprintf(os.Stderr, "build git-clonepath: %v\n%s", buildErr, output)
		_ = os.RemoveAll(testBinaryDir)
		os.Exit(1)
	}

	code := m.Run()
	if cleanupErr := os.RemoveAll(testBinaryDir); cleanupErr != nil {
		fmt.Fprintln(os.Stderr, "remove integration binary directory:", cleanupErr)
		if code == 0 {
			code = 1
		}
	}
	os.Exit(code)
}

func TestCloneWithRealGit(t *testing.T) {
	fixture := newFixture(t, true, true)

	code, stdout, stderr := run(t, fixture.env, clonepathBinary,
		"--depth", "1", fixture.remote,
	)
	if code != 0 {
		t.Fatalf("git-clonepath exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	assertSuccessfulClone(t, fixture)
}

func TestGitExternalSubcommandHonorsCommandScopedRoot(t *testing.T) {
	fixture := newFixture(t, false, true)
	overrideRoot := filepath.Join(fixture.base, "override-root")
	if err := os.Mkdir(overrideRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	fixture.root = overrideRoot

	env := append([]string{}, fixture.env...)
	env = replaceEnv(env, "PATH", testBinaryDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	code, stdout, stderr := run(t, env, gitBinary,
		"-c", "clonepath.root="+overrideRoot,
		"clonepath", "--depth=1", fixture.remote,
	)
	if code != 0 {
		t.Fatalf("git clonepath exited %d\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	assertSuccessfulClone(t, fixture)
}

func TestFailedClonePropagatesGitExitAndCleansParents(t *testing.T) {
	fixture := newFixture(t, true, false)

	code, _, stderr := run(t, fixture.env, clonepathBinary, fixture.remote)
	if code != 128 {
		t.Fatalf("git-clonepath exited %d, want Git's 128\nstderr:\n%s", code, stderr)
	}
	hostDirectory := filepath.Join(fixture.root, "Example.test")
	if _, err := os.Stat(hostDirectory); !os.IsNotExist(err) {
		t.Fatalf("failed clone left its created hierarchy at %q: %v", hostDirectory, err)
	}
	if _, err := os.Stat(fixture.root); err != nil {
		t.Fatalf("failed clone removed clone root: %v", err)
	}
}

func TestExistingDestinationIsRejectedBeforeClone(t *testing.T) {
	fixture := newFixture(t, true, true)
	destination := fixture.destination()
	if err := os.MkdirAll(destination, 0o755); err != nil {
		t.Fatal(err)
	}

	code, _, stderr := run(t, fixture.env, clonepathBinary, fixture.remote)
	if code != 1 {
		t.Fatalf("git-clonepath exited %d, want 1\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "already exists") {
		t.Fatalf("stderr does not explain the conflict:\n%s", stderr)
	}
}

func TestMissingRootShowsSetupCommand(t *testing.T) {
	fixture := newFixture(t, false, true)

	code, _, stderr := run(t, fixture.env, clonepathBinary, fixture.remote)
	if code != 1 {
		t.Fatalf("git-clonepath exited %d, want 1\nstderr:\n%s", code, stderr)
	}
	if !strings.Contains(stderr, "git config --global clonepath.root ~/Developer") {
		t.Fatalf("stderr does not include setup command:\n%s", stderr)
	}
}

type fixture struct {
	base   string
	root   string
	config string
	remote string
	env    []string
}

func newFixture(t *testing.T, configureRoot, remoteExists bool) fixture {
	t.Helper()

	base := t.TempDir()
	home := filepath.Join(base, "home")
	root := filepath.Join(base, "clone-root")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(home, ".gitconfig")
	env := isolatedGitEnv(home, configFile)
	remote := "https://Example.test/Group/repo.git"
	bareRepository := filepath.Join(base, "remote.git")
	if remoteExists {
		createBareRepository(t, env, base, bareRepository)
	}

	rewrite := (&url.URL{Scheme: "file", Path: bareRepository}).String()
	setConfig(t, env, configFile, "url."+rewrite+".insteadOf", remote)
	if configureRoot {
		setConfig(t, env, configFile, "clonepath.root", root)
	}

	return fixture{
		base:   base,
		root:   root,
		config: configFile,
		remote: remote,
		env:    env,
	}
}

func createBareRepository(t *testing.T, env []string, base, bareRepository string) {
	t.Helper()

	source := filepath.Join(base, "source")
	mustRun(t, env, gitBinary, "init", "--quiet", source)
	mustRun(t, env, gitBinary,
		"-C", source,
		"-c", "user.name=Clonepath Integration",
		"-c", "user.email=clonepath@example.test",
		"commit", "--allow-empty", "--quiet", "-m", "initial",
	)
	mustRun(t, env, gitBinary, "clone", "--quiet", "--bare", source, bareRepository)
}

func setConfig(t *testing.T, env []string, configFile, key, value string) {
	t.Helper()
	mustRun(t, env, gitBinary, "config", "--file", configFile, key, value)
}

func assertSuccessfulClone(t *testing.T, fixture fixture) {
	t.Helper()

	destination := fixture.destination()
	code, stdout, stderr := run(t, fixture.env, gitBinary,
		"-C", destination, "rev-parse", "--is-inside-work-tree",
	)
	if code != 0 || strings.TrimSpace(stdout) != "true" {
		t.Fatalf("destination is not a work tree (exit %d)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}

	code, stdout, stderr = run(t, fixture.env, gitBinary,
		"-C", destination, "config", "--get", "remote.origin.url",
	)
	if code != 0 || strings.TrimSpace(stdout) != fixture.remote {
		t.Fatalf("origin was not preserved (exit %d)\nstdout:\n%s\nstderr:\n%s", code, stdout, stderr)
	}
}

func (f fixture) destination() string {
	return filepath.Join(f.root, "Example.test", "Group", "repo")
}

func isolatedGitEnv(home, configFile string) []string {
	env := make([]string, 0, len(os.Environ())+3)
	for _, item := range os.Environ() {
		key, _, _ := strings.Cut(item, "=")
		if key == "HOME" || strings.HasPrefix(key, "GIT_CONFIG_") {
			continue
		}
		env = append(env, item)
	}
	env = append(env,
		"HOME="+home,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+configFile,
	)
	return env
}

func replaceEnv(env []string, key, value string) []string {
	prefix := key + "="
	for i, item := range env {
		if strings.HasPrefix(item, prefix) {
			env[i] = prefix + value
			return env
		}
	}
	return append(env, prefix+value)
}

func mustRun(t *testing.T, env []string, name string, args ...string) {
	t.Helper()
	code, stdout, stderr := run(t, env, name, args...)
	if code != 0 {
		t.Fatalf("%s %v exited %d\nstdout:\n%s\nstderr:\n%s", name, args, code, stdout, stderr)
	}
}

func run(t *testing.T, env []string, name string, args ...string) (int, string, string) {
	t.Helper()

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Env = env
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return 0, stdout.String(), stderr.String()
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode(), stdout.String(), stderr.String()
	}
	t.Fatalf("start %s %v: %v", name, args, err)
	return 0, "", ""
}
