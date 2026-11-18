// Kanso — front without framework: one state object, string templates,
// delegated events. The Go server is the only source of truth.

// ---------- helpers ----------
const $ = (sel, el = document) => el.querySelector(sel);
const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];
const ESC = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ESC[c]);

const ICONS = {
    calendar: '<rect x="3" y="4.5" width="18" height="16.5" rx="2.5"/><path d="M16 2.5v4M8 2.5v4M3 10h18"/>',
    receipt: '<path d="M6 2.5h12v19l-3-2-3 2-3-2-3 2z"/><path d="M9 7.5h6M9 11.5h6M9 15.5h3"/>',
    alert: '<circle cx="12" cy="12" r="9"/><path d="M12 7.5v5.5M12 16.5v.01"/>',
    folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.2h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    folderPlus: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.2h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 11v6M9 14h6"/>',
    file: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    chevL: '<path d="m15 18-6-6 6-6"/>',
    chevR: '<path d="m9 18 6-6-6-6"/>',
    upload: '<path d="M12 15V3.5M7 8.5l5-5 5 5"/><path d="M4.5 15v3.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>',
    finder: '<path d="M14 4h6v6M20 4l-8.5 8.5"/><path d="M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10"/>',
    external: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 15l6-6M10 9h5v5"/>',
    edit: '<path d="M4 20h4L19.5 8.5a2.1 2.1 0 0 0-3-3L5 17z"/>',
    check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
    x: '<path d="M6 6l12 12M18 6 6 18"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
    trash: '<path d="M4 7h16M9.5 7V4.5h5V7M6.5 7l1 13h9l1-13"/>',
    repeat: '<path d="M17 2.5 20 5.5l-3 3"/><path d="M4 11.5V10a4.5 4.5 0 0 1 4.5-4.5H20"/><path d="m7 21.5-3-3 3-3"/><path d="M20 12.5V14a4.5 4.5 0 0 1-4.5 4.5H4"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    wallet: '<path d="M4 7.5A2.5 2.5 0 0 1 6.5 5H18v4"/><rect x="3.5" y="7.5" width="17" height="12" rx="2.5"/><path d="M16 13.5h2"/>',
    sparkle: '<path d="M12 3.5 13.8 10.2 20.5 12 13.8 13.8 12 20.5 10.2 13.8 3.5 12 10.2 10.2z"/>',
    chart: '<path d="M4 20.5h16"/><rect x="5" y="11" width="3.6" height="6.5" rx="1"/><rect x="10.2" y="5.5" width="3.6" height="12" rx="1"/><rect x="15.4" y="13.5" width="3.6" height="4" rx="1"/>',
    layers: '<path d="M12 3.5 3 8l9 4.5L21 8z"/><path d="m3 12.5 9 4.5 9-4.5"/><path d="m3 16.5 9 4.5 9-4.5"/>',
    flag: '<path d="M5.5 21V4"/><path d="M5.5 4.5h11.5l-2.4 4.2 2.4 4.3H5.5"/>',
};
const icon = (name, size = 16, cls = "") =>
    `<svg class="i ${cls}" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[name]}</svg>`;

const pad = (n) => String(n).padStart(2, "0");
const iso = (d) => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
const parseISO = (s) => {
    const [y, m, d] = s.split("-").map(Number);
    return new Date(y, m - 1, d);
};
const firstOfMonth = (d) => new Date(d.getFullYear(), d.getMonth(), 1);
const MONTHS = ["janvier", "février", "mars", "avril", "mai", "juin", "juillet", "août", "septembre", "octobre", "novembre", "décembre"];
const MONTHS_SHORT = ["janv.", "févr.", "mars", "avr.", "mai", "juin", "juil.", "août", "sept.", "oct.", "nov.", "déc."];
const MONTH_FOLDERS = ["Janvier", "Fevrier", "Mars", "Avril", "Mai", "Juin", "Juillet", "Aout", "Septembre", "Octobre", "Novembre", "Decembre"];
const fmtDay = (s) => {
    const d = parseISO(s);
    return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
};
const monthLabel = (ym) => `${MONTHS[Number(ym.slice(5, 7)) - 1]} ${ym.slice(0, 4)}`;
const daysFromToday = (s) => Math.round((parseISO(s) - parseISO(S.st.today)) / 86400000);
const relDay = (s) => {
    const n = daysFromToday(s);
    if (n === 0) return "aujourd'hui";
    if (n === 1) return "demain";
    return n > 0 ? `dans ${n} jours` : `il y a ${-n} jours`;
};

// Fredoka draws the narrow space between thousands almost invisible: a plain non-breaking space instead.
const wide = (s) => s.replace(/\u202f/g, "\u00a0");
const money = (n, cur) =>
    n == null ? "—" : wide(new Intl.NumberFormat("fr-FR", { style: "currency", currency: cur || "EUR" }).format(n));
// moneyShort fits a calendar cell: « 17 $ », « 0,37 € » — the exact amount is in the tooltip.
const SYMBOL = { EUR: "€", USD: "$", GBP: "£" };
const moneyShort = (n, cur) => {
    if (n == null) return "";
    const digits = n >= 10 || Number.isInteger(n) ? 0 : 2;
    const num = n.toLocaleString("fr-FR", { minimumFractionDigits: digits, maximumFractionDigits: digits });
    return `${num} ${SYMBOL[cur] || cur}`;
};
const totals = (items) => {
    const t = {};
    for (const { amount, currency } of items) {
        if (amount == null) continue;
        const c = currency || "EUR";
        t[c] = (t[c] || 0) + amount;
    }
    return t;
};
const fmtTotals = (t) =>
    Object.keys(t)
        .sort()
        .map((c) => money(t[c], c))
        .join(" · ") || money(0, "EUR");
const approx = (e) => (e.estimate ? "≈ " : "");
const eurOf = (amount, cur) => (amount == null ? 0 : cur === "USD" ? amount * (S.st.forecast.usdToEur || 1) : amount);
const euros = (n) => `${wide(Math.round(n).toLocaleString("fr-FR"))}\u00a0€`;
const fmtSize = (n) => {
    if (n < 1024) return `${n} o`;
    if (n < 1048576) return `${Math.round(n / 1024)} Ko`;
    return `${(n / 1048576).toFixed(1).replace(".", ",")} Mo`;
};
const fmtMod = (s) => {
    const d = new Date(s);
    return `${d.getDate()} ${MONTHS_SHORT[d.getMonth()]} ${d.getFullYear()}`;
};
const groupBy = (list, key) => {
    const out = {};
    for (const x of list) (out[key(x)] ??= []).push(x);
    return out;
};
const encPath = (p) => p.split("/").map(encodeURIComponent).join("/");
const fileURL = (p) => "/files/" + encPath(p);

const CYCLES = {
    once: "Une seule fois",
    weekly: "Chaque semaine",
    monthly: "Chaque mois",
    quarterly: "Chaque trimestre",
    yearly: "Chaque année",
};
const CYCLE_SHORT = { once: "ponctuel", weekly: "hebdo", monthly: "mensuel", quarterly: "trimestriel", yearly: "annuel" };
const PER_MONTH = { weekly: 52 / 12, monthly: 1, quarterly: 1 / 3, yearly: 1 / 12, once: 0 };
const PROJECT_TONES = ["violet", "sky", "emerald", "amber", "rose", "teal", "orange", "indigo", "pink", "lime", "fuchsia", "blue"];
const KIND = { invoice: "Facture", receipt: "Reçu", order: "Commande", statement: "Relevé", document: "Document" };
const STATUS = {
    upcoming: "à venir",
    today: "aujourd'hui",
    matched: "facture trouvée",
    missing: "facture à récupérer",
    skipped: "sans facture",
    paid: "payé",
    invoice: "facture hors abonnement",
};

// Les couleurs des initiales d'un service : la palette Tailwind en 400, comme les dalles.
const PALETTE = ["#fb7185", "#38bdf8", "#a78bfa", "#34d399", "#fbbf24", "#818cf8", "#2dd4bf", "#f472b6", "#fb923c", "#a3e635", "#e879f9", "#60a5fa"];
const hashStr = (s) => [...s].reduce((h, c) => (h * 31 + c.codePointAt(0)) | 0, 7);
const colorOf = (name) => PALETTE[Math.abs(hashStr((name || "?").trim().toLowerCase())) % PALETTE.length];
function logo(name, domain, size = "") {
    const n = (name || "?").trim();
    const parts = n.split(/[\s\-_]+/).filter((w) => /^[\p{L}\p{N}]/u.test(w));
    const initials = (parts.length > 1 ? parts[0][0] + parts[1][0] : n[0] || "?").toUpperCase();
    const color = colorOf(n);
    const img = domain ? `<img src="/api/logo?d=${encodeURIComponent(domain)}" alt="" loading="lazy" onerror="this.remove()">` : "";
    return `<span class="logo ${size}" style="--c:${color}">${esc(initials)}${img}</span>`;
}

// ---------- api ----------
async function api(method, url, body) {
    const opts = { method, headers: { "X-Inc": "1" } };
    if (body !== undefined) {
        opts.headers["Content-Type"] = "application/json";
        opts.body = JSON.stringify(body);
    }
    const r = await fetch(url, opts);
    if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
    return r.json();
}

async function upload(files, params) {
    if (!files.length) return;
    const fd = new FormData();
    for (const f of files) fd.append("file", f, f.name);
    const r = await fetch("/api/upload?" + new URLSearchParams(params), {
        method: "POST",
        headers: { "X-Inc": "1" },
        body: fd,
    });
    if (!r.ok) throw new Error((await r.text()).trim());
    const res = await r.json();
    const n = res.saved.length;
    toast(`${n} fichier${n > 1 ? "s" : ""} rangé${n > 1 ? "s" : ""} dans ${res.dir || "Inc"}`);
    await load();
}

// ---------- state ----------
const S = {
    view: "calendar",
    st: null,
    sig: "",
    month: firstOfMonth(new Date()),
    events: [],
    dir: "",
    listing: null,
    q: "",
    year: "all",
    vendor: "all",
    asideTab: "upcoming",
    an: { period: "12m", project: "all", month: "", tab: "services", data: null },
};

function readHash() {
    const [view, ...rest] = location.hash.slice(1).split("/");
    S.view = ["calendar", "analytics", "invoices", "missing", "files"].includes(view) ? view : "calendar";
    if (S.view === "files") S.dir = rest.map(decodeURIComponent).join("/");
}

async function loadCalendar() {
    const { from, to } = gridRange(S.month);
    S.events = await api("GET", `/api/calendar?from=${iso(from)}&to=${iso(to)}`);
}

async function loadFiles() {
    try {
        S.listing = await api("GET", "/api/ls?dir=" + encodeURIComponent(S.dir));
    } catch (err) {
        if (!S.dir) throw err;
        toast(`« ${S.dir} » n'existe plus`, true);
        S.dir = "";
        history.replaceState(null, "", "#files");
        S.listing = await api("GET", "/api/ls?dir=");
    }
}

