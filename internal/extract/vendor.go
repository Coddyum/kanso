package extract

import (
	"strings"

	"github.com/coddyum/kanso/internal/day"
	"github.com/coddyum/kanso/internal/norm"
	"github.com/coddyum/kanso/internal/vendor"
)

// genericWords mark folders that sort documents without naming who sent them.
var genericWords = map[string]bool{
	"projects": true, "projets": true, "project": true, "projet": true, "clients": true, "client": true,
	"contract": true, "contracts": true, "contrat": true, "contrats": true, "factures": true, "facture": true,
	"invoices": true, "invoice": true, "documents": true, "document": true, "docs": true, "divers": true,
	"perso": true, "pro": true, "justificatif": true, "justificatifs": true, "classer": true, "inbox": true,
	"recus": true, "receipts": true, "declaration": true, "mandat": true, "archives": true, "archive": true,
	"abonnements": true, "depenses": true,
}

func isGenericName(s string) bool {
	for _, w := range strings.Fields(norm.Words(s)) {
		if genericWords[w] {
			return true
		}
	}
	return false
}

// detectVendor: a known vendor in the path, then the first meaningful folder
// name (the owner's own sorting), then the known vendor the text names most.
func detectVendor(vendors *vendor.Catalog, rel, text string) (string, string) {
	segs := strings.Split(rel, "/")
	for _, s := range segs {
		if v, ok := vendors.Find(s); ok {
			return v.Name, v.Domain
		}
	}
	for _, s := range segs[:len(segs)-1] {
		if day.IsTimeName(s) || isGenericName(s) {
			continue
		}
		return strings.TrimSpace(strings.NewReplacer("-", " ", "_", " ").Replace(s)), ""
	}
	if text == "" {
		return "", ""
	}
	if v, ok := vendors.InText(text); ok {
		return v.Name, v.Domain
	}
	return "", ""
}
