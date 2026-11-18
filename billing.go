package main

import (
	"net/url"
	"sort"
	"strings"
)

// billingPages is where each service keeps its invoices, by domain (or by name
// when it has none). Checked in their help pages in November 2026; a link
// typed on a payment replaces it. Some lead to the client area's home, where
// the invoices are one click away.
var billingPages = map[string]string{
	"anthropic.com":   "https://claude.ai/settings/billing",
	"cloudflare.com":  "https://dash.cloudflare.com/?to=/:account/billing",
	"render.com":      "https://dashboard.render.com/billing",
	"neon.tech":       "https://console.neon.tech/app/billing",
	"ovhcloud.com":    "https://www.ovh.com/manager/#/dedicated/billing/history",
	"x.ai":            "https://console.x.ai/team/default/billing/invoices",
	"proton.me":       "https://account.proton.me/",
	"openai.com":      "https://chatgpt.com/#settings/Account",
	"free.fr":         "https://mobile.free.fr/account/",
	"apple.com":       "https://www.apple.com/fr/shop/order/list",
	"inpi.fr":         "https://procedures.inpi.fr/",
	"stripe.com":      "https://dashboard.stripe.com/settings/documents",
	"sedomicilier.fr": "https://www.sedomicilier.fr/",
}

// billingPage is the page of a vendor's invoices, or "".
func billingPage(domain, vendor string) string {
	if u := billingPages[strings.ToLower(domain)]; u != "" {
		return u
	}
	return billingPages[norm(vendor)]
}

// cleanURL keeps an http(s) address, or nothing.
func cleanURL(s string) string {
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

// BillingLink is one place to fetch invoices from.
type BillingLink struct {
	Name   string `json:"name"`
	Domain string `json:"domain,omitempty"`
	URL    string `json:"url"`
}

// billingLinks lists, once per address, where to fetch the invoices of the
// running payments and of the vendors found in the folder.
func (m *model) billingLinks() []BillingLink {
	type entry struct {
		BillingLink
		names  map[string]bool
		vendor string // the vendor named by the first service of this page
	}
	byURL := map[string]*entry{}
	var order []string
	add := func(name, vendor, domain, u string) {
		if u == "" {
			return
		}
		e := byURL[u]
		if e == nil {
			e = &entry{BillingLink: BillingLink{Name: name, Domain: domain, URL: u}, names: map[string]bool{}, vendor: vendor}
			byURL[u] = e
			order = append(order, u)
		}
		e.names[name] = true
		if len(e.names) > 1 && e.vendor != "" {
			e.Name = e.vendor // several services share the page: their vendor's name
		}
	}
	today := m.today.Format(dayFmt)
	for _, p := range m.db.Payments {
		if p.End != "" && p.End < today {
			continue
		}
		add(p.Name, p.invoiceName(), p.Domain, p.billingURL())
	}
	for _, inv := range m.invoices {
		add(inv.Vendor, inv.Vendor, inv.Domain, billingPage(inv.Domain, inv.Vendor))
	}
	out := make([]BillingLink, 0, len(order))
	for _, u := range order {
		out = append(out, byURL[u].BillingLink)
	}
	sort.SliceStable(out, func(i, j int) bool { return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name) })
	return out
}

// billingURL is the link typed on the payment, or the known page of its vendor.
func (p Payment) billingURL() string {
	if p.Billing != "" {
		return p.Billing
	}
	return billingPage(p.Domain, p.invoiceName())
}