// fetchView loads what the current screen needs besides the state.
async function fetchView() {
    if (S.view === "calendar") await loadCalendar();
    else if (S.view === "analytics") S.an.data = await api("GET", "/api/analytics");
    else if (S.view === "files") await loadFiles();
}

async function load() {
    try {
        const [sg, st] = await Promise.all([api("GET", "/api/sig"), api("GET", "/api/state"), fetchView()]);
        S.sig = sg.sig;
        S.boot ??= sg.boot;
        S.st = st;
        const added = S.st.autoAdded;
        if (added.length) toast(`${added.length} abonnement${added.length > 1 ? "s" : ""} ajouté${added.length > 1 ? "s" : ""} d'après tes factures : ${added.join(", ")}`);
        render();
        // Inside the Mac app: its reminders follow what changed.
        window.webkit?.messageHandlers?.inc?.postMessage("changed");
    } catch (err) {
        toast(err.message, true);
    }
}

function render() {
    hidePop();
    $$(".nav a").forEach((a) => a.classList.toggle("on", a.dataset.view === S.view));
    $("#n-missing").textContent = S.st.missing.length || "";
    $("#sync").title = `Synchronisé avec ${S.st.root.replace(/^\/Users\/[^/]+/, "~")}`;
    const main = $("#main");
    main.classList.toggle("fit", S.view === "calendar");
    main.innerHTML = VIEWS[S.view]();
    if (S.view === "calendar") fitCalendar();
}

// ---------- calendar ----------
function gridRange(month) {
    const first = firstOfMonth(month);
    const from = new Date(first);
    from.setDate(1 - ((first.getDay() + 6) % 7));
    const last = new Date(month.getFullYear(), month.getMonth() + 1, 0);
    const to = new Date(last);
    to.setDate(last.getDate() + (6 - ((last.getDay() + 6) % 7)));
    return { from, to };
}

function nthDue(start, cycle, i) {
    if (cycle === "weekly") return new Date(start.getFullYear(), start.getMonth(), start.getDate() + 7 * i);
    const months = { monthly: 1, quarterly: 3, yearly: 12 }[cycle] * i;
    const y = start.getFullYear();
    const m = start.getMonth() + months;
    return new Date(y, m, Math.min(start.getDate(), new Date(y, m + 1, 0).getDate()));
}

