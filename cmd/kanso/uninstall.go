package main

import (
	"flag"
	"os"
	"os/exec"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/setup"
)

// runUninstall removes what Kanso put on the Mac — the LaunchAgent, the Mac
// app, the cache, the log and the settings — after a yes. The user's folder
// and its .kanso (the database, the backups) are never touched.
func runUninstall(args []string) error {
	c := newCLI()
	fs := flag.NewFlagSet("uninstall", flag.ExitOnError)
	yes := fs.Bool("yes", false, "do not ask")
	_ = fs.Parse(args)
	folder := c.cfg.Root
	if folder == "" {
		folder = "—"
	}
	if !*yes && !c.confirm(c.tr.T("cli.uninstall.confirm", "path", folder)) {
		c.say("cli.init.cancelled")
		return nil
	}
	if err := setup.UninstallAgent(c.paths); err != nil {
		return err
	}
	_ = exec.Command("osascript", "-e", `quit app "`+brand.Name+`"`).Run()
	for _, p := range []string{c.paths.App, c.paths.Cache, c.paths.Log, c.paths.Config} {
		if err := os.RemoveAll(p); err != nil {
			return err
		}
	}
	c.ok("cli.uninstall.done", "path", folder)
	c.say("cli.uninstall.repo", "path", c.repo)
	return nil
}
