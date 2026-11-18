// Formats: days, months, amounts, sizes, in the page's language. Pure functions, no state.

import { lang, t } from "./i18n.js";

export const pad = (n) => String(n).padStart(2, "0");
export const iso = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const parseISO = (s) => {
    const [y, m, d] = s.split("-").map(Number);
    return new Date(y, m - 1, d);
};
export const firstOfMonth = (d) => new Date(d.getFullYear(), d.getMonth(), 1);
export const capitalize = (s) => s.replace(/^./, (c) => c.toLocaleUpperCase(lang));

// Intl formatters are slow to build: one per language and options.
const formatters = new Map();
const cached = (Kind, opts) => {
    const key = `${Kind.name}|${lang}|${JSON.stringify(opts)}`;
    if (!formatters.has(key)) formatters.set(key, new Kind(lang, opts));
    return formatters.get(key);
};
const dateFmt = (opts) => cached(Intl.DateTimeFormat, opts);
const numFmt = (opts) => cached(Intl.NumberFormat, opts);

// The month folders the server creates for a dropped invoice: folder names, the same in every language.
export const MONTH_FOLDERS = ["Janvier", "Fevrier", "Mars", "Avril", "Mai", "Juin", "Juillet", "Aout", "Septembre", "Octobre", "Novembre", "Decembre"];

// monthName is the name of month m (0 to 11): « novembre »; short: « nov. ».
export const monthName = (m, short = false) => dateFmt({ month: short ? "short" : "long" }).format(new Date(2026, m, 1));
export const fmtDay = (s) => dateFmt({ day: "numeric", month: "short", year: "numeric" }).format(parseISO(s));
export const shortDay = (s) => dateFmt({ day: "numeric", month: "short" }).format(parseISO(s));
export const longDay = (s) => dateFmt({ weekday: "long", day: "numeric", month: "long" }).format(parseISO(s));
export const monthLabel = (ym) => dateFmt({ month: "long", year: "numeric" }).format(new Date(Number(ym.slice(0, 4)), Number(ym.slice(5, 7)) - 1, 1));
// monthParts splits « novembre 2026 » (« 2026年11月 ») so the year can be drawn softer.
export const monthParts = (d) => dateFmt({ month: "long", year: "numeric" }).formatToParts(d);
// weekdays are the short names of the days, Monday first.
export const weekdays = () => [...Array(7)].map((_, i) => dateFmt({ weekday: "short" }).format(new Date(2026, 10, 16 + i)));
export const fmtMod = (s) => dateFmt({ day: "numeric", month: "short", year: "numeric" }).format(new Date(s));

// Fredoka draws the narrow space between thousands almost invisible: a plain non-breaking space instead.
const wide = (s) => s.replace(/ /g, " ");
export const num = (n, digits = 0) => wide(numFmt({ maximumFractionDigits: digits }).format(n));
export const money = (n, cur) => (n == null ? "—" : wide(numFmt({ style: "currency", currency: cur || "EUR" }).format(n)));
export const euros = (n) => wide(numFmt({ style: "currency", currency: "EUR", maximumFractionDigits: 0, minimumFractionDigits: 0 }).format(Math.round(n)));
export const eurosExact = (n) => money(Math.round(n * 100) / 100, "EUR");
// approx marks an amount that is a guess (a variable service with no invoice yet).
export const approx = (e) => (e.estimate ? "≈ " : "");

// totals adds amounts up per currency: { EUR: 12, USD: 8.2 }.
export const totals = (items) => {
    const out = {};
    for (const { amount, currency } of items) {
        if (amount == null) continue;
        const c = currency || "EUR";
        out[c] = (out[c] || 0) + amount;
    }
    return out;
};
export const fmtTotals = (sums) =>
    Object.keys(sums)
        .sort()
        .map((c) => money(sums[c], c))
        .join(" · ") || money(0, "EUR");

export const fmtSize = (n) => {
    if (n < 1024) return t("size.bytes", { n: num(n) });
    if (n < 1048576) return t("size.kilo", { n: num(n / 1024) });
    return t("size.mega", { n: wide(numFmt({ minimumFractionDigits: 1, maximumFractionDigits: 1 }).format(n / 1048576)) });
};

export const groupBy = (list, key) => {
    const out = {};
    for (const x of list) (out[key(x)] ??= []).push(x);
    return out;
};

export const encPath = (p) => p.split("/").map(encodeURIComponent).join("/");
export const fileURL = (p) => "/files/" + encPath(p);