function nextDue(p) {
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

const activeSubs = () =>
    S.st.payments.filter((p) => p.cycle !== "once" && (!p.end || p.end >= S.st.today));
const subsPerMonth = (subs) =>
    totals(subs.map((p) => ({ amount: p.current * PER_MONTH[p.cycle], currency: p.currentCurrency })));

// balanceOn is the reserve left at the end of a day, or null before it was written.
function balanceOn(date) {
    const f = S.st.forecast;
    if (!f.set || date < f.date) return null;
    let balance = f.balance;
    for (const p of f.points) {
        if (p.date > date) break;
        balance = p.balance;
    }
    return balance;
}

// The state of a due date, in words: its colour comes from its s-* class.
const STATUS_TAG = {
    matched: "Facture trouvée",
    missing: "À récupérer",
    upcoming: "À venir",
    today: "Aujourd'hui",
    paid: "Payé",
    skipped: "Sans facture",
    invoice: "Hors abonnement",
};
// A deadline's state: its colour class and its word.
const DL = { todo: ["s-deadline", "À faire"], done: ["s-matched", "Faite"], past: ["s-paid", "Passée"] };
const dlIcon = (size = "sm") => `<span class="logo ${size} dl">${icon("flag", size === "sm" ? 12 : 14)}</span>`;
const shortDay = (s) => `${parseISO(s).getDate()} ${MONTHS_SHORT[parseISO(s).getMonth()]}`;
const tagOf = (status) => `<span class="tag s-${status}">${STATUS_TAG[status] || status}</span>`;

// ev is one payment of a day: its logo alone, and a dot for its state.
const ev = (e) =>
    e.type === "deadline"
        ? `<button class="ev ${DL[e.status][0]}" data-action="deadlines" aria-label="${esc(`${e.name} · ${DL[e.status][1]}`)}">${dlIcon()}</button>`
        : `
    <button class="ev s-${e.status}" data-action="${e.type === "payment" ? "edit-payment" : "open-invoice"}"
        data-id="${esc(e.type === "payment" ? e.paymentId : e.invoiceId)}"
        aria-label="${esc(`${e.name} · ${approx(e)}${money(e.amount, e.currency)} · ${STATUS_TAG[e.status] || ""}`)}">${logo(e.name, e.domain, "sm")}</button>`;

function dayCell(d, byDay) {
    const k = iso(d);
    const evs = byDay[k] || [];
    const runsOut = S.st.forecast.runsOut;
    const cls = [
        "day",
        d.getMonth() !== S.month.getMonth() && "out",
        k === S.st.today && "today",
        d.getDay() % 6 === 0 && "weekend",
        runsOut && k >= runsOut && "broke",
    ]
        .filter(Boolean)
        .join(" ");
    const zero = k === runsOut ? '<span class="zero">réserve à 0</span>' : "";
    return `<div class="${cls}" data-action="new-payment" data-date="${k}">
        <span class="dhead"><span class="dnum">${d.getDate()}</span>${zero}<span class="grow"></span>${daySum(evs)}</span>
        <span class="evs">${evs.map(ev).join("")}<span class="more" data-action="day-list" data-date="${k}" hidden></span></span>
    </div>`;
}

// daySum is what the day costs, in euros: the logos say who, the total says how much.
function daySum(evs) {
    const paid = evs.filter((e) => e.amount != null && e.status !== "skipped");
    const spent = paid.reduce((t, e) => t + eurOf(e.amount, e.currency), 0);
    return spent >= 0.5 ? `<span class="dsum">${paid.some((e) => e.estimate || e.currency !== "EUR") ? "≈ " : ""}${euros(spent)}</span>` : "";
}

// fitCalendar hides the logos a day has no room for, and says how many (« +2 »):
// the calendar page never scrolls.
function fitCalendar() {
    for (const day of $$(".day")) {
        const evs = $$(".ev", day);
        const more = $(".more", day);
        let hidden = 0;
        while (day.scrollHeight > day.clientHeight + 1 && hidden < evs.length) {
            hidden += 1;
            evs[evs.length - hidden].hidden = true;
            more.textContent = `+${hidden}`;
            more.hidden = false;
        }
    }
    for (const list of $$("[data-fit]")) {
        const items = [...list.children];
        const note = document.createElement("div");
        note.className = "empty-mini";
        let hidden = 0;
        while (list.scrollHeight > list.clientHeight + 1 && hidden < items.length) {
            hidden += 1;
            items[items.length - hidden].hidden = true;
            note.textContent = `+ ${hidden} autre${hidden > 1 ? "s" : ""}`;
            if (!note.isConnected) list.append(note);
        }
    }
}

const dateBadge = (date) => {
    const d = parseISO(date);
    return `<span class="datebadge"><small>${MONTHS_SHORT[d.getMonth()].replace(".", "")}</small><b>${d.getDate()}</b></span>`;
};

const upItem = (e) => `
    <button class="pitem" data-action="${e.type === "invoice" ? "open-invoice" : "edit-payment"}" data-id="${esc(e.type === "invoice" ? e.invoiceId : e.paymentId)}">
        ${dateBadge(e.date)}${logo(e.name, e.domain)}
        <span class="pm"><b>${esc(e.name)}</b><span>${e.status === "upcoming" || e.status === "today" ? relDay(e.date) : STATUS[e.status]}</span></span>
        <span class="amt">${approx(e)}${money(e.amount, e.currency)}</span>
    </button>`;

const subItem = (p) => {
    const next = nextDue(p);
    const auto = p.auto ? '<span class="mini" title="Ajouté d\'après tes factures">auto</span>' : "";
    return `<button class="pitem" data-action="edit-payment" data-id="${p.id}">
        ${logo(p.name, p.domain)}
        <span class="pm"><b>${esc(p.name)}${auto}</b><span>${CYCLE_SHORT[p.cycle]}${p.variable ? " · variable" : ""}${next ? ` · ${fmtDay(next)}` : ""}</span></span>
        <span class="amt">${p.variable ? "≈ " : ""}${money(p.current, p.currentCurrency)}</span>
    </button>`;
};

const suggItem = (s) => `
    <div class="sugg">
        ${logo(s.name, s.domain)}
        <div class="pm"><b>${esc(s.name)}</b><span>${CYCLE_SHORT[s.cycle]} · dernière facture le ${fmtDay(s.last)}</span></div>
        <div class="acts">
            <button class="btn sm primary" data-action="accept-sugg" data-key="${esc(s.key)}">Ajouter</button>
            <button class="btn sm ghost icon" data-action="dismiss-sugg" data-key="${esc(s.key)}" title="Ignorer">${icon("x", 14)}</button>
        </div>
    </div>`;

const reserveItem = (p) => `
    <div class="pitem${p.balance < 0 ? " neg" : ""}">
        ${dateBadge(p.date)}${logo(p.name, p.domain)}
        <span class="pm"><b>${esc(p.name)}</b><span>reste ${money(p.balance, "EUR")}</span></span>
        <span class="amt">−${money(p.amount, "EUR")}</span>
    </div>`;

// comingSoon: the payments of the next 45 days and the deadlines still to do in that time, by date.
function comingSoon() {
    const soon = S.st.deadlines.filter((d) => d.status === "todo" && daysFromToday(d.date) <= 45);
    return [...S.st.upcoming, ...soon].sort((a, b) => a.date.localeCompare(b.date));
}

const dlItem = (d) => `
    <button class="pitem" data-action="deadlines">
        ${dateBadge(d.date)}${dlIcon("")}
        <span class="pm"><b>${esc(d.title)}</b><span>${relDay(d.date)}</span></span>
        <span class="tag s-deadline">à faire</span>
    </button>`;

function asideTabs() {
    const st = S.st;
    const f = st.forecast;
    const subs = [...activeSubs()].sort((a, b) => (nextDue(a) || "9").localeCompare(nextDue(b) || "9"));
    const tabs = [
        ["upcoming", "À venir", comingSoon().length],
        ["subs", "Abos", subs.length],
        ["reserve", "Réserve", ""],
    ];
    if (st.suggestions.length) tabs.push(["stopped", "Arrêtés", st.suggestions.length]);
    if (!tabs.some(([k]) => k === S.asideTab)) S.asideTab = "upcoming";
    const reserveSum = f.set
        ? `${money(f.balance, "EUR")} au ${fmtDay(f.date)} · ${f.runsOut ? `<b class="warn">à 0 le ${fmtDay(f.runsOut)}</b>` : "tient plus de 3 ans"}`
        : "";
    const body = {
        upcoming: [`45 prochains jours · ${fmtTotals(totals(st.upcoming))}`, comingSoon().map((x) => (x.kind ? dlItem(x) : upItem(x))), "Rien de prévu."],
        subs: [`${fmtTotals(subsPerMonth(subs))} par mois`, subs.map(subItem), "Aucun abonnement."],
        reserve: [reserveSum, f.points.map(reserveItem), f.set ? "Aucun paiement prévu." : ""],
        stopped: ["Plus de facture depuis 3 mois", st.suggestions.map(suggItem), ""],
    }[S.asideTab];
    const empty =
        S.asideTab === "reserve" && !f.set
            ? `<div class="reserve-empty"><p>Saisis le solde du compte pro : l'app retire chaque paiement prévu à sa date et te montre le jour où il tombe à zéro.</p><button class="btn primary" data-action="reserve">Saisir la réserve</button></div>`
            : `<div class="empty-mini">${body[2]}</div>`;
    return `<aside class="aside">
        <div class="tabs">${tabs.map(([k, label, n]) => `<button class="${k === S.asideTab ? "on" : ""}" data-action="aside" data-tab="${k}">${label}${n !== "" ? `<em>${n}</em>` : ""}</button>`).join("")}</div>
        ${body[0] ? `<p class="aside-sum">${body[0]}${S.asideTab === "reserve" ? ` <a data-action="reserve">modifier</a>` : ""}</p>` : ""}
        <div class="plist" data-fit>${body[1].length ? body[1].join("") : empty}</div>
    </aside>`;
}

// pageHead is the top of a screen: a title, one line, the tools on the right.
const pageHead = ({ title, sub = "", tools = "" }) => `
    <div class="head">
        <div class="ht"><h1>${title}</h1>${sub ? `<p>${sub}</p>` : ""}</div>
        <span class="grow"></span>
        ${tools ? `<div class="toolbar">${tools}</div>` : ""}
    </div>`;

// kpi is a key number: what it is, the number, one line under it.
const kpi = ({ label, value, sub = "", href = "", action = "", bad = false }) => {
    const attrs = href ? `href="${href}"` : action ? `data-action="${action}"` : "";
    const tag = href || action ? "a" : "div";
    return `<${tag} class="kpi" ${attrs}><span class="k-label">${label}</span><span class="k-val${bad ? " neg" : ""}">${value}</span>${sub ? `<span class="k-sub">${sub}</span>` : ""}</${tag}>`;
};

function deadlineKpi() {
    const next = S.st.deadlines.find((d) => d.status === "todo");
    if (!next) return kpi({ label: "Démarches", value: "—", sub: "rien de prévu", action: "deadlines" });
    const n = daysFromToday(next.date);
    const when = n === 0 ? "aujourd'hui" : n === 1 ? "demain" : `dans ${n} jours`;
    return kpi({ label: "Prochaine démarche", value: shortDay(next.date), sub: `${next.title.replace("Déclaration ", "")} · ${when}`, action: "deadlines", bad: n <= 7 });
}

function reserveKpi() {
    const f = S.st.forecast;
    if (!f.set) return kpi({ label: "Réserve", value: "—", sub: "clique pour la saisir", action: "reserve" });
    const now = balanceOn(S.st.today) ?? f.balance;
    const soon = (f.runsOut && daysFromToday(f.runsOut) < 120) || now < 0;
    const sub = f.runsOut ? `<span class="${soon ? "neg" : ""}">à 0 le ${fmtDay(f.runsOut)}</span>` : "tient plus de 3 ans";
    return kpi({ label: "Réserve", value: money(now, "EUR"), sub, action: "reserve", bad: now < 0 });
}

// monthKpi: the month shown, in euros — paid, still to fetch, still to come.
function monthKpi(inMonth) {
    const sum = { matched: 0, missing: 0, upcoming: 0 };
    for (const e of inMonth) {
        const v = eurOf(e.amount, e.currency);
        if (e.status === "missing") sum.missing += v;
        else if (e.status === "upcoming" || e.status === "today") sum.upcoming += v;
        else sum.matched += v;
    }
    const total = sum.matched + sum.missing + sum.upcoming;
    const parts = [
        ["matched", "payé"],
        ["missing", "à récupérer"],
        ["upcoming", "à venir"],
    ]
        .filter(([k]) => sum[k] >= 0.5)
        .map(([k, label]) => `<span class="tag s-${k}">${euros(sum[k])} ${label}</span>`)
        .join("");
    return kpi({ label: `Total ${MONTHS[S.month.getMonth()]}`, value: `≈ ${euros(total)}`, sub: parts || "rien ce mois-ci" });
}

function viewCalendar() {
    const st = S.st;
    const { from, to } = gridRange(S.month);
    const byDay = groupBy(S.events, (e) => e.date);
    const monthKey = `${S.month.getFullYear()}-${pad(S.month.getMonth() + 1)}`;
    const inMonth = S.events.filter((e) => e.date.startsWith(monthKey) && e.status !== "skipped");
    const cells = [];
    for (let d = new Date(from); d <= to; d.setDate(d.getDate() + 1)) cells.push(dayCell(new Date(d), byDay));
    const subs = activeSubs();
    const missing = st.missing.length;
    const title = `${MONTHS[S.month.getMonth()].replace(/^./, (c) => c.toUpperCase())} <span class="soft">${S.month.getFullYear()}</span>`;
    const tools = `
        <button class="btn ghost icon" data-action="month" data-delta="-1" title="Mois précédent">${icon("chevL")}</button>
        <button class="btn" data-action="month" data-delta="0">Aujourd'hui</button>
        <button class="btn ghost icon" data-action="month" data-delta="1" title="Mois suivant">${icon("chevR")}</button>
        <button class="btn primary" data-action="new-payment">${icon("plus", 14)}Paiement</button>`;
    return `
    ${pageHead({ title, tools })}
    <div class="kpis">
        ${reserveKpi()}
        ${monthKpi(inMonth)}
        ${kpi({ label: "Abonnements", value: `≈ ${euros(st.forecast.perMonth)}`, sub: `par mois · ${subs.length} actifs`, href: "#analytics" })}
        ${kpi({ label: "Factures à récupérer", value: missing, sub: missing ? "voir la liste" : "tout est là", href: "#missing" })}
        ${deadlineKpi()}
    </div>
    <div class="cal-layout">
        <div class="cal">
            <div class="cal-dow">${["lun.", "mar.", "mer.", "jeu.", "ven.", "sam.", "dim."].map((d) => `<div>${d}</div>`).join("")}</div>
            <div class="cal-grid">${cells.join("")}</div>
            <div class="legend">
                <span><i class="dot s-upcoming"></i>à venir</span><span><i class="dot s-matched"></i>facture trouvée</span>
                <span><i class="dot s-missing"></i>à récupérer</span><span><i class="dot s-invoice"></i>hors abonnement</span>
                <span><i class="dot s-deadline"></i>démarche à faire</span>
            </div>
        </div>
        ${asideTabs()}
    </div>`;
}

// ---------- day popover ----------
const pop = document.createElement("div");
pop.className = "pop";
pop.hidden = true;
document.body.append(pop);
let popDay = null;

function popItem(e) {
    if (e.type === "deadline") {
        const [cls, label] = DL[e.status];
        return `<div class="pop-item" data-action="deadlines">${dlIcon("")}
            <div class="pm"><b>${esc(e.name)}</b><span class="wrap">${esc(e.detail)}</span></div>
            <div class="pop-r"><span class="tag ${cls}">${label}</span></div>
        </div>`;
    }
    const inv = e.invoiceId ? findInvoice(e.invoiceId) : null;
    const p = e.paymentId ? S.st.payments.find((x) => x.id === e.paymentId) : null;
    const files = inv ? inv.files.map((f) => KIND[f.kind] || "Fichier").join(" + ") : "";
    const lines = [
        [p && CYCLE_SHORT[p.cycle], inv?.label].filter(Boolean).join(" · "),
        inv ? [files, inv.number && `n° ${inv.number}`].filter(Boolean).join(" · ") : "",
        e.estimate && p ? `estimé sur la dernière facture · ${p.recent.map((a) => money(a, p.currentCurrency)).join(" · ")}` : "",
        e.status === "missing" ? "à déposer dans « À récupérer »" : "",
    ].filter(Boolean);
    const action = e.type === "payment" ? `data-action="edit-payment" data-id="${esc(e.paymentId)}"` : `data-action="open-invoice" data-id="${esc(e.invoiceId)}"`;
    return `<div class="pop-item" ${action}>${logo(e.name, e.domain)}
        <div class="pm"><b>${esc(e.name)}</b>${lines.map((l) => `<span>${esc(l)}</span>`).join("")}</div>
        <div class="pop-r"><b>${approx(e)}${money(e.amount, e.currency)}</b>${tagOf(e.status)}</div>
    </div>`;
}

function showPop(day) {
    const date = day.dataset.date;
    const evs = S.events.filter((e) => e.date === date);
    const balance = balanceOn(date);
    if (!evs.length && balance === null) return hidePop();
    popDay = day;
    const title = parseISO(date)
        .toLocaleDateString("fr-FR", { weekday: "long", day: "numeric", month: "long" })
        .replace(/^./, (c) => c.toUpperCase());
    const total = evs.some((e) => e.amount != null) ? fmtTotals(totals(evs.filter((e) => e.amount != null && e.status !== "skipped"))) : "";
    const foot = balance === null ? "" : `<footer class="${balance < 0 ? "neg" : ""}">Réserve prévue ce soir-là<b>${money(balance, "EUR")}</b></footer>`;
    pop.innerHTML = `<header><b>${title}</b><span>${total}</span></header>${evs.map(popItem).join("")}${foot}`;
    pop.hidden = false;
    const r = day.getBoundingClientRect();
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;
    const left = Math.min(Math.max(8, r.left + r.width / 2 - w / 2), innerWidth - w - 8);
    const above = r.top - h - 8;
    pop.classList.toggle("below", above < 8);
    pop.style.left = `${left}px`;
    pop.style.top = `${above >= 8 ? above : Math.min(r.bottom + 8, innerHeight - h - 8)}px`;
    pop.style.setProperty("--arrow", `${r.left + r.width / 2 - left}px`);
}

function hidePop() {
    clearTimeout(popTimer);
    inPop = false;
    popDay = null;
    pop.hidden = true;
}

// The bubble stays open while the mouse is on it and closes the moment it
// leaves. The only wait: 90 ms when the mouse slips from its day onto a
// neighbouring one, time enough to reach the bubble across the corner.
let popTimer;
let inPop = false;
document.addEventListener("mouseover", (e) => {
    clearTimeout(popTimer);
    if (e.target.closest?.(".pop")) {
        inPop = true;
        return;
    }
    const fromPop = inPop;
    inPop = false;
    const day = e.target.closest?.(".day");
    if (day === popDay) return;
    if (!day) return hidePop();
    if (!popDay || fromPop) return showPop(day);
    popTimer = setTimeout(() => showPop(day), 90);
});
document.documentElement.addEventListener("mouseleave", hidePop);
document.addEventListener("mousedown", (e) => {
    if (!e.target.closest?.(".pop")) hidePop();
});

// ---------- invoices ----------
function filteredInvoices() {
    const q = S.q.trim().toLowerCase();
    return S.st.invoices.filter(
        (i) =>
            (S.year === "all" || i.date.startsWith(S.year)) &&
            (S.vendor === "all" || i.vendor === S.vendor) &&
            (!q ||
                [i.vendor, i.label, i.number, i.folder, ...i.files.map((f) => f.name)]
                    .join(" ")
                    .toLowerCase()
                    .includes(q)),
    );
}

const filePill = (f) =>
    `<a class="pill k-${f.kind}" href="${fileURL(f.path)}" target="_blank" rel="noopener" title="${esc(f.path)}">${KIND[f.kind] || "Fichier"}</a>`;

const invRow = (i) => `
    <div class="row">
        ${logo(i.vendor || "?", i.domain)}
        <div class="r-main"><b>${esc(i.vendor || "Fournisseur inconnu")}</b><span>${esc([i.label, i.number && "n° " + i.number].filter(Boolean).join(" · ") || i.folder)}</span></div>
        <div class="r-files">${i.files.map(filePill).join("")}${i.edited ? '<span class="pill k-edited" title="Corrigé à la main">corrigé</span>' : ""}</div>
        <div class="r-date">${fmtDay(i.date)}</div>
        <div class="r-amt ${i.amount == null ? "none" : ""}">${i.amount == null ? "montant ?" : money(i.amount, i.currency)}</div>
        <div class="r-act">
            <button class="btn sm ghost icon" data-action="edit-invoice" data-id="${i.id}" title="Corriger">${icon("edit", 14)}</button>
            <button class="btn sm ghost icon" data-action="reveal" data-path="${esc(i.files[0].path)}" title="Afficher dans le Finder">${icon("finder", 14)}</button>
        </div>
    </div>`;

function invoiceList() {
    const list = filteredInvoices();
    if (!list.length) return '<div class="card files-empty">Aucune facture ne correspond.</div>';
    const months = groupBy(list, (i) => i.date.slice(0, 7));
    const sections = Object.entries(months).map(
        ([ym, items]) => `
        <section class="mgroup">
            <div class="stitle">${monthLabel(ym)}<em>${items.length} facture${items.length > 1 ? "s" : ""}</em><span class="sum">${fmtTotals(totals(items))}</span></div>
            <div class="card">${items.map(invRow).join("")}</div>
        </section>`,
    );
    return `<div class="totals"><span>Total affiché</span><b>${fmtTotals(totals(list))}</b><span>· ${list.length} facture${list.length > 1 ? "s" : ""}</span></div>${sections.join("")}`;
}

function viewInvoices() {
    const inv = S.st.invoices;
    const years = [...new Set(inv.map((i) => i.date.slice(0, 4)))].sort().reverse();
    const vendors = [...new Set(inv.map((i) => i.vendor).filter(Boolean))].sort((a, b) => a.localeCompare(b, "fr"));
    const merged = inv.reduce((n, i) => n + i.files.length - 1, 0);
    const sub = `${inv.length} dépenses lues dans le dossier${merged ? ` · ${merged} doublon${merged > 1 ? "s" : ""} regroupé${merged > 1 ? "s" : ""} (facture + reçu…)` : ""}${S.st.extractor ? "" : " · lecture des PDF désactivée"}`;
    const tools = `
        <label class="search">${icon("search", 15)}<input class="field" id="inv-q" placeholder="Rechercher…" value="${esc(S.q)}" autocomplete="off"></label>
        <select class="select" id="inv-year"><option value="all">Toutes les années</option>${years.map((y) => `<option ${S.year === y ? "selected" : ""}>${y}</option>`).join("")}</select>
        <select class="select" id="inv-vendor"><option value="all">Tous les fournisseurs</option>${vendors.map((v) => `<option ${S.vendor === v ? "selected" : ""}>${esc(v)}</option>`).join("")}</select>`;
    return `${pageHead({ title: "Factures", sub, tools })}
    <div id="inv-list">${invoiceList()}</div>`;
}

// ---------- missing ----------
// missCard is one month whose invoice is missing: drop the PDF on the line, or pick it.
const missCard = (m) => {
    const d = parseISO(m.date);
    return `
    <div class="mcard" data-drop="missing" data-key="${esc(m.key)}" data-vendor="${esc(m.vendor)}" data-date="${m.date}"
        title="Rangée dans ${d.getFullYear()} / ${esc(m.vendor)} / ${MONTH_FOLDERS[d.getMonth()]}">
        <span class="mc-month"><b>${MONTHS[d.getMonth()].replace(/^./, (c) => c.toUpperCase())} ${d.getFullYear()}</b><span>échéance du ${fmtDay(m.date)}</span></span>
        <span class="mc-amt">${approx(m)}${money(m.amount, m.currency)}</span>
        <label class="btn sm">${icon("upload", 13)}Déposer<input type="file" hidden data-upload="missing" accept=".pdf,image/*,.html"></label>
        <button class="btn sm ghost" data-action="dismiss-due" data-key="${esc(m.key)}">Pas de facture</button>
    </div>`;
};

// missGroup: one subscription, when its last invoice came, when the next one is due, and each missing month.
function missGroup(items) {
    const p = S.st.payments.find((x) => x.id === items[0].paymentId) || {};
    const next = p.id ? nextDue(p) : null;
    const last = p.lastInvoice ? `dernière facture reçue le <b>${fmtDay(p.lastInvoice)}</b>` : "aucune facture reçue";
    const n = items.length;
    return `
    <section class="card miss-group">
        <header>
            ${logo(items[0].name, items[0].domain)}
            <div class="pm"><b>${esc(items[0].name)}</b><span>${last}${next ? ` · prochaine le ${fmtDay(next)}` : ""}</span></div>
            ${p.billingUrl ? `<a class="btn sm" href="${esc(p.billingUrl)}" target="_blank" rel="noopener" title="${esc(p.billingUrl)}">Factures${icon("external", 13)}</a>` : ""}
            <span class="tag s-missing">${n} à récupérer</span>
        </header>
        ${items.map(missCard).join("")}
    </section>`;
}

// billingList: one link per place where invoices are fetched, a click away.
function billingList() {
    const links = S.st.billing;
    if (!links.length) return "";
    const host = (u) => {
        try {
            return new URL(u).host.replace(/^www\./, "");
        } catch {
            return u;
        }
    };
    return `
    <div class="stitle">Pages de facturation<em>où récupérer chaque facture, en un clic</em></div>
    <div class="bill-grid">${links
        .map(
            (b) => `<a class="bill" href="${esc(b.url)}" target="_blank" rel="noopener" title="${esc(b.url)}">
            ${logo(b.name, b.domain)}<span class="pm"><b>${esc(b.name)}</b><span>${esc(host(b.url))}</span></span>${icon("external", 14)}
        </a>`,
        )
        .join("")}</div>`;
}

const todoRow = (t) => `
    <div class="todo ${t.done ? "done" : ""}">
        <button class="tick-btn" data-action="toggle-todo" data-id="${t.id}" title="Fait">${icon("check", 13)}</button>
        <span>${esc(t.text)}</span>
        <button class="btn sm ghost icon x" data-action="delete-todo" data-id="${t.id}" title="Supprimer">${icon("x", 14)}</button>
    </div>`;

function viewMissing() {
    const st = S.st;
    const n = st.missing.length;
    const s = n > 1 ? "s" : "";
    const todos = [...st.todos].sort((a, b) => a.done - b.done);
    const intro = n
        ? `${n} facture${s} attendue${s} d'après tes abonnements, introuvable${s} dans le dossier. Glisse le PDF sur sa ligne, il est rangé tout seul.`
        : "Rien ne manque d'après tes abonnements.";
    const noPayments = `
        <div class="card callout">${icon("sparkle", 16)}<div><b>Commence par tes abonnements dans le Calendrier</b> (ou accepte ceux repérés dans ton dossier).
        Chaque échéance passée sans facture arrive ici, avec une zone où déposer le PDF : il est rangé tout seul dans <code>Année / Fournisseur / Mois</code>.</div></div>`;
    let cards = "";
    if (n) {
        const groups = Object.values(groupBy([...st.missing].sort((a, b) => a.date.localeCompare(b.date)), (m) => m.name));
        cards = `<div class="miss-groups">${groups.map(missGroup).join("")}</div>`;
    }
    else if (st.payments.length) cards = '<div class="card allgood"><b>Tout est là</b>Chaque échéance passée a sa facture.</div>';
    return `
    ${pageHead({ title: "À récupérer", sub: intro })}
    ${st.payments.length ? "" : noPayments}
    ${cards}
    ${billingList()}
    <div class="stitle">Pense-bête<em>ce qu'il faut aller chercher en dehors des abonnements</em></div>
    <div class="card todos">
        <form class="todo-form" data-form="todo">
            <input class="field" name="text" placeholder="Ex. : facture EDF d'octobre" autocomplete="off">
            <button class="btn primary">${icon("plus", 15)}Ajouter</button>
        </form>
        ${todos.map(todoRow).join("")}
    </div>`;
}

// ---------- files ----------
const rowActs = (e) => `
    <span class="r-act">
        ${!e.dir && e.kind === "document" ? `<button class="btn sm ghost icon" data-action="mark-invoice" data-hash="${esc(e.hash)}" title="C'est une facture">${icon("receipt", 14)}</button>` : ""}
        <button class="btn sm ghost icon" data-action="rename" data-path="${esc(e.path)}" data-name="${esc(e.name)}" title="Renommer">${icon("edit", 14)}</button>
        ${e.dir ? "" : `<button class="btn sm ghost icon" data-action="open-native" data-path="${esc(e.path)}" title="Ouvrir avec l'app du Mac">${icon("external", 14)}</button>`}
        <button class="btn sm ghost icon" data-action="reveal" data-path="${esc(e.path)}" title="Afficher dans le Finder">${icon("finder", 14)}</button>
    </span>`;

const fileRow = (e) =>
    e.dir
        ? `<div class="frow is-dir" data-action="cd" data-path="${esc(e.path)}" data-dropdir="${esc(e.path)}" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("folder", 18)}</span><span class="fname">${esc(e.name)}</span><span class="fkind"></span>
            <span class="fmeta">${e.items} élément${e.items > 1 ? "s" : ""}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`
        : `<div class="frow" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("file", 18)}</span>
            <span class="fname"><a href="${fileURL(e.path)}" target="_blank" rel="noopener">${esc(e.name)}</a></span>
            <span class="fkind">${e.kind && e.kind !== "document" ? `<span class="pill k-${e.kind}">${KIND[e.kind]}</span>` : ""}</span>
            <span class="fmeta">${fmtSize(e.size)}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`;

function viewFiles() {
    const L = S.listing;
    const parts = L.dir ? L.dir.split("/") : [];
    const crumbs = [
        `<a data-action="cd" data-path="" data-dropdir="">Inc</a>`,
        ...parts.map((p, i) => {
            const path = parts.slice(0, i + 1).join("/");
            return `<span class="sep">/</span><a data-action="cd" data-path="${esc(path)}" data-dropdir="${esc(path)}">${esc(p)}</a>`;
        }),
    ].join("");
    return `
    ${pageHead({
        title: "Dossier Inc",
        sub: "Le même dossier que dans le Finder : ce que tu ajoutes ici y apparaît, et inversement.",
        tools: `<button class="btn" data-action="mkdir">${icon("folderPlus", 15)}Nouveau dossier</button>
            <label class="btn">${icon("upload", 15)}Importer<input type="file" hidden multiple data-upload="dir"></label>
            <button class="btn icon" data-action="reveal" data-path="${esc(L.dir)}" title="Ouvrir dans le Finder">${icon("finder", 15)}</button>`,
    })}
    <div class="crumbs">${crumbs}</div>
    <div class="card files">${L.entries.length ? L.entries.map(fileRow).join("") : '<div class="files-empty">Dossier vide : glisse des fichiers ici.</div>'}</div>
    <div class="dropzone-hint">${icon("upload", 16)}Glisse des fichiers du Finder ici pour les ajouter, ou sur un dossier de la liste pour les y ranger.</div>`;
}

// ---------- analytics ----------
// Every amount comes from /api/analytics in euros: an invoice, a past due date
// whose invoice is missing (its estimate), or a due date to come. The page adds
// them up by month, by project and by service; nothing is stored here.
const KIND_WORD = { invoice: "facture", estimate: "facture à récupérer · montant estimé", planned: "prévu" };
const CYCLE_WORD = {
    monthly: "Abonnement mensuel",
    yearly: "Abonnement annuel",
    quarterly: "Abonnement trimestriel",
    weekly: "Abonnement hebdo",
    once: "Achat ponctuel",
    "": "Facture hors abonnement",
};
const ym = (date) => date.slice(0, 7);
const shortMonth = (key) => `${MONTHS_SHORT[Number(key.slice(5, 7)) - 1]}`;
const sumEur = (list) => list.reduce((t, s) => t + s.eur, 0);
const eurosExact = (n) => money(Math.round(n * 100) / 100, "EUR");

function monthsBetween(a, b) {
    const out = [];
    let [y, m] = a.split("-").map(Number);
    const [y2, m2] = b.split("-").map(Number);
    while (y < y2 || (y === y2 && m <= m2)) {
        out.push(`${y}-${pad(m)}`);
        if (++m > 12) (m = 1), y++;
    }
    return out;
}
const addMonthsKey = (key, n) => {
    const d = new Date(Number(key.slice(0, 4)), Number(key.slice(5, 7)) - 1 + n, 1);
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
};

// anRange: the months of the chosen period, the ones of them already lived, and the months to come shown in grey.
function anRange() {
    const now = ym(S.st.today);
    const spend = S.an.data.spend;
    const p = S.an.period;
    if (p === "12m") {
        const first = addMonthsKey(now, -11);
        return { months: monthsBetween(first, now), ahead: monthsBetween(addMonthsKey(now, 1), addMonthsKey(now, 3)), label: "sur 12 mois" };
    }
    if (p === "all") {
        const first = spend.length ? ym(spend[0].date) : now;
        return { months: monthsBetween(first < now ? first : now, now), ahead: monthsBetween(addMonthsKey(now, 1), addMonthsKey(now, 3)), label: "depuis le début" };
    }
    const all = monthsBetween(`${p}-01`, `${p}-12`);
    return { months: all.filter((k) => k <= now), ahead: all.filter((k) => k > now), label: `en ${p}` };
}

const inProject = (s) => S.an.project === "all" || (S.an.project === "none" ? !s.project : s.project === S.an.project);
const projectById = (id) => S.an.data?.projects.find((p) => p.id === id) || S.st.projects.find((p) => p.id === id);
const toneOf = (id) => (id ? `c-${projectById(id)?.color || "none"}` : "c-none");
const projectName = (id) => (id ? projectById(id)?.name || "?" : "Non classé");

// anScope: the amounts the lists add up — the period, the project, and the month picked on the chart.
function anScope(withProject = true) {
    const { months, ahead } = anRange();
    const keep = new Set(S.an.month ? [S.an.month] : [...months, ...ahead]);
    return S.an.data.spend.filter((s) => keep.has(ym(s.date)) && (!withProject || inProject(s)));
}

function anFigures() {
    const { months, label } = anRange();
    const lived = new Set(months);
    const today = S.st.today;
    const spend = S.an.data.spend.filter(inProject);
    const past = spend.filter((s) => s.kind !== "planned" && lived.has(ym(s.date)));
    const total = sumEur(past);
    const guessed = sumEur(past.filter((s) => s.kind === "estimate"));
    const yearAhead = iso(new Date(parseISO(today).getFullYear() + 1, parseISO(today).getMonth(), parseISO(today).getDate()));
    const in90 = iso(new Date(parseISO(today).getTime() + 90 * 86400000));
    const planned = spend.filter((s) => s.kind === "planned");
    const subs = sumEur(planned.filter((s) => s.cycle && s.cycle !== "once" && s.date <= yearAhead)) / 12;
    const soon = sumEur(planned.filter((s) => s.date <= in90));
    return `<div class="kpis">
        ${kpi({ label: "Dépensé", value: euros(total), sub: `${label}${guessed >= 1 ? ` · dont ${euros(guessed)} estimés` : ""}` })}
        ${kpi({ label: "Moyenne", value: euros(months.length ? total / months.length : 0), sub: `par mois, sur ${months.length} mois` })}
        ${kpi({ label: "Abonnements", value: `≈ ${euros(subs)}`, sub: "par mois, sur l'année à venir" })}
        ${kpi({ label: "À payer", value: euros(soon), sub: "dans les 3 prochains mois" })}
    </div>`;
}

// anChart: one bar per month, from zero; what is still to come in grey (« prévu »).
function anChart() {
    const { months, ahead } = anRange();
    const keys = [...months, ...ahead];
    const now = ym(S.st.today);
    const spend = S.an.data.spend.filter(inProject);
    const byMonth = groupBy(spend, (s) => ym(s.date));
    const bars = keys.map((key) => {
        const list = byMonth[key] || [];
        return { key, spent: sumEur(list.filter((s) => s.kind !== "planned")), plan: sumEur(list.filter((s) => s.kind === "planned")), list };
    });
    const hi = Math.max(...bars.map((b) => b.spent + b.plan), 1);
    const step = 10 ** Math.floor(Math.log10(hi));
    const top = Math.ceil(hi / step) * step;
    const tone = S.an.project === "all" ? "c-accent" : toneOf(S.an.project === "none" ? "" : S.an.project);
    const col = (b) => {
        const total = b.spent + b.plan;
        const top3 = Object.entries(groupBy(b.list, (s) => s.service))
            .map(([name, l]) => [name, sumEur(l)])
            .sort((x, y) => y[1] - x[1])
            .slice(0, 4)
            .map(([name, v]) => `${name} ${euros(v)}`)
            .join(" · ");
        const title = `${monthLabel(b.key)} : ${eurosExact(b.spent)} payés${b.plan ? `, ${eurosExact(b.plan)} prévus` : ""}${top3 ? `\n${top3}` : ""}`;
        const stack = total
            ? `<span class="stack" style="height:${(total / top) * 100}%"><span class="val">${euros(total)}</span>${b.plan ? `<i class="plan" style="flex:${b.plan}"></i>` : ""}${b.spent ? `<i class="spent" style="flex:${b.spent}"></i>` : ""}</span>`
            : "";
        return `<button class="col ${S.an.month === b.key ? "sel" : ""}" data-action="an-month" data-key="${b.key}" title="${esc(title)}">${stack}</button>`;
    };
    const label = (b) => {
        const cls = [b.key === now && "now", S.an.month === b.key && "sel"].filter(Boolean).join(" ");
        const yearMark = b.key.endsWith("-01") || b.key === keys[0] ? ` ${b.key.slice(2, 4)}` : "";
        return `<span class="${cls}">${shortMonth(b.key).replace(".", "")}${yearMark}</span>`;
    };
    return `<section class="card an-chart">
        <header>
            <b>Dépenses par mois</b><span>en euros · clique un mois pour son détail</span>
            <span class="legend"><span><i class="dot ${tone}"></i>payé</span>${ahead.length || bars.some((b) => b.plan) ? '<span><i class="dot" style="--c:#3a3d43"></i>prévu</span>' : ""}</span>
        </header>
        <div class="chart ${tone}">
            <div class="axis"><span style="top:0">${euros(top)}</span><span style="top:50%">${euros(top / 2)}</span><span style="top:100%">0 €</span></div>
            <div class="plot">
                <i class="gl" style="top:0"></i><i class="gl" style="top:50%"></i><i class="gl" style="bottom:0"></i>
                <div class="cols">${bars.map(col).join("")}</div>
            </div>
            <span></span>
            <div class="months">${bars.map(label).join("")}</div>
        </div>
    </section>`;
}

// projectRows: what each project weighs in the period; a click filters the whole page on it.
function projectRows() {
    const past = anScope(false).filter((s) => s.kind !== "planned");
    const total = sumEur(past) || 1;
    const by = groupBy(past, (s) => s.project || "");
    const rows = S.an.data.projects
        .map((p) => ({ id: p.id, list: by[p.id] || [] }))
        .concat(by[""] ? [{ id: "", list: by[""] }] : [])
        .map((r) => ({ ...r, sum: sumEur(r.list), services: new Set(r.list.map((s) => s.service)).size }))
        .sort((a, b) => (a.id === "") - (b.id === "") || b.sum - a.sum);
    const row = (r) => {
        const key = r.id || "none";
        const p = projectById(r.id);
        return `<div class="lrow ${S.an.project === key ? "on" : ""}" data-action="an-project" data-id="${key}" role="button">
            <i class="dot ${toneOf(r.id)}"></i>
            <span class="lm"><b>${esc(projectName(r.id))}</b><span>${r.services} service${r.services > 1 ? "s" : ""} · ${Math.round((r.sum / total) * 100)} %</span>
                <span class="share-line ${toneOf(r.id)}"><i style="width:${(r.sum / total) * 100}%"></i></span></span>
            <span class="lv"><b>${euros(r.sum)}</b></span>
            ${p ? `<button class="btn ghost sm edit" data-action="edit-project" data-id="${p.id}" title="Modifier le projet">${icon("edit", 14)}</button>` : ""}
        </div>`;
    };
    return rows.map(row).join("");
}

// projectIdeas: the domain names of the subscriptions that no project covers yet.
function projectIdeas() {
    const names = S.an.data.projects.map((p) => p.name.toLowerCase());
    const ideas = new Set();
    for (const p of S.st.payments) {
        const m = (p.match || "").toLowerCase().match(/^([a-z0-9-]+)\.[a-z]{2,}$/);
        if (m && !names.includes(m[1])) ideas.add(m[1].replace(/^./, (c) => c.toUpperCase()));
    }
    return [...ideas];
}

function projectsCard() {
    const ideas = projectIdeas();
    const empty = !S.an.data.projects.length;
    return `<section>
        <div class="list-head"><div class="stitle">Projets</div><span class="grow"></span>
            <button class="btn sm" data-action="new-project">${icon("plus", 13)}Projet</button></div>
        <div class="card an-list">
            ${empty ? '<div class="an-empty">Crée un projet (une app, un client, la boîte) pour y classer tes services et tes factures.</div>' : ""}
            ${projectRows()}
            ${ideas.length ? `<div class="list-foot"><small>Repérés dans tes abonnements :</small>${ideas.map((n) => `<button class="btn sm" data-action="quick-project" data-name="${esc(n)}">${icon("plus", 12)}${esc(n)}</button>`).join("")}</div>` : ""}
        </div>
    </section>`;
}

// projectSelect files a service or an invoice: « Auto » follows the keywords, « Aucun projet » overrides them.
function projectSelect(kind, ref, value, auto, autoWord = "Auto") {
    const projects = S.an.data.projects;
    if (!projects.length) return "";
    const shown = value === "-" ? "" : value || auto;
    const opts = [
        `<option value="">${esc(auto ? `${autoWord} : ${projectName(auto)}` : "Non classé")}</option>`,
        ...projects.map((p) => `<option value="${p.id}" ${value === p.id ? "selected" : ""}>${esc(p.name)}</option>`),
        auto || value === "-" ? `<option value="-" ${value === "-" ? "selected" : ""}>Aucun projet</option>` : "",
    ];
    return `<select class="select psel ${shown ? `set ${toneOf(shown)}` : ""}" data-assign="${kind}" data-ref="${esc(ref)}" title="Projet">${opts.join("")}</select>`;
}

function serviceRows(scope) {
    const items = scope.filter((s) => s.kind !== "planned");
    const total = sumEur(items) || 1;
    const groups = Object.entries(groupBy(items, (s) => s.service))
        .map(([name, list]) => ({ name, list, sum: sumEur(list) }))
        .sort((a, b) => b.sum - a.sum);
    if (!groups.length) return '<div class="an-empty">Aucune dépense sur cette période.</div>';
    return groups
        .map((g) => {
            const first = g.list[g.list.length - 1];
            const chosen = g.list.find((s) => s.projectBy === "service");
            const value = chosen ? chosen.project || "-" : "";
            const auto = g.list.find((s) => s.projectBy === "keyword")?.project || "";
            const pays = S.st.payments.filter((p) => p.name === g.name && p.cycle !== "once" && (!p.end || p.end >= S.st.today));
            const monthly = pays.reduce((t, p) => t + eurOf(p.current, p.currentCurrency) * PER_MONTH[p.cycle], 0);
            const n = g.list.length;
            const sub = [CYCLE_WORD[first.cycle || ""], `${n} paiement${n > 1 ? "s" : ""}`, monthly >= 0.5 ? `≈ ${euros(monthly)} par mois` : ""].filter(Boolean).join(" · ");
            return `<div class="lrow">
                ${logo(g.name, first.domain)}
                <span class="lm"><b>${esc(g.name)}</b><span>${sub}</span></span>
                ${projectSelect("service", g.name, value, auto)}
                <span class="lv"><b>${euros(g.sum)}</b><small>${Math.round((g.sum / total) * 100)} %</small></span>
            </div>`;
        })
        .join("");
}

function ledgerRows(scope) {
    const list = [...scope].sort((a, b) => b.date.localeCompare(a.date));
    if (!list.length) return '<div class="an-empty">Aucune dépense sur cette période.</div>';
    const serviceProject = {};
    for (const s of S.an.data.spend) if (s.projectBy !== "invoice") serviceProject[s.service] ??= s.project;
    return list
        .map((s) => {
            const pick = s.kind === "invoice"
                ? projectSelect("invoice", s.invoiceId, s.projectBy === "invoice" ? s.project || "-" : "", serviceProject[s.service] || "")
                : s.project ? `<span class="tag ${toneOf(s.project)}">${esc(projectName(s.project))}</span>` : "";
            const original = s.currency !== "EUR" ? `<small>${money(s.amount, s.currency)}</small>` : "";
            return `<div class="lrow">
                ${dateBadge(s.date)}${logo(s.service, s.domain)}
                <span class="lm"><b>${esc(s.service)}</b><span>${KIND_WORD[s.kind]}${s.label ? ` · ${esc(s.label)}` : ""}</span></span>
                ${pick}
                <span class="lv"><b>${s.kind === "estimate" ? "≈ " : ""}${eurosExact(s.eur)}</b>${original}</span>
            </div>`;
        })
        .join("");
}

function listsCard() {
    const scope = anScope();
    const tabs = [
        ["services", "Par service"],
        ["ledger", "Toutes les dépenses"],
    ];
    const chips = [
        S.an.month ? `<button class="btn sm" data-action="an-month" data-key="${S.an.month}" title="Revenir à toute la période">${monthLabel(S.an.month)}${icon("x", 12)}</button>` : "",
        S.an.project !== "all" ? `<button class="btn sm" data-action="an-project" data-id="all" title="Tous les projets"><i class="dot ${toneOf(S.an.project === "none" ? "" : S.an.project)}"></i>${esc(projectName(S.an.project === "none" ? "" : S.an.project))}${icon("x", 12)}</button>` : "",
    ].join("");
    return `<section>
        <div class="list-head">
            <div class="seg">${tabs.map(([k, label]) => `<button class="${S.an.tab === k ? "on" : ""}" data-action="an-tab" data-tab="${k}">${label}</button>`).join("")}</div>
            <span class="grow"></span>${chips}
        </div>
        <div class="card an-list">${S.an.tab === "services" ? serviceRows(scope) : ledgerRows(scope)}</div>
    </section>`;
}

function viewAnalytics() {
    const d = S.an.data;
    const years = [...new Set(d.spend.map((s) => s.date.slice(0, 4)))].filter((y) => y <= S.st.today.slice(0, 4)).sort().reverse();
    const periods = [["12m", "12 mois"], ...years.map((y) => [y, y]), ["all", "Tout"]];
    if (!periods.some(([k]) => k === S.an.period)) S.an.period = "12m";
    const tools = `<div class="seg">${periods.map(([k, label]) => `<button class="${S.an.period === k ? "on" : ""}" data-action="an-period" data-p="${k}">${label}</button>`).join("")}</div>`;
    const notes = [
        d.usdToEur ? `Dollars convertis au taux de la Banque centrale européenne du ${fmtDay(d.rateDate)} (1 $ = ${d.usdToEur.toLocaleString("fr-FR")} €).` : "",
        d.noAmount ? `${d.noAmount} facture${d.noAmount > 1 ? "s" : ""} sans montant lisible, laissée${d.noAmount > 1 ? "s" : ""} de côté : corrige-les dans Factures.` : "",
    ].filter(Boolean);
    return `
    ${pageHead({ title: "Analyse", sub: "Ce que coûte la boîte, en euros : par mois, par projet, par service.", tools })}
    ${anFigures()}
    ${anChart()}
    <div class="an-cols">${projectsCard()}${listsCard()}</div>
    ${notes.map((n) => `<p class="an-note">${n}</p>`).join("")}`;
}

function projectModal(p = {}) {
    const isNew = !p.id;
    const used = new Set(S.an.data.projects.filter((x) => x.id !== p.id).map((x) => x.color));
    const color = p.color || PROJECT_TONES.find((c) => !used.has(c)) || PROJECT_TONES[0];
    openModal(
        `<form class="form">
        <h2>${isNew ? "Nouveau projet" : esc(p.name)}</h2>
        <p class="sub">Une app, un client, la boîte elle-même : chaque dépense peut y être classée.</p>
        <div class="grid2">
            <label class="f span2">Nom<input name="name" required value="${esc(p.name || "")}" placeholder="Site perso, Client A…" autofocus></label>
            <div class="f span2">Couleur<div class="colors">${PROJECT_TONES.map((c) => `<label><input type="radio" name="color" value="${c}" ${c === color ? "checked" : ""}><span class="swatch c-${c}"></span></label>`).join("")}</div></div>
            <label class="f span2">Mots qui classent tout seuls (facultatif)<input name="keywords" value="${esc((p.keywords || []).join(", "))}" placeholder="monsite.fr, boutique"><span class="hint">Une dépense dont le service, le mot sur ses factures ou le libellé contient le nom du projet ou un de ces mots y va toute seule. Séparés par des virgules.</span></label>
        </div>
        <footer>
            ${isNew ? "" : `<button type="button" class="btn ghost danger" data-action="delete-project" data-id="${p.id}">${icon("trash", 15)}Supprimer</button>`}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">Annuler</button>
            <button class="btn primary">${isNew ? "Créer" : "Enregistrer"}</button>
        </footer>
    </form>`,
        async (data) => {
            const keywords = String(data.keywords || "").split(",").map((k) => k.trim()).filter(Boolean);
            await api("POST", "/api/projects", { id: p.id || "", name: data.name, color: data.color, keywords });
            toast(isNew ? "Projet créé" : "Projet modifié");
            await load();
        },
    );
}

const VIEWS = { calendar: viewCalendar, analytics: viewAnalytics, invoices: viewInvoices, missing: viewMissing, files: viewFiles };

// ---------- modals ----------
function openModal(html, onSubmit) {
    const m = $("#modal");
    m.innerHTML = `<div class="sheet" role="dialog" aria-modal="true">${html}</div>`;
    m.hidden = false;
    const form = $("form", m);
    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        const btn = $("button.primary", form);
        btn.disabled = true;
        try {
            await onSubmit(Object.fromEntries(new FormData(form)));
            closeModal();
        } catch (err) {
            toast(err.message, true);
            btn.disabled = false;
        }
    });
    const first = $("[autofocus]", m);
    first?.focus();
}

