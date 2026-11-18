// The calendar: the month's grid of logos, the key numbers above it, and the
// side panel (coming soon, subscriptions, reserve, stopped subscriptions).

import { $, $$, esc } from "../dom.js";
import { approx, capitalize, euros, firstOfMonth, fmtDay, fmtTotals, groupBy, iso, monthName, money, monthParts, pad, shortDay, totals, weekdays } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { activeSubs, balanceOn, comingSoon, daysFromToday, eurOf, nextDue, relDay, S, subsPerMonth } from "../state.js";
import { dateBadge, dlIcon, kpi, logo, pageHead } from "../ui/parts.js";
import { cycleShort, DL_CLASS, deadlineState, statusSentence, statusTag } from "../words.js";

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
        ? `<button class="ev ${DL_CLASS[e.status]}" data-action="deadlines" aria-label="${esc(`${e.name} · ${deadlineState(e.status)}`)}">${dlIcon()}</button>`
        : `
    <button class="ev s-${e.status}" data-action="${e.type === "payment" ? "edit-payment" : "open-invoice"}"
        data-id="${esc(e.type === "payment" ? e.paymentId : e.invoiceId)}"
        aria-label="${esc(`${e.name} · ${approx(e)}${money(e.amount, e.currency)} · ${statusTag(e.status)}`)}">${logo(e.name, e.domain, "sm")}</button>`;

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
    const zero = k === runsOut ? `<span class="zero">${t("calendar.zeroDay")}</span>` : "";
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
            note.textContent = t("calendar.more", { count: hidden });
            if (!note.isConnected) list.append(note);
        }
    }
}

// upItem is one due date in a list: an upcoming payment, or an invoice no payment claims.
export const upItem = (e) => `
    <button class="pitem" data-action="${e.type === "invoice" ? "open-invoice" : "edit-payment"}" data-id="${esc(e.type === "invoice" ? e.invoiceId : e.paymentId)}">
        ${dateBadge(e.date)}${logo(e.name, e.domain)}
        <span class="pm"><b>${esc(e.name)}</b><span>${e.status === "upcoming" || e.status === "today" ? relDay(e.date) : statusSentence(e.status)}</span></span>
        <span class="amt">${approx(e)}${money(e.amount, e.currency)}</span>
    </button>`;

const subItem = (p) => {
    const next = nextDue(p);
    const auto = p.auto ? `<span class="mini" title="${t("calendar.autoTitle")}">${t("calendar.auto")}</span>` : "";
    return `<button class="pitem" data-action="edit-payment" data-id="${p.id}">
        ${logo(p.name, p.domain)}
        <span class="pm"><b>${esc(p.name)}${auto}</b><span>${[cycleShort(p.cycle), p.variable && t("calendar.variable"), next && fmtDay(next)].filter(Boolean).join(" · ")}</span></span>
        <span class="amt">${p.variable ? "≈ " : ""}${money(p.current, p.currentCurrency)}</span>
    </button>`;
};

const suggItem = (s) => `
    <div class="sugg">
        ${logo(s.name, s.domain)}
        <div class="pm"><b>${esc(s.name)}</b><span>${cycleShort(s.cycle)} · ${t("calendar.lastInvoice", { date: fmtDay(s.last) })}</span></div>
        <div class="acts">
            <button class="btn sm primary" data-action="accept-sugg" data-key="${esc(s.key)}">${t("common.add")}</button>
            <button class="btn sm ghost icon" data-action="dismiss-sugg" data-key="${esc(s.key)}" title="${t("common.ignore")}">${icon("x", 14)}</button>
        </div>
    </div>`;

const reserveItem = (p) => `
    <div class="pitem${p.balance < 0 ? " neg" : ""}">
        ${dateBadge(p.date)}${logo(p.name, p.domain)}
        <span class="pm"><b>${esc(p.name)}</b><span>${t("calendar.left", { amount: money(p.balance, "EUR") })}</span></span>
        <span class="amt">−${money(p.amount, "EUR")}</span>
    </div>`;

const dlItem = (d) => `
    <button class="pitem" data-action="deadlines">
        ${dateBadge(d.date)}${dlIcon("")}
        <span class="pm"><b>${esc(d.title)}</b><span>${relDay(d.date)}</span></span>
        <span class="tag s-deadline">${t("calendar.todo")}</span>
    </button>`;

