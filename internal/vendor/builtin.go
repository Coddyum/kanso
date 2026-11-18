package vendor

// builtin is the vendors every user may meet.
var builtin = []Vendor{
	{"Anthropic", "anthropic.com", []string{"anthropic"}, true},
	{"Cloudflare", "cloudflare.com", []string{"cloudflare"}, true},
	{"Neon", "neon.tech", []string{"neon"}, true},
	{"OVHcloud", "ovhcloud.com", []string{"ovh", "ovhcloud"}, true},
	{"Render", "render.com", []string{"render"}, true},
	{"Stripe", "stripe.com", []string{"stripe"}, true},
	{"SeDomicilier", "sedomicilier.fr", []string{"sedomicilier", "hellodom"}, true},
	{"Bouygues Telecom", "bouyguestelecom.fr", []string{"bouygues", "bouyguestelecom"}, true},
	{"Free Mobile", "free.fr", []string{"free mobile"}, true},
	{"Free", "free.fr", []string{"free"}, false}, // a folder named « Free »; the word is too common in invoices to be read in their text
	{"Apple", "apple.com", []string{"apple"}, true},
	{"Urssaf", "urssaf.fr", []string{"urssaf", "ursaff"}, true},
	{"Insee", "insee.fr", []string{"insee"}, true},
	{"INPI", "inpi.fr", []string{"inpi"}, true},
	{"Indy", "indy.fr", []string{"indy"}, true},
	{"Impôts", "impots.gouv.fr", []string{"impot", "impots", "dgfip"}, true},
	{"Revolut", "revolut.com", []string{"revolut"}, true},
	{"GitHub", "github.com", []string{"github"}, true},
	{"Google", "google.com", []string{"google"}, true},
	{"OpenAI", "openai.com", []string{"openai"}, true},
	{"Vercel", "vercel.com", []string{"vercel"}, true},
	{"Resend", "resend.com", []string{"resend"}, true},
	{"Expo", "expo.dev", []string{"expo"}, true},
	{"Figma", "figma.com", []string{"figma"}, true},
	{"Notion", "notion.so", []string{"notion"}, true},
	{"Proton", "proton.me", []string{"proton"}, false}, // many customers have their own e-mail address on proton.me
}

// builtinBilling is where each service keeps its invoices, by domain. Checked
// in their help pages in November 2026; a link typed on a payment replaces it.
// Some lead to the client area's home, where the invoices are one click away.
var builtinBilling = map[string]string{
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
