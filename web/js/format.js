// Formats: days, months, amounts, sizes. Pure functions, no state.

export const pad = (n) => String(n).padStart(2, "0");
export const iso = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
export const parseISO = (s) => {
    const [y, m, d] = s.split("-").map(Number);
    return new Date(y, m - 1, d);
};
export const firstOfMonth = (d) => new Date(d.getFullYear(), d.getMonth(), 1);
export const capitalize = (s) => s.replace(/^./, (c) => c.toUpperCase());

export const MONTHS = ["janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"];
export const MONTHS_SHORT = ["janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."];
// The month folders the server creates for a dropped invoice.
export const MONTH_FOLDERS = ["Janvier", "Fevrier", "Mars", "Avril", "Mai", "Juin", "Juillet", "Aout", "Septembre", "Octobre", "Novembre", "Decembre"];

export const fmtDay = (s) => {
    const d = parseISO(s);
    return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
};
export const shortDay = (s) => `${parseISO(s).getDate()} ${MONTHS_SHORT[parseISO(s).getMonth()]}`;
export const monthLabel = (ym) => `${MONTHS[Number(ym.slice(5, 7)) - 1]} ${ym.slice(0, 4)}`;
export const fmtMod = (s) => {
    const d = new Date(s);
    return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
};

// Fredoka draws the narrow space between thousands almost invisible: a plain non-breaking space instead.
const wide = (s) => s.replace(/ /g, " ");
export const money = (n, cur) =>
    n == null ? "—" : wide(new Intl.NumberFormat("fr-FR", { style: "currency", currency: cur || "EUR" }).format(n));
export const euros = (n) => `${wide(Math.round(n).toLocaleString("fr-FR"))} €`;
export const eurosExact = (n) => money(Math.round(n * 100) / 100, "EUR");
// approx marks an amount that is a guess (a variable service with no invoice yet).
export const approx = (e) => (e.estimate ? "≈ " : "");

// totals adds amounts up per currency: { EUR: 12, USD: 8.2 }.
export const totals = (items) => {
    const t = {};
    for (const { amount, currency } of items) {
        if (amount == null) continue;
        const c = currency || "EUR";
        t[c] = (t[c] || 0) + amount;
    }
    return t;
};
export const fmtTotals = (t) =>
    Object.keys(t)
        .sort()
        .map((c) => money(t[c], c))
        .join(" · ") || money(0, "EUR");

export const fmtSize = (n) => {
    if (n < 1024) return `${n} o`;
    if (n < 1048576) return `${Math.round(n / 1024)} Ko`;
    return `${(n / 1048576).toFixed(1).replace(".", ",")} Mo`;
};

export const groupBy = (list, key) => {
    const out = {};
    for (const x of list) (out[key(x)] ??= []).push(x);
    return out;
};

export const encPath = (p) => p.split("/").map(encodeURIComponent).join("/");
export const fileURL = (p) => "/files/" + encPath(p);
