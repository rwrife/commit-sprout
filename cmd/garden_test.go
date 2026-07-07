package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPlural(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{0, "s"},
		{1, ""},
		{2, "s"},
		{100, "s"},
	}
	for _, c := range cases {
		if got := plural(c.n); got != c.want {
			t.Errorf("plural(%d) = %q; want %q", c.n, got, c.want)
		}
	}
}

// mkRepo creates a directory with a .git marker so isRepo/scanForRepos treat it
// as a repository without needing a real git init.
func mkRepo(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkRepo %s: %v", name, err)
	}
	return dir
}

func TestIsRepo(t *testing.T) {
	root := t.TempDir()
	repo := mkRepo(t, root, "withgit")
	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	if !isRepo(repo) {
		t.Errorf("isRepo(%q) = false; want true", repo)
	}
	if isRepo(plain) {
		t.Errorf("isRepo(%q) = true; want false", plain)
	}
	if isRepo(filepath.Join(root, "does-not-exist")) {
		t.Errorf("isRepo(missing) = true; want false")
	}
}

// TestIsRepoGitFile covers the worktree/submodule case where .git is a file
// rather than a directory.
func TestIsRepoGitFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "wt")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../real\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !isRepo(dir) {
		t.Errorf("isRepo with .git file = false; want true")
	}
}

func TestScanForReposFindsOnlyRepos(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root, "alpha")
	mkRepo(t, root, "beta")
	// A non-repo dir and a stray file that must be ignored.
	if err := os.MkdirAll(filepath.Join(root, "notrepo"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "afile"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := scanForRepos(root)
	if err != nil {
		t.Fatalf("scanForRepos: %v", err)
	}
	want := []string{filepath.Join(root, "alpha"), filepath.Join(root, "beta")}
	if len(got) != len(want) {
		t.Fatalf("scanForRepos = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("scanForRepos[%d] = %q; want %q (should be sorted)", i, got[i], want[i])
		}
	}
}

func TestScanForReposMissingDirErrors(t *testing.T) {
	_, err := scanForRepos(filepath.Join(t.TempDir(), "nope"))
	if err == nil {
		t.Error("scanForRepos on a missing dir: want error, got nil")
	}
}

// TestResolveGardenReposArgsWin verifies explicit path args take precedence over
// both --scan and any saved set.
func TestResolveGardenReposArgsWin(t *testing.T) {
	// Set --scan to something that would otherwise be used; args should win.
	prevScan := gardenScan
	gardenScan = t.TempDir()
	defer func() { gardenScan = prevScan }()

	args := []string{"/explicit/a", "/explicit/b"}
	got, err := resolveGardenRepos(args)
	if err != nil {
		t.Fatalf("resolveGardenRepos: %v", err)
	}
	if len(got) != 2 || got[0] != args[0] || got[1] != args[1] {
		t.Errorf("resolveGardenRepos(args) = %v; want %v", got, args)
	}
}

// TestResolveGardenReposScanBranch verifies that with no args, --scan drives the
// result.
func TestResolveGardenReposScanBranch(t *testing.T) {
	root := t.TempDir()
	mkRepo(t, root, "one")
	mkRepo(t, root, "two")

	prevScan := gardenScan
	gardenScan = root
	defer func() { gardenScan = prevScan }()

	got, err := resolveGardenRepos(nil)
	if err != nil {
		t.Fatalf("resolveGardenRepos: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("resolveGardenRepos(--scan) = %v; want 2 repos", got)
	}
}

func TestTerminalWidthFromColumns(t *testing.T) {
	t.Setenv("COLUMNS", "123")
	if w := terminalWidth(nil); w != 123 {
		t.Errorf("terminalWidth() = %d; want 123 from COLUMNS", w)
	}
	// Non-numeric COLUMNS falls back to 0 (renderer default).
	t.Setenv("COLUMNS", "wide")
	if w := terminalWidth(nil); w != 0 {
		t.Errorf("terminalWidth() with bad COLUMNS = %d; want 0", w)
	}
}
