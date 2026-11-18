package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// runUpdate fetches the new version of the repository (git pull, fast-forward
// only), builds it, and reinstalls the Mac app when its code changed. The
// launcher restarts the server on the new binary by itself.
func runUpdate(args []string) error {
	c := newCLI()
	if _, err := os.Stat(filepath.Join(c.repo, ".git")); err != nil {
		return errors.New(c.tr.T("cli.update.noRepo", "path", c.repo))
	}
	head := func() string {
		out, _ := exec.Command("git", "-C", c.repo, "rev-parse", "HEAD").Output()
		return strings.TrimSpace(string(out))
	}
	before := head()
	c.say("cli.update.pulling")
	if err := c.run("git", "pull", "--ff-only"); err != nil {
		return err
	}
	after := head()
	if before == after {
		c.ok("cli.update.upToDate")
		return nil
	}
	c.say("cli.update.building")
	if err := c.run("make", "build"); err != nil {
		return err
	}
	changed, _ := exec.Command("git", "-C", c.repo, "diff", "--name-only", before, after, "--", "macos/").Output()
	if len(strings.TrimSpace(string(changed))) > 0 {
		c.say("cli.update.appChanged")
		if err := c.run("make", "app"); err != nil {
			return err
		}
	}
	c.ok("cli.update.done")
	return nil
}