function closeModal() {
    const m = $("#modal");
    m.hidden = true;
    m.innerHTML = "";
}

const currencyOptions = (cur) =>
    ["EUR", "USD", "GBP"].map((c) => `<option ${c === (cur || "EUR") ? "selected" : ""}>${c}</option>`).join("");

function paymentModal(p = {}, suggestion = null) {
    const isNew = !p.id;
    const sub = suggestion
        ? `Repéré dans ton dossier : ${suggestion.count} facture${suggestion.count > 1 ? "s" : ""}, la dernière le ${fmtDay(suggestion.last)}.`
        : p.auto
          ? "Ajouté tout seul d'après tes factures. Le supprimer l'empêche de revenir."
          : "Un abonnement ou une dépense prévue. Ses échéances passées sans facture iront dans « À récupérer ».";
    const shown = isNew ? p.amount : p.current;
    const shownCurrency = isNew ? p.currency : p.currentCurrency;
    const amountNote = p.variable
        ? `Montant variable. 3 dernières factures : ${p.recent.map((a) => money(a, p.currentCurrency)).join(" · ")}. Les prochaines échéances reprennent la dernière ; la réserve compte la plus haute.`
        : p.manual
          ? "Montant tapé à la main : il sert jusqu'à la prochaine facture, qui reprend la main."
          : "";
    openModal(
        `<form class="form">
        <h2>${logo(p.name || "?", p.domain, "lg")}${isNew ? "Nouveau paiement" : esc(p.name)}</h2>
        <p class="sub">${sub}</p>
        <div class="grid2">
            <label class="f span2">Nom<input name="name" required value="${esc(p.name || "")}" placeholder="Render, Claude Max, Apple Developer…" autofocus></label>
            <label class="f">Montant${p.variable ? " (variable)" : ""}<input name="amount" type="number" step="0.01" min="0" required value="${shown ?? ""}"><span class="hint">changé ici, il sert jusqu'à la prochaine facture</span></label>
            <label class="f">Devise<select name="currency">${currencyOptions(shownCurrency)}</select></label>
            ${amountNote ? `<p class="note span2">${amountNote}</p>` : ""}
            <label class="f">Fréquence<select name="cycle">${Object.entries(CYCLES)
                .map(([k, v]) => `<option value="${k}" ${k === (p.cycle || "monthly") ? "selected" : ""}>${v}</option>`)
                .join("")}</select></label>
            <label class="f">Première échéance<input name="start" type="date" required value="${p.start || S.st.today}"></label>
            <label class="f">Nom sur les factures<input name="vendor" value="${esc(p.vendor || "")}" placeholder="si différent du nom"><span class="hint">pour retrouver ses factures dans le dossier</span></label>
            <label class="f">Site, pour le logo<input name="domain" value="${esc(p.domain || "")}" placeholder="render.com"></label>
            <label class="f">Fin (facultatif)<input name="end" type="date" value="${p.end || ""}"></label>
            <label class="f">Mot sur ses factures (facultatif)<input name="match" value="${esc(p.match || "")}" placeholder="monsite.fr"><span class="hint">pour séparer deux abonnements du même fournisseur</span></label>
            <label class="f span2">Page des factures<input name="billing" value="${esc(p.billing || "")}" placeholder="${esc(p.billingUrl && !p.billing ? p.billingUrl : "https://…")}"><span class="hint">${p.billingUrl && !p.billing ? "vide : la page connue du fournisseur, en gris ci-dessus" : "là où se télécharge chaque facture"}</span></label>
            ${projectField(p)}
            <label class="f ${S.st.projects.length ? "" : "span2"}">Note<input name="note" value="${esc(p.note || "")}"></label>
            <label class="check span2"><input type="checkbox" name="expectInvoice" ${p.expectInvoice === false ? "" : "checked"}>J'attends une facture à chaque échéance</label>
        </div>
        <footer>
            ${isNew ? "" : `<button type="button" class="btn ghost danger" data-action="delete-payment" data-id="${p.id}">${icon("trash", 15)}Supprimer</button>`}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">Annuler</button>
            <button class="btn primary">Enregistrer</button>
        </footer>
    </form>`,
        async (data) => {
            const amount = parseFloat(String(data.amount).replace(",", ".")) || 0;
            const { project = "", ...fields } = data;
            data = fields;
            await api("POST", "/api/payments", {
                ...data,
                id: p.id || "",
                amount,
                amountChanged: amount !== shown || data.currency !== (shownCurrency || "EUR"),
                expectInvoice: "expectInvoice" in data,
                auto: !!p.auto,
                suggestKey: p.suggestKey || suggestion?.key || "",
            });
            if (S.st.projects.length && project !== chosenProject(p)) {
                await api("POST", "/api/assign", { kind: "service", ref: data.name, project });
            }
            toast(isNew ? "Paiement ajouté" : "Paiement modifié");
            await load();
        },
    );
}

