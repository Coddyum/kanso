package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/setup"
)

// runDoctor checks, one line each, what makes Kanso work on this Mac: the
// folder, the LaunchAgent, the server and its access to the folder, the PDF
// reader, the Mac app, its notifications and its opening at login. It says
// how to fix each problem, and fails when one is found.
func runDoctor(args []string) error {
	c := newCLI()
	problems := 0
	bad := func(key string, vars ...any) {
		problems++
		c.bad(key, vars...)
	}
	fmt.Println()
	if c.cfg.Root == "" {
		bad("cli.doctor.noConfig")
		return c.doctorEnd(problems)
	}
	c.ok("cli.doctor.config", "path", config.Path(c.home))
	if st, err := os.Stat(c.cfg.Root); err != nil || !st.IsDir() {
		bad("cli.doctor.folderMissing", "path", c.cfg.Root)
	} else {
		c.ok("cli.doctor.folder", "path", c.cfg.Root)
	}
	if _, err := os.Stat(c.paths.Agent); err != nil || !setup.AgentLoaded() {
		bad("cli.doctor.agentMissing")
	} else {
		c.ok("cli.doctor.agent")
	}
	var d struct {
		Version   string `json:"version"`
		FolderOK  bool   `json:"folderOK"`
		PDFReader bool   `json:"pdfReader"`
		Downloads bool   `json:"downloads"`
		App       struct {
			Notifications string `json:"notifications"`
			LoginItem     bool   `json:"loginItem"`
			Downloads     string `json:"downloads"`
			Seen          string `json:"seen"`
		} `json:"app"`
	}
	body, err := c.gate.Local(c.cfg.Port, "api/diagnostic")
	if err == nil {
		err = json.Unmarshal(body, &d)
	}
	if err != nil {
		bad("cli.doctor.serverDown", "log", c.paths.Log)
		return c.doctorEnd(problems)
	}
	c.ok("cli.doctor.server", "port", c.cfg.Port, "version", d.Version)
	if d.FolderOK {
		c.ok("cli.doctor.access")
	} else {
		bad("cli.doctor.noAccess")
	}
	if d.PDFReader {
		c.ok("cli.doctor.pdfReader")
	} else {
		bad("cli.doctor.noPdf")
	}
	switch _, err := os.Stat(c.paths.App); {
	case err != nil:
		bad("cli.doctor.noApp")
	case d.App.Seen == "" || d.App.Seen == "0001-01-01T00:00:00Z":
		c.ok("cli.doctor.appInstalled")
		c.warn("cli.doctor.appUnknown")
	default:
		c.ok("cli.doctor.appInstalled")
		if d.App.Notifications == "authorized" || d.App.Notifications == "provisional" {
			c.ok("cli.doctor.notifications")
		} else {
			bad("cli.doctor.noNotifications")
		}
		if d.App.LoginItem {
			c.ok("cli.doctor.loginItem")
		} else {
			c.warn("cli.doctor.noLoginItem")
		}
		c.warn("cli.doctor.focus")
		switch {
		case !d.Downloads:
		case d.App.Downloads == "watching":
			c.ok("cli.doctor.downloads")
		case d.App.Downloads == "denied":
			bad("cli.doctor.downloadsDenied")
		}
	}
	if _, err := os.Stat(filepath.Join(c.cfg.Root, brand.DataDir, "db.json")); errors.Is(err, os.ErrNotExist) {
		c.warn("cli.doctor.noData")
	}
	return c.doctorEnd(problems)
}

func (c *cli) doctorEnd(problems int) error {
	fmt.Println()
	if problems == 0 {
		c.say("cli.doctor.allGood")
		return nil
	}
	return errors.New(c.tr.N("cli.doctor.problems", problems))
}
