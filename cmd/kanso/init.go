package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/setup"
)

// runInit chooses the folder (a name in Documents, or a path), creates it and
// its .kanso, saves the config, installs the LaunchAgent, waits for the server
// and opens the app. An existing folder is used as it is: nothing in it moves.
func runInit(args []string) error {
	c := newCLI()
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "answer yes: create the folder without asking")
	port := fs.Int("port", c.cfg.Port, "HTTP port")
	_ = fs.Parse(args)

	fmt.Println()
	c.say("cli.init.hello")
	fmt.Println()
	answer := fs.Arg(0)
	if answer == "" {
		def := c.cfg.Root
		if def == "" {
			def = filepath.Join("~", "Documents", c.tr.T("cli.init.defaultName"))
		}
		answer = c.ask(c.tr.T("cli.init.ask"), def)
	}
	root, err := resolveFolder(c.home, answer)
	if err != nil {
		return err
	}
	switch st, err := os.Stat(root); {
	case err == nil && !st.IsDir():
		return errors.New(c.tr.T("cli.init.notFolder", "path", root))
	case err == nil:
		c.ok("cli.init.existing", "path", root)
	default:
		if !*yes && !c.confirm(c.tr.T("cli.init.create", "path", root)) {
			c.say("cli.init.cancelled")
			return nil
		}
		if err := os.MkdirAll(root, 0o755); err != nil {
			return err
		}
		c.ok("cli.init.created", "path", root)
	}
	if err := os.MkdirAll(filepath.Join(root, brand.DataDir), 0o755); err != nil {
		return err
	}
	c.cfg = config.Config{Root: root, Port: *port}
	if err := config.Save(c.home, c.cfg); err != nil {
		return err
	}
	if err := setup.InstallAgent(c.paths, c.binDir, c.repo); err != nil {
		return err
	}
	c.ok("cli.init.agent")
	c.say("cli.init.access")
	c.say("cli.init.waiting")
	for i := 0; i < 30 && !c.serverUp(); i++ {
		time.Sleep(500 * time.Millisecond)
	}
	if !c.serverUp() {
		c.warn("cli.init.notReady")
		return nil
	}
	c.ok("cli.init.ready", "port", c.cfg.Port)
	target := fmt.Sprintf("http://localhost:%d", c.cfg.Port)
	if _, err := os.Stat(c.paths.App); err == nil {
		target = c.paths.App
	}
	_ = exec.Command("open", target).Run()
	fmt.Println()
	c.say("cli.init.next")
	return nil
}

// resolveFolder turns an answer into an absolute folder: « ~/… » and « /… »
// are paths, a bare name is a folder of Documents.
func resolveFolder(home, answer string) (string, error) {
	answer = strings.TrimSpace(answer)
	switch {
	case answer == "":
		return "", errors.New("no folder")
	case answer == "~" || strings.HasPrefix(answer, "~/"):
		answer = filepath.Join(home, strings.TrimPrefix(answer, "~"))
	case !strings.Contains(answer, "/"):
		answer = filepath.Join(home, "Documents", answer)
	}
	abs, err := filepath.Abs(answer)
	if err != nil {
		return "", err
	}
	if abs == home || abs == "/" {
		return "", fmt.Errorf("refusing %s: choose a folder of its own", abs)
	}
	return abs, nil
}
