package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode"
)

const web = "../../web"

func catalog(t *testing.T) *Catalog {
	t.Helper()
	c, err := Load(os.DirFS(web))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

var (
	rePlural = regexp.MustCompile(`_(zero|one|two|few|many|other)$`)
	reVar    = regexp.MustCompile(`\{\{(\w+)\}\}`)
)

// base strips the plural suffix of a key: « a.b_one » → « a.b ».
func base(key string) string { return rePlural.ReplaceAllString(key, "") }

func vars(s string) string {
	var out []string
	for _, m := range reVar.FindAllStringSubmatch(s, -1) {
		out = append(out, m[1])
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

// TestSameKeys: every language has the keys of fr.json, no other, the same
// {{variables}}, an _other for each plural, and lists of the same length.
func TestSameKeys(t *testing.T) {
	c := catalog(t)
	ref := c.Keys("fr")
	refBases := map[string]bool{}
	for k := range ref {
		refBases[base(k)] = true
	}
	for _, code := range Langs {
		dict := c.Keys(code)
		bases := map[string]bool{}
		for k, v := range dict {
			b := base(k)
			bases[b] = true
			if !refBases[b] {
				t.Errorf("%s.json: %s is not in fr.json", code, k)
				continue
			}
			if k != b {
				if _, ok := dict[b+"_other"]; !ok {
					t.Errorf("%s.json: %s has no %s_other", code, k, b)
				}
			}
			refValue := ref[k]
			if k != b {
				refValue = ref[b+"_other"]
			}
			switch v := v.(type) {
			case string:
				rs, _ := refValue.(string)
				if want := vars(rs); vars(v) != want && !(k != b && vars(v) == strings.TrimPrefix(strings.ReplaceAll(","+want, ",count", ""), ",")) {
					t.Errorf("%s.json: %s uses {{%s}}, fr.json {{%s}}", code, k, vars(v), want)
				}
				if strings.TrimSpace(v) == "" && !strings.HasPrefix(k, "format.") {
					t.Errorf("%s.json: %s is empty", code, k)
				}
			case []string:
				if rl, _ := refValue.([]string); len(v) != len(rl) {
					t.Errorf("%s.json: %s has %d items, fr.json %d", code, k, len(v), len(rl))
				}
			}
		}
		for b := range refBases {
			if !bases[b] {
				t.Errorf("%s.json: %s is missing", code, b)
			}
		}
	}
}

// used gathers the keys the code names: t("a.b") in the page, T("a.b"),
// N("a.b"), Invalid("a.b") and Key: "a.b" in Go. A key built from a variable
// (t(`a.b.${x}`), "a." + x) counts as a prefix: every key under it is used.
func used(t *testing.T) (keys, prefixes map[string]string) {
	t.Helper()
	keys, prefixes = map[string]string{}, map[string]string{}
	reJS := regexp.MustCompile("\\bt\\(\\s*[\"'`]([a-zA-Z][\\w.]*)(\\$\\{)?")
	reTernary := regexp.MustCompile("\\bt\\(\\s*[\\w.!]+\\s*\\?\\s*\"([\\w.]+)\"\\s*:\\s*\"([\\w.]+)\"")
	reAttr := regexp.MustCompile(`data-i18n(?:-title|-label)?="([\w.]+)"`)
	walkFiles(t, filepath.Join(web), []string{".js", ".html"}, func(rel string, data []byte) {
		for _, m := range reJS.FindAllStringSubmatch(string(data), -1) {
			if m[2] != "" {
				prefixes[m[1]] = rel
			} else {
				keys[m[1]] = rel
			}
		}
		for _, m := range reTernary.FindAllStringSubmatch(string(data), -1) {
			keys[m[1]], keys[m[2]] = rel, rel
		}
		for _, m := range reAttr.FindAllStringSubmatch(string(data), -1) {
			keys[m[1]] = rel
		}
	})
	// The Mac app names its menus with text("mac.quit").
	reSwift := regexp.MustCompile(`\btext\("([\w.]+)"\)`)
	walkFiles(t, "../..", []string{".swift"}, func(rel string, data []byte) {
		for _, m := range reSwift.FindAllStringSubmatch(string(data), -1) {
			keys[m[1]] = rel
		}
	})
	// install.sh reads its sentences with plutil: « -extract "cli.install.$1" », « -extract cli.init.hello ».
	reShPrefix := regexp.MustCompile(`-extract "([a-z][\w.]*)\.\$`)
	reShKey := regexp.MustCompile(`-extract "?([a-z][\w.]*[\w])"? raw`)
	walkFiles(t, "../..", []string{".sh"}, func(rel string, data []byte) {
		for _, m := range reShPrefix.FindAllStringSubmatch(string(data), -1) {
			prefixes[m[1]] = rel
		}
		for _, m := range reShKey.FindAllStringSubmatch(string(data), -1) {
			if !strings.Contains(m[1], "$") {
				keys[m[1]] = rel
			}
		}
	})
	reKey := regexp.MustCompile(`^[a-z][a-zA-Z]*(\.[a-zA-Z0-9]+)*\.?$`)
	reFile := regexp.MustCompile(`\.(json|csv|plist|log|app|pdf)$`) // « rates.json » is a file, not a key
	tops := map[string]bool{}                                       // the first words of fr.json's keys: « cli », « error »…
	for k := range catalog(t).Keys("fr") {
		tops[strings.Split(k, ".")[0]] = true
	}
	walkFiles(t, "../..", []string{".go"}, func(rel string, data []byte) {
		f, err := parser.ParseFile(token.NewFileSet(), rel, data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			if !reKey.MatchString(s) || !strings.Contains(s, ".") || !tops[strings.Split(s, ".")[0]] || reFile.MatchString(s) {
				return true
			}
			if strings.HasSuffix(s, ".") {
				prefixes[strings.TrimSuffix(s, ".")] = rel
			} else {
				keys[s] = rel
			}
			return true
		})
	})
	return keys, prefixes
}

// TestKeysExist: every key the code names is in fr.json, and every key of
// fr.json is named somewhere (a dead key is a sentence nobody sees).
func TestKeysExist(t *testing.T) {
	ref := catalog(t).Keys("fr")
	bases := map[string]bool{}
	for k := range ref {
		bases[base(k)] = true
	}
	keys, prefixes := used(t)
	for k, file := range keys {
		if !bases[k] {
			t.Errorf("%s names %s: not in fr.json", file, k)
		}
	}
	for p, file := range prefixes {
		found := false
		for b := range bases {
			found = found || strings.HasPrefix(b, p)
		}
		if !found {
			t.Errorf("%s builds keys from %s: none in fr.json", file, p)
		}
	}
	for b := range bases {
		if keys[b] != "" {
			continue
		}
		covered := false
		for p := range prefixes {
			covered = covered || strings.HasPrefix(b, p)
		}
		if !covered {
			t.Errorf("fr.json: %s is named nowhere in the code", b)
		}
	}
}

func walkFiles(t *testing.T, root string, exts []string, fn func(rel string, data []byte)) {
	t.Helper()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && (strings.HasPrefix(d.Name(), ".") || d.Name() == "bin" || d.Name() == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		for _, ext := range exts {
			if strings.HasSuffix(path, ext) && !strings.HasSuffix(path, "_test.go") {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, _ := filepath.Rel(root, path)
				fn(filepath.ToSlash(rel), data)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

var (
	reComment   = regexp.MustCompile(`(?m)^\s*//.*$|\s//\s.*$`)
	reTextNode  = regexp.MustCompile(`(?:<[a-zA-Z][^<>]*|</[a-zA-Z]+\s*)>([^<>]*)<`)
	reAttrText  = regexp.MustCompile(`\b(?:title|placeholder|aria-label|alt)="([^"]*)"`)
	reLiteralUI = regexp.MustCompile("(?:toast|promptModal)\\(\\s*[\"'`][^\"'`]*\\p{L}{2}|\\b(?:label|sub|title|text):\\s*[\"'`][^\"'`$]*\\p{L}{2}")
	reWord      = regexp.MustCompile(`\p{L}{2,}`)
	reExample   = regexp.MustCompile(`^(?:[a-z0-9-]+\.)+[a-z]{2,}$|^https?://|^Kanso$`)
)

// stripExpr removes the ${…} of a template literal, nested braces included.
func stripExpr(s string) string {
	var b strings.Builder
	depth := 0
	for i := 0; i < len(s); i++ {
		switch {
		case depth == 0 && strings.HasPrefix(s[i:], "${"):
			depth, i = 1, i+1
		case depth > 0 && s[i] == '{':
			depth++
		case depth > 0 && s[i] == '}':
			depth--
		case depth == 0:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func nonASCIILetter(s string) bool {
	for _, r := range s {
		if r > unicode.MaxASCII && unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// TestNoHardCodedText: the page writes no sentence of its own, every text
// goes through t(); the server's packages write none either.
func TestNoHardCodedText(t *testing.T) {
	walkFiles(t, web, []string{".js", ".html"}, func(rel string, data []byte) {
		for i, line := range strings.Split(reComment.ReplaceAllString(string(data), ""), "\n") {
			code := stripExpr(line)
			bad := ""
			switch {
			case nonASCIILetter(code):
				bad = "a letter outside ASCII"
			case reLiteralUI.MatchString(line):
				bad = "a sentence in a call"
			}
			for _, m := range reTextNode.FindAllStringSubmatch(code, -1) {
				if reWord.MatchString(m[1]) && !reExample.MatchString(strings.TrimSpace(m[1])) {
					bad = "a text between tags: " + strings.TrimSpace(m[1])
				}
			}
			for _, m := range reAttrText.FindAllStringSubmatch(code, -1) {
				if reWord.MatchString(m[1]) && !reExample.MatchString(m[1]) {
					bad = "a text in an attribute: " + m[1]
				}
			}
			if bad != "" {
				t.Errorf("%s:%d writes %s: put it in web/locales/fr.json", rel, i+1, bad)
			}
		}
	})
	// day, extract, vendor and norm read French and English invoices: their words are data, not texts.
	skip := regexp.MustCompile(`^(internal/(day|extract|vendor|norm)|tools)/`)
	walkFiles(t, "../..", []string{".go"}, func(rel string, data []byte) {
		if skip.MatchString(rel) {
			return
		}
		f, err := parser.ParseFile(token.NewFileSet(), rel, data, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if lit, ok := n.(*ast.BasicLit); ok && lit.Kind == token.STRING && nonASCIILetter(lit.Value) {
				t.Errorf("%s writes %s: put it in web/locales/fr.json", rel, lit.Value)
			}
			return true
		})
	})
}

func TestPlural(t *testing.T) {
	cases := []struct {
		code string
		n    int
		want string
	}{
		{"fr", 0, "one"}, {"fr", 1, "one"}, {"fr", 2, "other"}, {"fr", 1000000, "many"},
		{"en", 0, "other"}, {"en", 1, "one"}, {"es", 1, "one"}, {"ja", 1, "other"}, {"zh", 5, "other"},
		{"ar", 0, "zero"}, {"ar", 1, "one"}, {"ar", 2, "two"}, {"ar", 5, "few"}, {"ar", 11, "many"}, {"ar", 100, "other"},
	}
	for _, c := range cases {
		if got := Plural(c.code, c.n); got != c.want {
			t.Errorf("Plural(%s, %d) = %s, want %s", c.code, c.n, got, c.want)
		}
	}
}

func TestFrench(t *testing.T) {
	fr := catalog(t).Lang("fr")
	if got := fr.Money(1234.5, "EUR"); got != "1\u00a0234,50\u00a0€" {
		t.Errorf("Money = %q", got)
	}
	if got := fr.Money(1200, "JPY"); got != "1\u00a0200\u00a0¥" {
		t.Errorf("Money JPY = %q", got)
	}
	if got := fr.ShortDate(time.Date(2026, 11, 18, 0, 0, 0, 0, time.Local)); got != "18 nov." {
		t.Errorf("ShortDate = %q", got)
	}
	if got := fr.N("reminder.missing", 1); got != "1 facture à récupérer" {
		t.Errorf("N one = %q", got)
	}
	if got := fr.N("reminder.missing", 3); got != "3 factures à récupérer" {
		t.Errorf("N other = %q", got)
	}
	if got := catalog(t).Lang("xx").Code(); got != "fr" {
		t.Errorf("an unknown language must fall back to French, got %s", got)
	}
}

func TestMatch(t *testing.T) {
	for header, want := range map[string]string{
		"fr-FR,fr;q=0.9,en;q=0.8": "fr", "de-DE,ja;q=0.5": "ja", "zh-Hans-CN": "zh", "de": "", "": "",
	} {
		if got := Match(header); got != want {
			t.Errorf("Match(%q) = %q, want %q", header, got, want)
		}
	}
}
