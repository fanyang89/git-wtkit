package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the actual pins of a populated project without writing to its worktrees.
func TestOfflineCheckout(t *testing.T) {
	source := os.Getenv("WTKIT_TEST_PROJECT")
	if source == "" {
		t.Skip("set WTKIT_TEST_PROJECT to a worktree with locally available modules")
	}
	t.Setenv("GIT_ALLOW_PROTOCOL", "file")
	t.Setenv("GIT_LFS_SKIP_SMUDGE", "1")
	dir := t.TempDir()
	tree := filepath.Join(dir, "checkout")
	if _, err := git(dir, "clone", "--shared", "--no-checkout", "--", source, tree); err != nil {
		t.Fatal(err)
	}
	head, err := git(source, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = git(tree, "checkout", "--detach", head); err != nil {
		t.Fatal(err)
	}
	donors, err := peers(source)
	if err != nil {
		t.Fatal(err)
	}
	// Some nested modules may never have been downloaded. Still verify every
	// direct pin, so partial coverage cannot be mistaken for an offline pass.
	if err = syncTree(tree, "", append([]string{source}, donors...), 0); err != nil {
		t.Log(err)
	}
	status, err := git(tree, "submodule", "status")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(status, "\n") {
		if !strings.HasPrefix(line, " ") {
			t.Fatalf("direct pin not checked out: %s", line)
		}
	}
}
