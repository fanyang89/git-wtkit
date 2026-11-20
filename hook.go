package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

const checkoutHook = `#!/bin/sh
# Installed by git-wtkit.
[ "$3" = 1 ] || exit 0
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_PREFIX
exec git wtkit setup
`

func installHook(cmd *cobra.Command, _ []string) error {
	if _, err := git(".", "rev-parse", "--show-toplevel"); err != nil {
		return err
	}
	_, err := git(".", "config", "--get", "core.hooksPath")
	var exit *exec.ExitError
	if err == nil {
		return errors.New("custom core.hooksPath; add the post-checkout invocation documented in README.md to your hook")
	}
	if !errors.As(err, &exit) || exit.ExitCode() != 1 {
		return err
	}
	path, err := git(".", "rev-parse", "--path-format=absolute", "--git-path", "hooks/post-checkout")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0755)
	if errors.Is(err, os.ErrExist) {
		body, err := os.ReadFile(path)
		if err != nil || string(body) != checkoutHook {
			return fmt.Errorf("%s already exists; leaving it unchanged", path)
		}
		st, err := os.Stat(path)
		if err != nil || st.Mode()&0111 == 0 {
			return fmt.Errorf("%s is not executable", path)
		}
	} else if err != nil {
		return err
	} else {
		_, err := f.WriteString(checkoutHook)
		if err := errors.Join(err, f.Close()); err != nil {
			os.Remove(path)
			return err
		}
	}
	fmt.Fprintln(cmd.OutOrStdout(), "Hook installed:", path)
	return nil
}
