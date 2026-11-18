package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/i18n"
	"github.com/coddyum/kanso/internal/netgate"
	"github.com/coddyum/kanso/internal/setup"
	"github.com/coddyum/kanso/web"
)

// cli is what every command needs: the user's language, their paths and the
// folders of this install (bin/ and the repository above it).
type cli struct {
	tr     i18n.Lang
	home   string
	paths  setup.Paths
	cfg    config.Config
	binDir string // where kanso and kanso-launcher are
	repo   string // the git clone they were built from
	gate   *netgate.Gate
	in     *bufio.Reader
}

func newCLI() *cli {
	home, _ := os.UserHomeDir()
	cfg, _ := config.Load(home)
	exe, _ := os.Executable()
	exe, _ = filepath.EvalSymlinks(exe)
	binDir := filepath.Dir(exe)
	catalog, err := i18n.Load(web.Files)
	if err != nil { // the language files are embedded: only a broken build lacks them
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	return &cli{
		tr: catalog.Lang(cliLang(cfg)), home: home, paths: setup.PathsOf(home), cfg: cfg,
		binDir: binDir, repo: filepath.Dir(binDir), gate: netgate.New(), in: bufio.NewReader(os.Stdin),
	}
}

// cliLang is the language chosen in the app, then the terminal's, then the
// Mac's, then English.
func cliLang(cfg config.Config) string {
	if data, err := os.ReadFile(filepath.Join(cfg.Root, brand.DataDir, "db.json")); err == nil && cfg.Root != "" {
		var db struct {
			Lang string `json:"lang"`
		}
		if json.Unmarshal(data, &db) == nil && i18n.Known(db.Lang) {
			return db.Lang
		}
	}
	for _, v := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if code := i18n.Match(strings.ReplaceAll(strings.Split(os.Getenv(v), ".")[0], "_", "-")); code != "" {
			return code
		}
	}
	if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
		list := strings.NewReplacer("(", "", ")", "", "\"", "", "\n", "", " ", "").Replace(string(out))
		if code := i18n.Match(list); code != "" {
			return code
		}
	}
	return "en"
}

// The lines a command prints: done, to look at, wrong.
func (c *cli) ok(key string, vars ...any)   { fmt.Println("  ✓", c.tr.T(key, vars...)) }
func (c *cli) warn(key string, vars ...any) { fmt.Println("  !", c.tr.T(key, vars...)) }
func (c *cli) bad(key string, vars ...any)  { fmt.Println("  ✗", c.tr.T(key, vars...)) }
func (c *cli) say(key string, vars ...any)  { fmt.Println(c.tr.T(key, vars...)) }

// ask prints a question and returns the answer, or def when it is empty.
func (c *cli) ask(question, def string) string {
	if def != "" {
		fmt.Printf("%s [%s] : ", question, def)
	} else {
		fmt.Printf("%s : ", question)
	}
	line, _ := c.in.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}

// confirm asks a yes or no question: yes only for a word of the language's
// cli.yes list (« o », « oui »…) or « y ».
func (c *cli) confirm(question string) bool {
	answer := strings.ToLower(c.ask(question+" "+c.tr.T("cli.yesNo"), ""))
	for _, yes := range append(strings.Split(c.tr.T("cli.yes"), ","), "y", "yes") {
		if yes = strings.TrimSpace(yes); yes != "" && answer == yes {
			return true
		}
	}
	return false
}

// serverUp says whether the server answers on its port.
func (c *cli) serverUp() bool {
	_, err := c.gate.Local(c.cfg.Port, "api/sig")
	return err == nil
}

// run runs a program of the install (git, make, scripts) in the repository, its output on screen.
func (c *cli) run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Stdout, cmd.Stderr = c.repo, os.Stdout, os.Stderr
	return cmd.Run()
}