// The project a payment was filed in by hand (« - » for none), or "" when it follows the keywords.
const chosenProject = (p) => (p.projectBy === "service" ? p.project || "-" : "");

function projectField(p) {
    if (!S.st.projects.length) return "";
    const value = chosenProject(p);
    const auto = p.projectBy === "keyword" ? p.project : "";
    const opts = [
        `<option value="">${esc(auto ? `Auto : ${projectName(auto)}` : "Non classé")}</option>`,
        ...S.st.projects.map((x) => `<option value="${x.id}" ${value === x.id ? "selected" : ""}>${esc(x.name)}</option>`),
        auto || value === "-" ? `<option value="-" ${value === "-" ? "selected" : ""}>Aucun projet</option>` : "",
    ];
    return `<label class="f">Projet<select name="project">${opts.join("")}</select></label>`;
}

function invoiceModal(i) {
    openModal(
        `<form class="form">
        <h2>${logo(i.vendor || "?", i.domain, "lg")}Corriger la facture</h2>
        <p class="sub">${i.files.map((f) => esc(f.path)).join("<br>")}</p>
        <div class="grid2">
            <label class="f">Fournisseur<input name="vendor" value="${esc(i.vendor || "")}" autofocus></label>
            <label class="f">Libellé<input name="label" value="${esc(i.label || "")}"></label>
            <label class="f">Date<input name="date" type="date" value="${i.date}"></label>
            <label class="f">Montant TTC<input name="amount" type="number" step="0.01" value="${i.amount ?? ""}"></label>
            <label class="f">Devise<select name="currency">${currencyOptions(i.currency)}</select></label>
            <span></span>
            <label class="check span2"><input type="checkbox" name="notInvoice">Ce n'est pas une facture pro : la retirer de la liste</label>
        </div>
        <footer>
            ${i.edited ? `<button type="button" class="btn ghost" data-action="reset-invoice" data-id="${i.id}">Revenir à la lecture auto</button>` : ""}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">Annuler</button>
            <button class="btn primary">Enregistrer</button>
        </footer>
    </form>`,
        async (data) => {
            const amount = data.amount === "" ? null : parseFloat(data.amount);
            const override = { kind: "notInvoice" in data ? "document" : "" };
            if (data.vendor !== (i.vendor || "")) override.vendor = data.vendor;
            if (data.label !== (i.label || "")) override.label = data.label;
            if (data.date !== i.date) override.date = data.date;
            if (amount !== i.amount && amount != null) override.amount = amount;
            if (data.currency !== i.currency) override.currency = data.currency;
            await api("POST", "/api/override", { id: i.id, override });
            toast("Facture corrigée");
            await load();
        },
    );
}

