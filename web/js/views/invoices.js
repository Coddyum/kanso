// Factures: every expense read in the folder, by month, with a search and two filters.

import { esc } from "../dom.js";
import { fileURL, fmtDay, fmtTotals, groupBy, money, monthLabel, totals } from "../format.js";
import { lang, t } from "../i18n.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { logo, pageHead } from "../ui/parts.js";
import { kindName } from "../words.js";

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
    `<a class="pill k-${f.kind}" href="${fileURL(f.path)}" target="_blank" rel="noopener" title="${esc(f.path)}">${kindName(f.kind)}</a>`;

const invRow = (i) => `
    <div class="row">
        ${logo(i.vendor || "?", i.domain)}
        <div class="r-main"><b>${esc(i.vendor || t("common.unknownVendor"))}</b><span>${esc([i.label, i.number && t("invoices.number", { n: i.number })].filter(Boolean).join(" · ") || i.folder)}</span></div>
        <div class="r-files">${i.files.map(filePill).join("")}${i.edited ? `<span class="pill k-edited" title="${t("invoices.editedTitle")}">${t("invoices.edited")}</span>` : ""}</div>
        <div class="r-date">${fmtDay(i.date)}</div>
        <div class="r-amt ${i.amount == null ? "none" : ""}">${i.amount == null ? t("invoices.noAmount") : money(i.amount, i.currency)}</div>
        <div class="r-act">
            <button class="btn sm ghost icon" data-action="edit-invoice" data-id="${i.id}" title="${t("invoices.fix")}">${icon("edit", 14)}</button>
            <button class="btn sm ghost icon" data-action="reveal" data-path="${esc(i.files[0].path)}" title="${t("common.showInFinder")}">${icon("finder", 14)}</button>
        </div>
    </div>`;

// invoiceList is the list alone: the search and the filters redraw only it.
export function invoiceList() {
    const list = filteredInvoices();
    if (!list.length) return `<div class="card files-empty">${t("invoices.noMatch")}</div>`;
    const months = groupBy(list, (i) => i.date.slice(0, 7));
    const sections = Object.entries(months).map(
        ([ym, items]) => `
        <section class="mgroup">
            <div class="stitle">${monthLabel(ym)}<em>${t("invoices.count", { count: items.length })}</em><span class="sum">${fmtTotals(totals(items))}</span></div>
            <div class="card">${items.map(invRow).join("")}</div>
        </section>`,
    );
    return `<div class="totals"><span>${t("invoices.shownTotal")}</span><b>${fmtTotals(totals(list))}</b><span>· ${t("invoices.count", { count: list.length })}</span></div>${sections.join("")}`;
}

export function viewInvoices() {
    const inv = S.st.invoices;
    const years = [...new Set(inv.map((i) => i.date.slice(0, 4)))].sort().reverse();
    const vendors = [...new Set(inv.map((i) => i.vendor).filter(Boolean))].sort((a, b) => a.localeCompare(b, lang));
    const merged = inv.reduce((n, i) => n + i.files.length - 1, 0);
    const sub = [t("invoices.read", { count: inv.length }), merged && t("invoices.merged", { count: merged }), !S.st.extractor && t("invoices.noReader")].filter(Boolean).join(" · ");
    const tools = `
        <label class="search">${icon("search", 15)}<input class="field" id="inv-q" placeholder="${t("invoices.search")}" value="${esc(S.q)}" autocomplete="off"></label>
        <select class="select" id="inv-year"><option value="all">${t("invoices.allYears")}</option>${years.map((y) => `<option ${S.year === y ? "selected" : ""}>${y}</option>`).join("")}</select>
        <select class="select" id="inv-vendor"><option value="all">${t("invoices.allVendors")}</option>${vendors.map((v) => `<option ${S.vendor === v ? "selected" : ""}>${esc(v)}</option>`).join("")}</select>`;
    return `${pageHead({ title: t("invoices.title"), sub, tools })}
    <div id="inv-list">${invoiceList()}</div>`;
}
