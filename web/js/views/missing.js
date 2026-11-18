// À récupérer: the past due dates with no invoice, grouped by subscription,
// the pages where invoices are fetched, and the free reminders.

import { esc } from "../dom.js";
import { approx, capitalize, fmtDay, groupBy, money, monthLabel, MONTH_FOLDERS, parseISO } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { nextDue, S } from "../state.js";
import { logo, pageHead } from "../ui/parts.js";

// missCard is one month whose invoice is missing: drop the PDF on the line, or pick it.
const missCard = (m) => {
    const d = parseISO(m.date);
    return `
    <div class="mcard" data-drop="missing" data-key="${esc(m.key)}" data-vendor="${esc(m.vendor)}" data-date="${m.date}"
        title="${esc(t("missing.filedIn", { path: `${d.getFullYear()} / ${m.vendor} / ${MONTH_FOLDERS[d.getMonth()]}` }))}">
        <span class="mc-month"><b>${capitalize(monthLabel(m.date.slice(0, 7)))}</b><span>${t("missing.dueOn", { date: fmtDay(m.date) })}</span></span>
        <span class="mc-amt">${approx(m)}${money(m.amount, m.currency)}</span>
        <label class="btn sm">${icon("upload", 13)}${t("missing.drop")}<input type="file" hidden data-upload="missing" accept=".pdf,image/*,.html"></label>
        <button class="btn sm ghost" data-action="dismiss-due" data-key="${esc(m.key)}">${t("missing.noInvoice")}</button>
    </div>`;
};

// missGroup: one subscription, when its last invoice came, when the next one is due, and each missing month.
function missGroup(items) {
    const p = S.st.payments.find((x) => x.id === items[0].paymentId) || {};
    const next = p.id ? nextDue(p) : null;
    const last = p.lastInvoice ? t("missing.lastInvoice", { date: `<b>${fmtDay(p.lastInvoice)}</b>` }) : t("missing.noneReceived");
    const n = items.length;
    return `
    <section class="card miss-group">
        <header>
            ${logo(items[0].name, items[0].domain)}
            <div class="pm"><b>${esc(items[0].name)}</b><span>${last}${next ? ` · ${t("missing.next", { date: fmtDay(next) })}` : ""}</span></div>
            ${p.billingUrl ? `<a class="btn sm" href="${esc(p.billingUrl)}" target="_blank" rel="noopener" title="${esc(p.billingUrl)}">${t("missing.invoices")}${icon("external", 13)}</a>` : ""}
            <span class="tag s-missing">${t("missing.toFetch", { count: n })}</span>
        </header>
        ${items.map(missCard).join("")}
    </section>`;
}

const host = (u) => {
    try {
        return new URL(u).host.replace(/^www\./, "");
    } catch {
        return u;
    }
};

// billingList: one link per place where invoices are fetched, a click away.
function billingList() {
    const links = S.st.billing;
    if (!links.length) return "";
    return `
    <div class="stitle">${t("missing.billingTitle")}<em>${t("missing.billingSub")}</em></div>
    <div class="bill-grid">${links
        .map(
            (b) => `<a class="bill" href="${esc(b.url)}" target="_blank" rel="noopener" title="${esc(b.url)}">
            ${logo(b.name, b.domain)}<span class="pm"><b>${esc(b.name)}</b><span>${esc(host(b.url))}</span></span>${icon("external", 14)}
        </a>`,
        )
        .join("")}</div>`;
}

const todoRow = (todo) => `
    <div class="todo ${todo.done ? "done" : ""}">
        <button class="tick-btn" data-action="toggle-todo" data-id="${todo.id}" title="${t("missing.done")}">${icon("check", 13)}</button>
        <span>${esc(todo.text)}</span>
        <button class="btn sm ghost icon x" data-action="delete-todo" data-id="${todo.id}" title="${t("common.delete")}">${icon("x", 14)}</button>
    </div>`;

export function viewMissing() {
    const st = S.st;
    const n = st.missing.length;
    const todos = [...st.todos].sort((a, b) => a.done - b.done);
    const intro = n ? t("missing.intro", { count: n }) : t("missing.nothing");
    const noPayments = `
        <div class="card callout">${icon("sparkle", 16)}<div><b>${t("missing.startTitle")}</b> ${t("missing.startText", { layout: `<code>${t("missing.layout")}</code>` })}</div></div>`;
    let cards = "";
    if (n) {
        const groups = Object.values(groupBy([...st.missing].sort((a, b) => a.date.localeCompare(b.date)), (m) => m.name));
        cards = `<div class="miss-groups">${groups.map(missGroup).join("")}</div>`;
    } else if (st.payments.length) cards = `<div class="card allgood"><b>${t("missing.allThere")}</b>${t("missing.allThereText")}</div>`;
    return `
    ${pageHead({ title: t("missing.title"), sub: intro })}
    ${st.payments.length ? "" : noPayments}
    ${cards}
    ${billingList()}
    <div class="stitle">${t("missing.todoTitle")}<em>${t("missing.todoSub")}</em></div>
    <div class="card todos">
        <form class="todo-form" data-form="todo">
            <input class="field" name="text" placeholder="${t("missing.todoPlaceholder")}" autocomplete="off">
            <button class="btn primary">${icon("plus", 15)}${t("common.add")}</button>
        </form>
        ${todos.map(todoRow).join("")}
    </div>`;
}
