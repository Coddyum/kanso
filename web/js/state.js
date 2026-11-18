// The one state object, and what is read from it. The server's state (S.st)
// is replaced whole at every load; the rest is what the screens remember.

import { firstOfMonth, iso, parseISO, totals } from "./format.js";
import { t } from "./i18n.js";
import { PER_MONTH } from "./words.js";

export const S = {
    view: "calendar",
    st: null,
    sig: "",
    boot: undefined,
    month: firstOfMonth(new Date()),
    events: [],
    dir: "",
    listing: null,
    q: "",
    year: "all",
    vendor: "all",
    asideTab: "upcoming",
    an: { period: "12m", project: "all", month: "", tab: "services", data: null },
    settings: null,
};

// rootName is the name of the synced folder: « Inc », « Factures »…
export const rootName = () => S.st.root.split("/").filter(Boolean).pop() || "/";

export const VIEW_NAMES = ["calendar", "analytics", "invoices", "missing", "files", "settings"];

// readHash picks the screen from the address: #calendar, #files/2026/Mars…
export function readHash() {
    const [view, ...rest] = location.hash.slice(1).split("/");
    S.view = VIEW_NAMES.includes(view) ? view : "calendar";
    if (S.view === "files") S.dir = rest.map(decodeURIComponent).join("/");
}

export const findInvoice = (id) => S.st.invoices.find((i) => i.id === id);
export const projectById = (id) => S.an.data?.projects.find((p) => p.id === id) || S.st.projects.find((p) => p.id === id);
export const toneOf = (id) => (id ? `c-${projectById(id)?.color || "none"}` : "c-none");
export const projectName = (id) => (id ? projectById(id)?.name || "?" : t("analytics.unfiled"));

export const daysFromToday = (s) => Math.round((parseISO(s) - parseISO(S.st.today)) / 86400000);
export const relDay = (s) => {
    const n = daysFromToday(s);
    if (n === 0) return t("day.today");
    if (n === 1) return t("day.tomorrow");
    return n > 0 ? t("day.inDays", { count: n }) : t("day.daysAgo", { count: -n });
};

// eurOf converts an amount into euros with the ECB rates the server sends; 0 when its currency has no known rate.
export const eurOf = (amount, cur) => {
    if (amount == null) return 0;
    if (!cur || cur === "EUR") return amount;
    const rate = S.st.forecast.perEUR?.[cur];
    return rate ? amount / rate : 0;
};

// balanceOn is the reserve left at the end of a day, or null before it was written.
export function balanceOn(date) {
    const f = S.st.forecast;
    if (!f.set || date < f.date) return null;
    let balance = f.balance;
    for (const p of f.points) {
        if (p.date > date) break;
        balance = p.balance;
    }
    return balance;
}

function nthDue(start, cycle, i) {
    if (cycle === "weekly") return new Date(start.getFullYear(), start.getMonth(), start.getDate() + 7 * i);
    const months = { monthly: 1, quarterly: 3, yearly: 12 }[cycle] * i;
    const y = start.getFullYear();
    const m = start.getMonth() + months;
    return new Date(y, m, Math.min(start.getDate(), new Date(y, m + 1, 0).getDate()));
}

// nextDue is a payment's next due date from today, or null when it has none left.
export function nextDue(p) {
    const up = S.st.upcoming.find((u) => u.paymentId === p.id);
    if (up) return up.date;
    const today = S.st.today;
    if (p.cycle === "once") return p.start >= today ? p.start : null;
    const start = parseISO(p.start);
    for (let i = 0; i < 3000; i++) {
        const d = iso(nthDue(start, p.cycle, i));
        if (p.end && d > p.end) return null;
        if (d >= today) return d;
    }
    return null;
}

export const activeSubs = () => S.st.payments.filter((p) => p.cycle !== "once" && (!p.end || p.end >= S.st.today));
export const subsPerMonth = (subs) => totals(subs.map((p) => ({ amount: p.current * PER_MONTH[p.cycle], currency: p.currentCurrency })));

// comingSoon: the payments of the next 45 days and the deadlines still to do in that time, by date.
export function comingSoon() {
    const soon = S.st.deadlines.filter((d) => d.status === "todo" && daysFromToday(d.date) <= 45);
    return [...S.st.upcoming, ...soon].sort((a, b) => a.date.localeCompare(b.date));
}
