// Factures: every expense read in the folder, by month, with a search and two filters.

import { esc } from "../dom.js";
import { fileURL, fmtDay, fmtTotals, groupBy, money, monthLabel, totals } from "../format.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { logo, pageHead } from "../ui/parts.js";
import { KIND } from "../words.js";

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

// invoiceList is the list alone: the search and the filters redraw only it.
export function invoiceList() {
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

export function viewInvoices() {
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
