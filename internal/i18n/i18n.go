// Package i18n reads the language files of web/locales, the same the page
// uses, for the texts the server writes itself: administrative deadlines,
// notifications, refusals. fr.json is the reference: a key missing in another
// language falls back to French.
package i18n

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/coddyum/kanso/internal/currency"
)

// Langs are the languages the app speaks, French first: the reference.
var Langs = []string{"fr", "en", "es", "ja", "zh", "ar"}

// Catalog holds every language, each flattened to « a.b.c » keys.
type Catalog struct {
	dicts map[string]map[string]any // a value is a string or a []string
}

// Load reads locales/<lang>.json of fsys for every language of Langs.
func Load(fsys fs.FS) (*Catalog, error) {
	c := &Catalog{dicts: map[string]map[string]any{}}
	for _, code := range Langs {
		data, err := fs.ReadFile(fsys, "locales/"+code+".json")
		if err != nil {
			return nil, fmt.Errorf("i18n: %w", err)
		}
		var tree map[string]any
		if err := json.Unmarshal(data, &tree); err != nil {
			return nil, fmt.Errorf("i18n: locales/%s.json: %w", code, err)
		}
		flat := map[string]any{}
		flatten("", tree, flat)
		c.dicts[code] = flat
	}
	return c, nil
}

// Keys lists the keys of a language, plural forms included (« a_one », « a_other »).
func (c *Catalog) Keys(code string) map[string]any { return c.dicts[code] }

func flatten(prefix string, v any, out map[string]any) {
	switch v := v.(type) {
	case map[string]any:
		for k, sub := range v {
			key := k
			if prefix != "" {
				key = prefix + "." + k
			}
			flatten(key, sub, out)
		}
	case []any:
		list := make([]string, len(v))
		for i, x := range v {
			list[i] = fmt.Sprint(x)
		}
		out[prefix] = list
	default:
		out[prefix] = fmt.Sprint(v)
	}
}

// Lang is one language of the catalog, French behind it.
type Lang struct {
	code      string
	dict, ref map[string]any
}

// Lang is the language code; an unknown code is French.
func (c *Catalog) Lang(code string) Lang {
	if _, ok := c.dicts[code]; !ok {
		code = "fr"
	}
	return Lang{code: code, dict: c.dicts[code], ref: c.dicts["fr"]}
}

// Code is the language's code: « fr », « en »…
func (l Lang) Code() string { return l.code }

func (l Lang) lookup(key string) (any, bool) {
	if v, ok := l.dict[key]; ok {
		return v, true
	}
	v, ok := l.ref[key]
	return v, ok
}

// T is the text of key with each {{name}} replaced: vars go by pairs, name then value.
// A key missing everywhere comes back as itself, so a mistake shows instead of vanishing.
func (l Lang) T(key string, vars ...any) string {
	v, ok := l.lookup(key)
	s, isText := v.(string)
	if !ok || !isText {
		return key
	}
	for i := 0; i+1 < len(vars); i += 2 {
		s = strings.ReplaceAll(s, "{{"+fmt.Sprint(vars[i])+"}}", fmt.Sprint(vars[i+1]))
	}
	return s
}

// N is T for a count: it picks key_one, key_few… as the language has them,
// key_other otherwise, and fills {{count}}.
func (l Lang) N(key string, count int, vars ...any) string {
	k := key + "_" + Plural(l.code, count)
	if _, ok := l.lookup(k); !ok {
		k = key + "_other"
	}
	return l.T(k, append([]any{"count", count}, vars...)...)
}

// List is a list of the file: the names of the months.
func (l Lang) List(key string) []string {
	if v, ok := l.dict[key].([]string); ok {
		return v
	}
	v, _ := l.ref[key].([]string)
	return v
}

// Plural is the plural category of n in a language, as Intl.PluralRules gives it on the page.
func Plural(code string, n int) string {
	switch code {
	case "fr", "es":
		if n == 0 && code == "fr" || n == 1 {
			return "one"
		}
		if n != 0 && n%1000000 == 0 {
			return "many"
		}
	case "en":
		if n == 1 {
			return "one"
		}
	case "ar":
		switch m := n % 100; {
		case n == 0:
			return "zero"
		case n == 1:
			return "one"
		case n == 2:
			return "two"
		case m >= 3 && m <= 10:
			return "few"
		case m >= 11 && m <= 99:
			return "many"
		}
	}
	return "other"
}

// Money writes an amount the language's way: « 1 234,50 € », « €1,234.50 ».
func (l Lang) Money(v float64, cur string) string {
	digits := currency.Decimals(cur)
	s := strconv.FormatFloat(math.Abs(v), 'f', digits, 64)
	whole, dec, _ := strings.Cut(s, ".")
	group := l.T("format.group")
	for i := len(whole) - 3; i > 0; i -= 3 {
		whole = whole[:i] + group + whole[i:]
	}
	n := whole
	if dec != "" {
		n += l.T("format.decimal") + dec
	}
	if v < 0 {
		n = "-" + n
	}
	return l.T("format.money", "n", n, "sym", currency.Symbol(cur))
}

// ShortDate writes a day without its year: « 18 nov. », « Nov 18 ».
func (l Lang) ShortDate(t time.Time) string {
	month := ""
	if months := l.List("date.monthsShort"); len(months) == 12 {
		month = months[t.Month()-1]
	}
	return l.T("format.shortDate", "day", t.Day(), "month", month)
}

// Match picks the language of an Accept-Language header (« fr-FR,fr;q=0.9,en;q=0.8 »),
// in its order; "" when none is spoken by the app.
func Match(header string) string {
	for _, part := range strings.Split(header, ",") {
		tag, _, _ := strings.Cut(strings.TrimSpace(part), ";")
		base, _, _ := strings.Cut(strings.ToLower(tag), "-")
		if Known(base) {
			return base
		}
	}
	return ""
}

// Known says whether the app speaks code.
func Known(code string) bool {
	for _, l := range Langs {
		if l == code {
			return true
		}
	}
	return false
}
