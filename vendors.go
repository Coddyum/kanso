package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// localVendors are the vendors of one user only (their own companies, a local
// shop…). They live in the data folder, beside db.json, so they never enter
// the code:
//
//	{
//	  "vendors": [{"name": "Mon asso", "domain": "monasso.fr", "aliases": ["mon asso"]}],
//	  "billing": {"monasso.fr": "https://monasso.fr/espace/factures"}
//	}
type localVendors struct {
	Vendors []struct {
		Name    string   `json:"name"`
		Domain  string   `json:"domain,omitempty"`
		Aliases []string `json:"aliases,omitempty"` // the name, lowercased, when empty
		InText  *bool    `json:"inText,omitempty"`  // also recognised in the text; true when left out
	} `json:"vendors"`
	Billing map[string]string `json:"billing"` // domain, or name when it has none → page of its invoices
}

// loadLocalVendors adds the vendors of path to the known ones. Called once at
// startup, before anything is read; a missing file adds nothing.
func loadLocalVendors(path string) error {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("vendors: read %s: %w", path, err)
	}
	var local localVendors
	if err := json.Unmarshal(data, &local); err != nil {
		return fmt.Errorf("vendors: parse %s: %w", path, err)
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
		knownVendors = append(knownVendors, vendor{name, cleanDomain(v.Domain), aliases, v.InText == nil || *v.InText})
	}
	for key, page := range local.Billing {
		if u := cleanURL(page); u != "" {
			billingPages[strings.ToLower(strings.TrimSpace(key))] = u
		}
	}
	return nil
}
