// Package vendor knows the companies invoices come from: their names, their
// domain (for the logo) and the page where they keep their invoices.
package vendor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"strings"

	"github.com/coddyum/kanso/internal/norm"
)

// Vendor is a company invoices come from.
type Vendor struct {
	Name    string
	Domain  string
	Aliases []string // words that name it, in a folder name or a text
	InText  bool     // may be recognised from the text, not only from the path
}

// Catalog is the list of known vendors and of their invoice pages. It is
// filled at startup (built-in list, then the user's own file) and only read after.
type Catalog struct {
	vendors []Vendor
	billing map[string]string // domain, or name key when it has none → page of its invoices
}

// NewCatalog returns the built-in vendors.
func NewCatalog() *Catalog {
	c := &Catalog{vendors: append([]Vendor(nil), builtin...), billing: map[string]string{}}
	for k, v := range builtinBilling {
		c.billing[k] = v
	}
	return c
}

// localFile is the user's own vendors (their companies, a local shop…), kept
// in the data folder beside the database so they never enter the code:
//
//	{
//	  "vendors": [{"name": "Mon asso", "domain": "monasso.fr", "aliases": ["mon asso"]}],
//	  "billing": {"monasso.fr": "https://monasso.fr/espace/factures"}
//	}
type localFile struct {
	Vendors []struct {
		Name    string   `json:"name"`
		Domain  string   `json:"domain,omitempty"`
		Aliases []string `json:"aliases,omitempty"` // the name, lowercased, when empty
		InText  *bool    `json:"inText,omitempty"`  // also recognised in the text; true when left out
	} `json:"vendors"`
	Billing map[string]string `json:"billing"`
}

// LoadLocal adds the vendors of the user's file; a missing file adds nothing.
func (c *Catalog) LoadLocal(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("vendor: read %s: %w", path, err)
	}
	var local localFile
	if err := json.Unmarshal(data, &local); err != nil {
		return fmt.Errorf("vendor: parse %s: %w", path, err)
	}
	for _, v := range local.Vendors {
		name := strings.TrimSpace(v.Name)
		if name == "" {
			continue
		}
		aliases := v.Aliases
		if len(aliases) == 0 {
			aliases = []string{strings.ToLower(name)}
		}
		c.vendors = append(c.vendors, Vendor{name, CleanDomain(v.Domain), aliases, v.InText == nil || *v.InText})
	}
	for key, page := range local.Billing {
		if u := CleanURL(page); u != "" {
			c.billing[strings.ToLower(strings.TrimSpace(key))] = u
		}
	}
	return nil
}

// Find returns the first known vendor one of whose aliases s contains.
func (c *Catalog) Find(s string) (Vendor, bool) {
	w := norm.Words(s)
	for _, v := range c.vendors {
		for _, a := range v.Aliases {
			if strings.Contains(w, norm.Words(a)) {
				return v, true
			}
		}
	}
	return Vendor{}, false
}

// Domain is the domain of a known vendor named in name, or "".
func (c *Catalog) Domain(name string) string {
	if v, ok := c.Find(name); ok {
		return v.Domain
	}
	return ""
}

// InText returns the vendor a document's text names most, among those that
// may be read from a text.
func (c *Catalog) InText(text string) (Vendor, bool) {
	w := norm.Words(text)
	best, bestN := -1, 0
	for i, v := range c.vendors {
		if !v.InText {
			continue
		}
		n := 0
		for _, a := range v.Aliases {
			n += strings.Count(w, norm.Words(a))
		}
		if n > bestN {
			best, bestN = i, n
		}
	}
	if best < 0 {
		return Vendor{}, false
	}
	return c.vendors[best], true
}

// BillingPage is the page of a vendor's invoices, by domain then by name, or "".
func (c *Catalog) BillingPage(domain, name string) string {
	if u := c.billing[strings.ToLower(domain)]; u != "" {
		return u
	}
	return c.billing[norm.Key(name)]
}

// Matches says whether two vendor names are the same company: one key holds the other.
func Matches(name, vendorName string) bool {
	a, b := norm.Key(name), norm.Key(vendorName)
	if a == "" || b == "" {
		return false
	}
	return strings.Contains(a, b) || strings.Contains(b, a)
}

// CleanDomain turns « https://www.render.com/pricing » into « render.com ».
func CleanDomain(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(strings.TrimPrefix(s, "https://"), "http://")
	s = strings.TrimPrefix(s, "www.")
	if i := strings.IndexAny(s, "/?#"); i >= 0 {
		s = s[:i]
	}
	return s
}

// CleanURL keeps an http(s) address, or nothing; « site.fr/x » gets https://.
func CleanURL(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.String()
}
