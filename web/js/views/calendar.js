// The calendar: the month's grid of logos, the key numbers above it, and the
// side panel (coming soon, subscriptions, reserve, stopped subscriptions).

import { $, $$, esc } from "../dom.js";
import { approx, capitalize, euros, firstOfMonth, fmtDay, fmtTotals, groupBy, iso, money, MONTHS, pad, shortDay, totals } from "../format.js";
import { icon } from "../icons.js";
import { activeSubs, balanceOn, comingSoon, daysFromToday, eurOf, nextDue, relDay, S, subsPerMonth } from "../state.js";
import { dateBadge, dlIcon, kpi, logo, pageHead } from "../ui/parts.js";
import { CYCLE_SHORT, DL, STATUS, STATUS_TAG } from "../words.js";

// gridRange is the first and last day shown for a month: whole weeks, Monday first.
export function gridRange(month) {
    const first = firstOfMonth(month);
    const from = new Date(first);
    from.setDate(1 - ((first.getDay() + 6) % 7));
    const last = new Date(month.getFullYear(), month.getMonth() + 1, 0);
    const to = new Date(last);
    to.setDate(last.getDate() + (6 - ((last.getDay() + 6) % 7)));
    return { from, to };
}

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
export function fitCalendar() {
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

// upItem is one due date in a list: an upcoming payment, or an invoice no payment claims.
export const upItem = (e) => `
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

export function viewCalendar() {
    const st = S.st;
    const { from, to } = gridRange(S.month);
    const byDay = groupBy(S.events, (e) => e.date);
    const monthKey = `${S.month.getFullYear()}-${pad(S.month.getMonth() + 1)}`;
    const inMonth = S.events.filter((e) => e.date.startsWith(monthKey) && e.status !== "skipped");
    const cells = [];
    for (let d = new Date(from); d <= to; d.setDate(d.getDate() + 1)) cells.push(dayCell(new Date(d), byDay));
    const subs = activeSubs();
    const missing = st.missing.length;
    const title = `${capitalize(MONTHS[S.month.getMonth()])} <span class="soft">${S.month.getFullYear()}</span>`;
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