function reserveModal() {
    const f = S.st.forecast;
    const rate = f.usdToEur ? `Les montants en dollars sont convertis au taux de la Banque centrale européenne du ${fmtDay(f.rateDate)} : 1 $ = ${f.usdToEur.toLocaleString("fr-FR")} €.` : "Taux de change indisponible : les montants en dollars ne sont pas comptés.";
    openModal(
        `<form class="form">
        <h2>Réserve du compte pro</h2>
        <p class="sub">Le solde du compte à une date. L'app retire chaque paiement prévu à sa date et montre le jour où la réserve tombe à zéro.</p>
        <div class="grid2">
            <label class="f">Solde (€)<input name="amount" type="number" step="0.01" required value="${f.set ? f.balance : ""}" autofocus></label>
            <label class="f">À la date du<input name="date" type="date" value="${S.st.today}"></label>
        </div>
        <p class="sub" style="margin-top: 14px">${rate}</p>
        <footer>
            ${f.set ? '<button type="button" class="btn ghost danger" data-action="clear-reserve">Retirer</button>' : ""}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">Annuler</button>
            <button class="btn primary">Enregistrer</button>
        </footer>
    </form>`,
        async (data) => {
            await api("POST", "/api/reserve", { amount: parseFloat(String(data.amount).replace(",", ".")), date: data.date });
            S.asideTab = "reserve";
            await load();
        },
    );
}

