// Command privacy refuses a commit that would publish personal data: an
// invoice, a bank export, the app's database, or a line holding an IBAN, an
// e-mail address, a home folder, a phone or SIRET number, or a word of the
// private list kept outside the repository.
//
//	go run ./tools/privacy           # what is staged, and the commit's author (pre-commit hook)
//	go run ./tools/privacy -msg FILE # a commit message (commit-msg hook)
//	go run ./tools/privacy -all      # every file git would publish
//
// The private list is one word per line (# for a comment), in the file named by
// $PRIVACY_WORDS, or ~/.config/git/private-words: your name, your address…
// It never enters the repository.
//
// It prints where and which rule, never the value it found. A line where a
// pattern is a false alarm carries « privacy:ok »; nothing excuses a private word.
package main

import (
	"bufio"
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const allowMarker = "privacy:ok"

// Files that are personal whatever they hold: invoices, receipts, bank exports, mail.
var personalExt = map[string]bool{
	".pdf": true, ".jpg": true, ".jpeg": true, ".heic": true, ".eml": true,
	".csv": true, ".ofx": true, ".qif": true, ".xls": true, ".xlsx": true, ".numbers": true,
}

// The app's own data and the owner's renaming work.
var personalName = map[string]bool{"db.json": true, "propositions.json": true, "annuler.json": true}
var personalDir = map[string]bool{".coddyum-inc": true, "renommage": true}

var linePatterns = []struct {
	rule string
	re   *regexp.Regexp
}{
	{"IBAN", regexp.MustCompile(`\b[A-Z]{2}\d{2}(?: ?[A-Z0-9]{4}){3,7}(?: ?[A-Z0-9]{1,3})?\b`)},
	{"adresse e-mail", regexp.MustCompile(`[A-Za-z0-9._%+-]+@([A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)*\.[A-Za-z]{2,})`)},
	{"dossier personnel", regexp.MustCompile(`/Users/[^/\s"'$]+`)}, // privacy:ok
	{"numéro de téléphone", regexp.MustCompile(`(?:\+33 ?|\b0)[1-9](?:[ .]?\d{2}){4}\b`)},
	{"numéro SIRET", regexp.MustCompile(`\b\d{3} ?\d{3} ?\d{3} ?\d{5}\b`)},
}

// exampleMail is an address of a domain reserved for examples.
var exampleMail = regexp.MustCompile(`(?i)(^|\.)example\.(com|org|net|fr)$`)

type checker struct {
	words []string // lowercased
}

type finding struct {
	file string
	line int // 0: the file itself
	rule string
}

func main() {
	all := flag.Bool("all", false, "check every file git would publish, not only what is staged")
	msg := flag.String("msg", "", "check this commit message file instead")
	flag.Parse()

	words, err := loadWords(wordsFile())
	if err != nil {
		fmt.Fprintln(os.Stderr, "privacy:", err)
		os.Exit(2)
	}
	c := checker{words: words}
	var found []finding
	switch {
	case *msg != "":
		found, err = c.message(*msg)
	case *all:
		found, err = c.tree()
	default:
		found, err = c.staged()
		if err == nil {
			var who []finding
			who, err = c.author()
			found = append(found, who...)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "privacy:", err)
		os.Exit(2)
	}
	if len(words) == 0 {
		fmt.Fprintln(os.Stderr, "privacy: pas de liste de mots privés ("+wordsFile()+") : ton nom, ton adresse et ton SIRET ne sont pas cherchés")
	}
	if len(found) == 0 {
		return
	}
	sort.Slice(found, func(i, j int) bool {
		if found[i].file != found[j].file {
			return found[i].file < found[j].file
		}
		return found[i].line < found[j].line
	})
	for _, f := range found {
		if f.line == 0 {
			fmt.Fprintf(os.Stderr, "  %s — %s\n", f.file, f.rule)
		} else {
			fmt.Fprintf(os.Stderr, "  %s:%d — %s\n", f.file, f.line, f.rule)
		}
	}
	fmt.Fprintf(os.Stderr, "Refusé : %d donnée(s) personnelle(s) possible(s). Retire-les, ou écris « %s » sur la ligne si c'est une fausse alerte.\n", len(found), allowMarker)
	os.Exit(1)
}

func wordsFile() string {
	if p := os.Getenv("PRIVACY_WORDS"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "git", "private-words")
}

// loadWords reads the private list; a missing file is an empty list.
func loadWords(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var words []string
	for _, l := range strings.Split(string(data), "\n") {
		l = strings.ToLower(strings.TrimSpace(l))
		if len([]rune(l)) >= 3 && !strings.HasPrefix(l, "#") {
			words = append(words, l)
		}
	}
	return words, nil
}

// path names the rule a file breaks by its name or place alone, or "".
// A testdata folder holds made-up documents: it is exempt.
func (c checker) path(p string) string {
	segs := strings.Split(filepath.ToSlash(p), "/")
	for _, s := range segs {
		if s == "testdata" {
			return ""
		}
	}
	for _, s := range segs[:len(segs)-1] {
		if personalDir[s] {
			return "données de l'app ou du dossier perso"
		}
	}
	name := strings.ToLower(segs[len(segs)-1])
	if personalName[name] {
		return "données de l'app ou du dossier perso"
	}
	if personalExt[filepath.Ext(name)] {
		return "facture, reçu ou export bancaire"
	}
	return ""
}

// hasPrivateWord tells whether text holds a word of the private list.
func (c checker) hasPrivateWord(text string) bool {
	lower := strings.ToLower(text)
	for _, w := range c.words {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// line names the rules a line of p breaks. The private words apply everywhere;
// the patterns spare testdata, whose values are made up, and marked lines.
func (c checker) line(p, text string) []string {
	var rules []string
	if c.hasPrivateWord(text) {
		rules = append(rules, "mot de la liste privée")
	}
	if strings.Contains(text, allowMarker) || strings.Contains(filepath.ToSlash(p), "testdata/") {
		return rules
	}
	for _, lp := range linePatterns {
		for _, m := range lp.re.FindAllStringSubmatch(text, -1) {
			if lp.rule == "adresse e-mail" && exampleMail.MatchString(m[1]) {
				continue
			}
			rules = append(rules, lp.rule)
			break
		}
	}
	return rules
}

// staged checks the files and the added lines of the next commit.
func (c checker) staged() ([]finding, error) {
	names, err := git("diff", "--cached", "--name-only", "-z", "--diff-filter=ACMR")
	if err != nil {
		return nil, err
	}
	var found []finding
	for _, p := range strings.Split(strings.TrimRight(names, "\x00"), "\x00") {
		if p == "" {
			continue
		}
		if r := c.path(p); r != "" {
			found = append(found, finding{p, 0, r})
		}
	}
	diff, err := git("diff", "--cached", "-U0", "--no-color", "--no-ext-diff", "--no-prefix")
	if err != nil {
		return nil, err
	}
	file, n := "", 0
	sc := bufio.NewScanner(strings.NewReader(diff))
	sc.Buffer(make([]byte, 1024*1024), 16*1024*1024)
	for sc.Scan() {
		l := sc.Text()
		switch {
		case strings.HasPrefix(l, "+++ "):
			file = strings.TrimPrefix(l, "+++ ")
		case strings.HasPrefix(l, "@@ "):
			n = hunkStart(l)
		case strings.HasPrefix(l, "+") && file != "":
			for _, r := range c.line(file, l[1:]) {
				found = append(found, finding{file, n, r})
			}
			n++
		}
	}
	return found, sc.Err()
}

// tree checks every file git would publish, whole.
func (c checker) tree() ([]finding, error) {
	names, err := git("ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, err
	}
	var found []finding
	for _, p := range strings.Split(strings.TrimRight(names, "\x00"), "\x00") {
		if p == "" {
			continue
		}
		if r := c.path(p); r != "" {
			found = append(found, finding{p, 0, r})
			continue
		}
		data, err := os.ReadFile(p)
		if err != nil || bytes.IndexByte(data[:min(len(data), 8000)], 0) >= 0 {
			continue // deleted since, or binary
		}
		for i, l := range strings.Split(string(data), "\n") {
			for _, r := range c.line(p, l) {
				found = append(found, finding{p, i + 1, r})
			}
		}
	}
	return found, nil
}

// author checks the name and address the commit is signed with, which GitHub
// shows to everyone: only the private words apply.
func (c checker) author() ([]finding, error) {
	var found []finding
	for _, who := range []string{"GIT_AUTHOR_IDENT", "GIT_COMMITTER_IDENT"} {
		ident, err := git("var", who)
		if err != nil {
			return nil, err
		}
		if c.hasPrivateWord(ident) {
			found = append(found, finding{"signature du commit (git config user.name / user.email)", 0, "mot de la liste privée"})
			break
		}
	}
	return found, nil
}

// message checks a commit message; git's own comment lines are left out.
func (c checker) message(path string) ([]finding, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var found []finding
	for i, l := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(l, "#") {
			continue
		}
		for _, r := range c.line("message du commit", l) {
			found = append(found, finding{"message du commit", i + 1, r})
		}
	}
	return found, nil
}

// hunkStart reads the first new line of « @@ -a,b +c,d @@ ».
func hunkStart(h string) int {
	f := strings.Fields(h)
	if len(f) < 3 {
		return 0
	}
	n, _ := strconv.Atoi(strings.SplitN(strings.TrimPrefix(f[2], "+"), ",", 2)[0])
	return n
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", append([]string{"-c", "core.quotepath=off"}, args...)...).Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}