function asideTabs() {
    const st = S.st;
    const f = st.forecast;
    const subs = [...activeSubs()].sort((a, b) => (nextDue(a) || "9").localeCompare(nextDue(b) || "9"));
    const tabs = [
        ["upcoming", comingSoon().length],
        ["subs", subs.length],
        ["reserve", ""],
    ];
    if (st.suggestions.length) tabs.push(["stopped", st.suggestions.length]);
    if (!tabs.some(([k]) => k === S.asideTab)) S.asideTab = "upcoming";
    const reserveSum = f.set
        ? `${t("calendar.balanceOn", { amount: money(f.balance, "EUR"), date: fmtDay(f.date) })} · ${f.runsOut ? `<b class="warn">${t("calendar.runsOut", { date: fmtDay(f.runsOut) })}</b>` : t("calendar.holds")}`
        : "";
    const body = {
        upcoming: [t("calendar.upcomingSum", { total: fmtTotals(totals(st.upcoming)) }), comingSoon().map((x) => (x.kind ? dlItem(x) : upItem(x))), t("calendar.noUpcoming")],
        subs: [t("calendar.perMonth", { total: fmtTotals(subsPerMonth(subs)) }), subs.map(subItem), t("calendar.noSubs")],
        reserve: [reserveSum, f.points.map(reserveItem), f.set ? t("calendar.noPayment") : ""],
        stopped: [t("calendar.stoppedSum"), st.suggestions.map(suggItem), ""],
    }[S.asideTab];
    const empty =
        S.asideTab === "reserve" && !f.set
            ? `<div class="reserve-empty"><p>${t("calendar.reserveIntro")}</p><button class="btn primary" data-action="reserve">${t("calendar.reserveSet")}</button></div>`
            : `<div class="empty-mini">${body[2]}</div>`;
    return `<aside class="aside">
        <div class="tabs">${tabs.map(([k, n]) => `<button class="${k === S.asideTab ? "on" : ""}" data-action="aside" data-tab="${k}">${t(`calendar.tab.${k}`)}${n !== "" ? `<em>${n}</em>` : ""}</button>`).join("")}</div>
        ${body[0] ? `<p class="aside-sum">${body[0]}${S.asideTab === "reserve" ? ` <a data-action="reserve">${t("common.edit")}</a>` : ""}</p>` : ""}
        <div class="plist" data-fit>${body[1].length ? body[1].join("") : empty}</div>
    </aside>`;
}

function deadlineKpi() {
    const next = S.st.deadlines.find((d) => d.status === "todo");
    if (!next) return kpi({ label: t("calendar.kpi.deadlines"), value: "—", sub: t("calendar.kpi.nothingPlanned"), action: "deadlines" });
    const n = daysFromToday(next.date);
    return kpi({ label: t("calendar.kpi.nextDeadline"), value: shortDay(next.date), sub: `${esc(next.short)} · ${relDay(next.date)}`, action: "deadlines", bad: n <= 7 });
}

function reserveKpi() {
    const f = S.st.forecast;
    if (!f.set) return kpi({ label: t("calendar.kpi.reserve"), value: "—", sub: t("calendar.kpi.reserveSet"), action: "reserve" });
    const now = balanceOn(S.st.today) ?? f.balance;
    const soon = (f.runsOut && daysFromToday(f.runsOut) < 120) || now < 0;
    const sub = f.runsOut ? `<span class="${soon ? "neg" : ""}">${t("calendar.runsOut", { date: fmtDay(f.runsOut) })}</span>` : t("calendar.holds");
    return kpi({ label: t("calendar.kpi.reserve"), value: money(now, "EUR"), sub, action: "reserve", bad: now < 0 });
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
    const parts = ["matched", "missing", "upcoming"]
        .filter((k) => sum[k] >= 0.5)
        .map((k) => `<span class="tag s-${k}">${t(`calendar.monthPart.${k}`, { amount: euros(sum[k]) })}</span>`)
        .join("");
    return kpi({ label: t("calendar.kpi.monthTotal", { month: monthName(S.month.getMonth()) }), value: `≈ ${euros(total)}`, sub: parts || t("calendar.kpi.nothingThisMonth") });
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
    const title = monthParts(S.month)
        .map((p) => (p.type === "year" ? `<span class="soft">${p.value}</span>` : p.type === "month" ? capitalize(p.value) : p.value))
        .join("");
    const tools = `
        <button class="btn ghost icon" data-action="month" data-delta="-1" title="${t("calendar.prevMonth")}">${icon("chevL")}</button>
        <button class="btn" data-action="month" data-delta="0">${t("calendar.today")}</button>
        <button class="btn ghost icon" data-action="month" data-delta="1" title="${t("calendar.nextMonth")}">${icon("chevR")}</button>
        <button class="btn primary" data-action="new-payment">${icon("plus", 14)}${t("calendar.newPayment")}</button>`;
    return `
    ${pageHead({ title, tools })}
    <div class="kpis">
        ${reserveKpi()}
        ${monthKpi(inMonth)}
        ${kpi({ label: t("calendar.kpi.subs"), value: `≈ ${euros(st.forecast.perMonth)}`, sub: t("calendar.kpi.subsActive", { count: subs.length }), href: "#analytics" })}
        ${kpi({ label: t("calendar.kpi.missing"), value: missing, sub: missing ? t("calendar.kpi.seeList") : t("calendar.kpi.allThere"), href: "#missing" })}
        ${deadlineKpi()}
    </div>
    <div class="cal-layout">
        <div class="cal">
            <div class="cal-dow">${weekdays().map((d) => `<div>${d}</div>`).join("")}</div>
            <div class="cal-grid">${cells.join("")}</div>
            <div class="legend">
                ${["upcoming", "matched", "missing", "invoice", "deadline"].map((k) => `<span><i class="dot s-${k}"></i>${t(`calendar.legend.${k}`)}</span>`).join("")}
            </div>
        </div>
        ${asideTabs()}
    </div>`;
}
