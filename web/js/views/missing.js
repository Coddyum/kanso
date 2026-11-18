// À récupérer: the past due dates with no invoice, grouped by subscription,
// the pages where invoices are fetched, and the free reminders.

import { esc } from "../dom.js";
import { approx, capitalize, fmtDay, groupBy, money, MONTH_FOLDERS, MONTHS, parseISO } from "../format.js";
import { icon } from "../icons.js";
import { nextDue, S } from "../state.js";
import { logo, pageHead } from "../ui/parts.js";

// missCard is one month whose invoice is missing: drop the PDF on the line, or pick it.
const missCard = (m) => {
    const d = parseISO(m.date);
    return `
    <div class="mcard" data-drop="missing" data-key="${esc(m.key)}" data-vendor="${esc(m.vendor)}" data-date="${m.date}"
        title="Rangée dans ${d.getFullYear()} / ${esc(m.vendor)} / ${MONTH_FOLDERS[d.getMonth()]}">
        <span class="mc-month"><b>${capitalize(MONTHS[d.getMonth()])} ${d.getFullYear()}</b><span>échéance du ${fmtDay(m.date)}</span></span>
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

export function viewMissing() {
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
    } else if (st.payments.length) cards = '<div class="card allgood"><b>Tout est là</b>Chaque échéance passée a sa facture.</div>';
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