// deadlinesModal lists the administrative due dates, ticks them, and says which apply.
function deadlinesModal() {
    const a = S.st.admin;
    const row = (d) => {
        const [cls, label] = DL[d.status];
        const done = d.status === "done";
        return `<div class="dl-row">
            ${dateBadge(d.date)}
            <span class="pm"><b>${esc(d.title)}${d.approx ? '<span class="mini">date prévisionnelle</span>' : ""}</b><span>${esc(d.detail)}</span></span>
            <span class="tag ${cls}">${label}</span>
            <a class="btn sm ghost icon" href="${esc(d.url)}" target="_blank" rel="noopener" title="Ouvrir le site">${icon("external", 14)}</a>
            <button type="button" class="btn sm" data-action="toggle-deadline" data-key="${esc(d.key)}" data-done="${done ? "" : "1"}">${done ? "Annuler" : "C'est fait"}</button>
        </div>`;
    };
    const urssaf = [["quarterly", "Chaque trimestre"], ["monthly", "Chaque mois"], ["off", "Pas de déclaration"]]
        .map(([k, label]) => `<option value="${k}" ${a.urssaf === k ? "selected" : ""}>${label}</option>`)
        .join("");
    openModal(
        `<form class="form wide">
        <h2>Démarches administratives</h2>
        <p class="sub">Les dates limites officielles, reportées au jour ouvré suivant quand elles tombent un week-end ou un jour férié. Coche ce qui est fait.</p>
        <div class="dl-list">${S.st.deadlines.length ? S.st.deadlines.map(row).join("") : '<div class="empty-mini">Aucune démarche suivie.</div>'}</div>
        <div class="grid2 dl-settings">
            <label class="f">Déclaration URSSAF<select name="urssaf">${urssaf}</select></label>
            <div class="f">Rappels
                <label class="check"><input type="checkbox" name="cfe" ${a.cfe ? "checked" : ""}>CFE, à payer avant le 15 décembre</label>
                <label class="check"><input type="checkbox" name="ir" ${a.ir ? "checked" : ""}>Déclaration de revenus, fin mai</label>
            </div>
        </div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">Fermer</button><button class="btn primary">Enregistrer</button></footer>
    </form>`,
        async (data) => {
            await api("POST", "/api/admin", { urssaf: data.urssaf, cfe: "cfe" in data, ir: "ir" in data });
            toast("Démarches mises à jour");
            await load();
        },
    );
}

function promptModal(title, label, value, onOk) {
    openModal(
        `<form class="form">
        <h2>${esc(title)}</h2>
        <div class="grid2"><label class="f span2">${esc(label)}<input name="value" value="${esc(value)}" required autofocus></label></div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">Annuler</button><button class="btn primary">OK</button></footer>
    </form>`,
        (data) => onOk(data.value.trim()),
    );
}

function dayModal(date) {
    const evs = S.events.filter((e) => e.date === date);
    openModal(
        `<form class="form">
        <h2>${fmtDay(date)}</h2>
        <div class="plist">${evs.map(upItem).join("")}</div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">Fermer</button></footer>
    </form>`,
        async () => {},
    );
}

let toastTimer;
function toast(msg, isError = false) {
    const t = $("#toast");
    t.textContent = msg;
    t.className = "toast" + (isError ? " err" : "");
    t.hidden = false;
    clearTimeout(toastTimer);
    toastTimer = setTimeout(() => (t.hidden = true), isError ? 5000 : 2600);
}

