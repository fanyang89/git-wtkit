package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func git(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(string(exit.Stderr)))
	}
	return strings.TrimSuffix(string(out), "\n"), err
}

func peers(root string) ([]string, error) {
	out, err := git(root, "worktree", "list", "--porcelain", "-z")
	var dirs []string
	for _, field := range strings.Split(out, "\x00") {
		if dir, ok := strings.CutPrefix(field, "worktree "); ok && dir != root {
			dirs = append(dirs, dir)
		}
	}
	return dirs, err
}

func repository(dir string) bool {
	if _, err := os.Lstat(filepath.Join(dir, ".git")); err != nil {
		return false
	}
	root, err := git(dir, "rev-parse", "--show-toplevel")
	return err == nil && root == dir
}

func populate(root, path, name, pin string, sources []string) (string, error) {
	dir := filepath.Join(root, path)
	for p := dir; p != root; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		if err == nil && !st.IsDir() {
			return "", fmt.Errorf("%s is not a directory", p)
		}
	}
	if repository(dir) {
		head, err := git(dir, "rev-parse", "HEAD")
		if err != nil {
			return "", err
		}
		status, err := git(dir, "status", "--porcelain", "--untracked-files=normal", "--ignore-submodules=all")
		if err != nil {
			return "", err
		}
		if status != "" {
			return "", errors.New("local changes; left untouched")
		}
		if head == pin {
			return "ready", nil
		}
		_, err = git(dir, "symbolic-ref", "--quiet", "HEAD")
		var exit *exec.ExitError
		if err == nil {
			return "", errors.New("on a local branch; left untouched")
		}
		if !errors.As(err, &exit) || exit.ExitCode() != 1 {
			return "", err
		}
		kind := "local"
		if _, err := git(dir, "--no-lazy-fetch", "cat-file", "-e", pin+"^{commit}"); err != nil {
			src := "origin"
			for _, peer := range sources {
				if _, err := git(peer, "--no-lazy-fetch", "cat-file", "-e", pin+"^{commit}"); err == nil {
					src = peer
					break
				}
			}
			if src == "origin" {
				kind = "upstream"
			}
			if _, err := git(dir, "fetch", "--no-tags", "--no-recurse-submodules", "--", src, pin); err != nil {
				return "", err
			}
		}
		_, err = git(dir, "checkout", "--no-recurse-submodules", "--no-overwrite-ignore", "--detach", pin)
		return kind, err
	}
	if _, err := git(root, "submodule", "init", "--", path); err != nil {
		return "", err
	}
	url, err := git(root, "config", "--get", "submodule."+name+".url")
	if err != nil || url == "" {
		return "", fmt.Errorf("missing URL for %q", name)
	}
	entries, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	if len(entries) != 0 {
		return "", errors.New("nonempty destination; left untouched")
	}
	var reference string
	for _, src := range sources {
		// A partial or shallow clone is not a complete offline donor.
		if shallow, err := git(src, "rev-parse", "--is-shallow-repository"); err != nil || shallow != "false" {
			continue
		}
		if partial, _ := git(src, "config", "--get-regexp", `^(extensions\.partialclone|remote\..*\.(promisor|partialclonefilter))$`); partial != "" {
			continue
		}
		if reference == "" {
			reference = src
		}
		if _, err := git(src, "cat-file", "-e", pin+"^{commit}"); err != nil {
			continue
		}
		if err := localClone(root, dir, src, url, pin); err != nil {
			return "", err
		}
		_, err := git(root, "submodule", "absorbgitdirs", "--", path)
		return "local", err
	}
	args := []string{"-c", "submodule.recurse=false", "submodule", "update", "--checkout", "--no-recommend-shallow"}
	if reference != "" {
		args = append(args, "--reference", reference, "--dissociate")
	}
	args = append(args, "--", path)
	_, err = git(root, args...)
	return "upstream", err
}

func localClone(root, dir, src, url, pin string) error {
	parent := filepath.Dir(dir)
	if err := os.MkdirAll(parent, 0755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, ".wtkit-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	if _, err = git(root, "clone", "--no-checkout", "--dissociate", "--", src, tmp); err != nil {
		return err
	}
	if _, err = git(tmp, "remote", "set-url", "origin", url); err != nil {
		return err
	}
	if _, err = git(tmp, "-c", "submodule.recurse=false", "checkout", "--detach", pin); err != nil {
		return err
	}
	// Remove only the empty placeholder, never a populated directory.
	if err = os.Remove(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return os.Rename(tmp, dir)
}

func syncTree(root, prefix string, others []string, depth int) error {
	if depth > 32 {
		return errors.New("submodule nesting exceeds 32 levels")
	}
	index, err := git(root, "ls-files", "--stage", "-z")
	if err != nil || !(strings.HasPrefix(index, "160000 ") || strings.Contains(index, "\x00160000 ")) {
		return err
	}
	cfg, err := git(root, "config", "-z", "--file", ".gitmodules", "--get-regexp", `^submodule\..*\.path$`)
	if err != nil {
		return fmt.Errorf("cannot read submodule paths: %w", err)
	}
	names := map[string]string{}
	for _, entry := range strings.Split(cfg, "\x00") {
		key, path, ok := strings.Cut(entry, "\n")
		if ok {
			names[path] = strings.TrimSuffix(strings.TrimPrefix(key, "submodule."), ".path")
		}
	}
	var failures []error
	for _, entry := range strings.Split(index, "\x00") {
		meta, path, _ := strings.Cut(entry, "\t")
		if !strings.HasPrefix(meta, "160000 ") {
			continue
		}
		fields := strings.Fields(meta)
		label := filepath.Join(prefix, path)
		if fields[2] != "0" || names[path] == "" || !filepath.IsLocal(path) {
			failures = append(failures, fmt.Errorf("%s: unresolved gitlink or missing module name", label))
			continue
		}
		var sources []string
		for _, peer := range others {
			if src := filepath.Join(peer, path); repository(src) {
				sources = append(sources, src)
			}
		}
		kind, err := populate(root, path, names[path], fields[1], sources)
		if err == nil {
			fmt.Printf("%-8s %s\n", kind, label)
			err = syncTree(filepath.Join(root, path), label, sources, depth+1)
		}
		if err != nil {
			failures = append(failures, fmt.Errorf("%s: %w", label, err))
		}
	}
	return errors.Join(failures...)
}

func run() error {
	root, err := git(".", "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	others, err := peers(root)
	if err != nil {
		return err
	}
	return syncTree(root, "", others, 0)
}
