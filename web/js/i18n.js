// The page's words. Each language is a JSON file of web/locales holding the
// keys of fr.json, the reference; t() reads them. A key missing in a language
// falls back to French. Numbers, amounts and dates go through Intl in the same
// language. The server writes its own texts (deadlines, reminders, refusals)
// from the same files, in the language the page sends with every call.

const RTL = new Set(["ar"]);

let dict = {};
let ref = {};
let rules = new Intl.PluralRules("fr");

// lang is the page's language, set once by loadLocale.
export let lang = "fr";
let loaded = false;

// chosenLang is the language the page sends the server; "" until it is known.
export const chosenLang = () => (loaded ? lang : "");

const lookup = (d, key) => key.split(".").reduce((o, k) => (o == null ? undefined : o[k]), d);

async function read(code) {
    const r = await fetch(`locales/${code}.json`);
    if (!r.ok) throw new Error(`locales/${code}.json: ${r.status}`);
    return r.json();
}

// loadLocale reads French and the page's language, then turns the page to its direction.
export async function loadLocale(code) {
    ref = await read("fr");
    dict = code === "fr" ? ref : await read(code).catch(() => ref);
    lang = dict === ref ? "fr" : code;
    rules = new Intl.PluralRules(lang);
    document.documentElement.lang = lang;
    document.documentElement.dir = RTL.has(lang) ? "rtl" : "ltr";
    loaded = true;
}

// t is the text of key with each {{name}} replaced by vars.name. A vars.count
// picks the plural form: key_one, key_few… as the language has them, key_other otherwise.
export function t(key, vars = {}) {
    let k = key;
    if (vars.count != null) {
        const exact = `${key}_${rules.select(vars.count)}`;
        k = lookup(dict, exact) != null || lookup(ref, exact) != null ? exact : `${key}_other`;
    }
    const s = lookup(dict, k) ?? lookup(ref, k);
    if (typeof s !== "string") return s ?? key;
    return s.replace(/\{\{(\w+)\}\}/g, (_, v) => String(vars[v] ?? ""));
}

// translatePage fills the fixed parts of index.html: data-i18n (text), data-i18n-title, data-i18n-label (aria-label).
export function translatePage(root = document) {
    root.querySelectorAll("[data-i18n]").forEach((el) => (el.textContent = t(el.dataset.i18n)));
    root.querySelectorAll("[data-i18n-title]").forEach((el) => (el.title = t(el.dataset.i18nTitle)));
    root.querySelectorAll("[data-i18n-label]").forEach((el) => el.setAttribute("aria-label", t(el.dataset.i18nLabel)));
}