// ---------- actions ----------
const findInvoice = (id) => S.st.invoices.find((i) => i.id === id);
const dismiss = (key) => api("POST", "/api/dismiss", { key, on: true });

const ACTIONS = {
    month: async (el) => {
        const delta = Number(el.dataset.delta);
        S.month = delta === 0 ? firstOfMonth(new Date()) : new Date(S.month.getFullYear(), S.month.getMonth() + delta, 1);
        await loadCalendar();
        render();
    },
    "new-payment": (el) => paymentModal({ start: el.dataset.date }),
    "edit-payment": (el) => paymentModal(S.st.payments.find((p) => p.id === el.dataset.id)),
    "open-invoice": (el) => {
        const inv = findInvoice(el.dataset.id);
        if (inv) window.open(fileURL(inv.files[0].path), "_blank", "noopener");
    },
    "edit-invoice": (el) => invoiceModal(findInvoice(el.dataset.id)),
    "day-list": (el) => dayModal(el.dataset.date),
    aside: (el) => {
        S.asideTab = el.dataset.tab;
        render();
    },
    reserve: () => reserveModal(),
    reveal: (el) => api("POST", "/api/reveal", { path: el.dataset.path }),
    "open-native": (el) => api("POST", "/api/open", { path: el.dataset.path }),
    "accept-sugg": (el) => {
        const s = S.st.suggestions.find((x) => x.key === el.dataset.key);
        const { name, vendor, match, domain, amount, currency, cycle, start } = s;
        paymentModal({ name, vendor, match, domain, amount, currency, cycle, start, expectInvoice: true }, s);
    },
    "dismiss-sugg": async (el) => {
        await dismiss(el.dataset.key);
        await load();
    },
    "dismiss-due": async (el) => {
        await dismiss(el.dataset.key);
        toast("Échéance marquée « sans facture »");
        await load();
    },
    "toggle-todo": async (el) => {
        const t = S.st.todos.find((x) => x.id === el.dataset.id);
        await api("POST", "/api/todos", { ...t, done: !t.done });
        await load();
    },
    "delete-todo": async (el) => {
        await api("DELETE", "/api/todos/" + el.dataset.id);
        await load();
    },
    cd: (el) => {
        location.hash = "files" + (el.dataset.path ? "/" + encPath(el.dataset.path) : "");
    },
    mkdir: () =>
        promptModal("Nouveau dossier", `Dans ${S.dir || "Inc"}`, "", async (name) => {
            await api("POST", "/api/mkdir", { path: S.dir, name });
            await load();
        }),
    rename: (el) =>
        promptModal("Renommer", "Nouveau nom", el.dataset.name, async (name) => {
            await api("POST", "/api/rename", { path: el.dataset.path, name });
            await load();
        }),
    "mark-invoice": async (el) => {
        await api("POST", "/api/override", { id: el.dataset.hash, override: { kind: "invoice" } });
        toast("Ajouté aux factures");
        await load();
    },
    close: () => closeModal(),
    "clear-reserve": async () => {
        await api("POST", "/api/reserve", { amount: null });
        closeModal();
        await load();
    },
    "delete-payment": async (el) => {
        await api("DELETE", "/api/payments/" + el.dataset.id);
        closeModal();
        toast("Paiement supprimé");
        await load();
    },
    deadlines: () => deadlinesModal(),
    "toggle-deadline": async (el) => {
        const list = $(".dl-list");
        const scroll = list?.scrollTop || 0;
        await api("POST", "/api/deadline", { key: el.dataset.key, done: el.dataset.done === "1" });
        await load();
        deadlinesModal();
        if ($(".dl-list")) $(".dl-list").scrollTop = scroll;
    },
    "an-period": (el) => {
        S.an.period = el.dataset.p;
        S.an.month = "";
        render();
    },
    "an-month": (el) => {
        S.an.month = S.an.month === el.dataset.key ? "" : el.dataset.key;
        render();
    },
    "an-project": (el) => {
        const id = el.dataset.id;
        S.an.project = id === S.an.project ? "all" : id;
        render();
    },
    "an-tab": (el) => {
        S.an.tab = el.dataset.tab;
        render();
    },
    "new-project": () => projectModal(),
    "edit-project": (el) => projectModal(projectById(el.dataset.id)),
    "quick-project": async (el) => {
        await api("POST", "/api/projects", { name: el.dataset.name });
        toast(`Projet « ${el.dataset.name} » créé`);
        await load();
    },
    "delete-project": async (el) => {
        await api("DELETE", "/api/projects/" + el.dataset.id);
        if (S.an.project === el.dataset.id) S.an.project = "all";
        closeModal();
        toast("Projet supprimé");
        await load();
    },
    "reset-invoice": async (el) => {
        await api("POST", "/api/override", { id: el.dataset.id, override: null });
        closeModal();
        await load();
    },
};

document.addEventListener("click", async (e) => {
    const el = e.target.closest("[data-action]");
    if (!el || !ACTIONS[el.dataset.action]) return;
    if (el.closest(".pop")) hidePop();
    if (el.tagName === "A" && el.getAttribute("href")) return;
    e.preventDefault();
    try {
        await ACTIONS[el.dataset.action](el);
    } catch (err) {
        toast(err.message, true);
    }
});

document.addEventListener("input", (e) => {
    if (e.target.id !== "inv-q") return;
    S.q = e.target.value;
    $("#inv-list").innerHTML = invoiceList();
});

document.addEventListener("change", async (e) => {
    const t = e.target;
    try {
        if (t.dataset.assign) {
            await api("POST", "/api/assign", { kind: t.dataset.assign, ref: t.dataset.ref, project: t.value });
            await load();
        } else if (t.id === "inv-year" || t.id === "inv-vendor") {
            S[t.id === "inv-year" ? "year" : "vendor"] = t.value;
            $("#inv-list").innerHTML = invoiceList();
        } else if (t.dataset.upload === "missing") {
            const card = t.closest(".mcard");
            await upload([...t.files], { vendor: card.dataset.vendor, date: card.dataset.date, key: card.dataset.key });
        } else if (t.dataset.upload === "dir") {
            await upload([...t.files], { dir: S.dir });
        }
    } catch (err) {
        toast(err.message, true);
    }
});

document.addEventListener("submit", async (e) => {
    const form = e.target;
    if (form.dataset.form !== "todo") return;
    e.preventDefault();
    const text = form.elements.text.value.trim();
    if (!text) return;
    try {
        await api("POST", "/api/todos", { text });
        await load();
        $('[data-form="todo"] input')?.focus();
    } catch (err) {
        toast(err.message, true);
    }
});

$("#modal").addEventListener("mousedown", (e) => {
    if (e.target.id === "modal") closeModal();
});
document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !$("#modal").hidden) closeModal();
});

// ---------- drag & drop ----------
const INTERNAL = "text/x-inc-path";
const hasFiles = (e) => [...(e.dataTransfer?.types || [])].includes("Files");
const isInternal = (e) => [...(e.dataTransfer?.types || [])].includes(INTERNAL);

function droppedFiles(e) {
    const out = [];
    for (const item of e.dataTransfer.items || []) {
        if (item.kind !== "file") continue;
        const entry = item.webkitGetAsEntry?.();
        if (entry && !entry.isFile) continue; // a folder: skipped
        const f = item.getAsFile();
        if (f) out.push(f);
    }
    return out;
}

function clearDrag() {
    $$(".drag, .drop-target").forEach((el) => el.classList.remove("drag", "drop-target"));
    $("#main").classList.remove("dragging");
}

document.addEventListener("dragstart", (e) => {
    const row = e.target.closest?.("[data-drag]");
    if (!row) return;
    e.dataTransfer.setData(INTERNAL, row.dataset.drag);
    e.dataTransfer.effectAllowed = "move";
});

document.addEventListener("dragover", (e) => {
    e.preventDefault();
    if (!hasFiles(e) && !isInternal(e)) return;
    const card = hasFiles(e) && e.target.closest(".mcard[data-drop]");
    const dir = e.target.closest("[data-dropdir]");
    const target = card || dir;
    if (target && target.classList.contains(card ? "drag" : "drop-target")) return;
    clearDrag();
    if (card) card.classList.add("drag");
    else if (dir) dir.classList.add("drop-target");
    else if (S.view === "files" && hasFiles(e)) $("#main").classList.add("dragging");
});

document.addEventListener("dragleave", (e) => {
    if (!e.relatedTarget) clearDrag();
});
document.addEventListener("dragend", clearDrag);

document.addEventListener("drop", async (e) => {
    e.preventDefault();
    const files = droppedFiles(e);
    const internal = e.dataTransfer.getData(INTERNAL);
    const card = e.target.closest(".mcard[data-drop]");
    const dir = e.target.closest("[data-dropdir]");
    clearDrag();
    try {
        if (card && files.length) {
            await upload(files, { vendor: card.dataset.vendor, date: card.dataset.date, key: card.dataset.key });
        } else if (dir && internal) {
            if (internal === dir.dataset.dropdir) return;
            await api("POST", "/api/move", { path: internal, toDir: dir.dataset.dropdir });
            toast(`Déplacé dans ${dir.dataset.dropdir || "Inc"}`);
            await load();
        } else if (dir && files.length) {
            await upload(files, { dir: dir.dataset.dropdir });
        } else if (S.view === "files" && files.length) {
            await upload(files, { dir: S.dir });
        } else if (files.length) {
            toast("Dépose les fichiers dans « Dossier » ou sur une facture à récupérer.", true);
        }
    } catch (err) {
        toast(err.message, true);
    }
});

// ---------- sync with the folder ----------
const isTyping = () => {
    const a = document.activeElement;
    return a && a.closest("#main") && (a.tagName === "INPUT" || a.tagName === "SELECT") && a.value;
};

async function poll() {
    if (document.hidden || !S.st || !$("#modal").hidden || isTyping()) return;
    try {
        const { sig, boot } = await api("GET", "/api/sig");
        if (S.boot && boot !== S.boot) return location.reload();
        if (sig === S.sig) return;
        const dot = $("#sync-dot");
        dot.classList.remove("pulse");
        void dot.offsetWidth;
        dot.classList.add("pulse");
        await load();
    } catch {
        /* the server may be restarting: next tick */
    }
}

$$("[data-icon]").forEach((el) => (el.outerHTML = icon(el.dataset.icon, 17)));
// Switching screens keeps the state already loaded: only the screen's own data is fetched.
window.addEventListener("hashchange", async () => {
    readHash();
    if (!S.st) return load();
    try {
        await fetchView();
        render();
        window.scrollTo(0, 0);
    } catch (err) {
        toast(err.message, true);
    }
});
document.addEventListener("visibilitychange", poll);
let resizeTimer;
window.addEventListener("resize", () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(() => S.st && S.view === "calendar" && render(), 120);
});
setInterval(poll, 4000);
readHash();
load();
