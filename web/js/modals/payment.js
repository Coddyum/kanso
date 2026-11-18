// The payment form: create one, or change its amount, cycle, project, invoice page…

import { api } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtDay, money } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { projectName, S } from "../state.js";
import { openModal } from "../ui/modal.js";
import { currencyOptions, logo } from "../ui/parts.js";
import { toast } from "../ui/toast.js";
import { CYCLE_CODES, cycleLong } from "../words.js";

// The project a payment was filed in by hand (« - » for none), or "" when it follows the keywords.
const chosenProject = (p) => (p.projectBy === "service" ? p.project || "-" : "");

function projectField(p) {
    if (!S.st.projects.length) return "";
    const value = chosenProject(p);
    const auto = p.projectBy === "keyword" ? p.project : "";
    const opts = [
        `<option value="">${esc(auto ? t("analytics.autoProject", { name: projectName(auto) }) : t("analytics.unfiled"))}</option>`,
        ...S.st.projects.map((x) => `<option value="${x.id}" ${value === x.id ? "selected" : ""}>${esc(x.name)}</option>`),
        auto || value === "-" ? `<option value="-" ${value === "-" ? "selected" : ""}>${t("analytics.noneProject")}</option>` : "",
    ];
    return `<label class="f">${t("analytics.project")}<select name="project">${opts.join("")}</select></label>`;
}

// paymentModal edits p, or creates it when it has no id; a suggestion says where it was found.
export function paymentModal(p = {}, suggestion = null) {
    const isNew = !p.id;
    const sub = suggestion
        ? t("payment.found", { count: suggestion.count, date: fmtDay(suggestion.last) })
        : p.auto
          ? t("payment.autoAdded")
          : t("payment.intro");
    const shown = isNew ? p.amount : p.current;
    const shownCurrency = isNew ? p.currency : p.currentCurrency;
    const amountNote = p.variable
        ? t("payment.variableNote", { amounts: p.recent.map((a) => money(a, p.currentCurrency)).join(" · ") })
        : p.manual
          ? t("payment.manualNote")
          : "";
    openModal(
        `<form class="form">
        <h2>${logo(p.name || "?", p.domain, "lg")}${isNew ? t("payment.new") : esc(p.name)}</h2>
        <p class="sub">${sub}</p>
        <div class="grid2">
            <label class="f span2">${t("payment.name")}<input name="name" required value="${esc(p.name || "")}" placeholder="${t("payment.namePlaceholder")}" autofocus></label>
            <label class="f">${t(p.variable ? "payment.amountVariable" : "payment.amount")}<input name="amount" type="number" step="0.01" min="0" required value="${shown ?? ""}"><span class="hint">${t("payment.amountHint")}</span></label>
            <label class="f">${t("payment.currency")}<select name="currency">${currencyOptions(shownCurrency)}</select></label>
            ${amountNote ? `<p class="note span2">${amountNote}</p>` : ""}
            <label class="f">${t("payment.cycle")}<select name="cycle">${CYCLE_CODES.map((k) => `<option value="${k}" ${k === (p.cycle || "monthly") ? "selected" : ""}>${cycleLong(k)}</option>`).join("")}</select></label>
            <label class="f">${t("payment.start")}<input name="start" type="date" required value="${p.start || S.st.today}"></label>
            <label class="f">${t("payment.vendor")}<input name="vendor" value="${esc(p.vendor || "")}" placeholder="${t("payment.vendorPlaceholder")}"><span class="hint">${t("payment.vendorHint")}</span></label>
            <label class="f">${t("payment.domain")}<input name="domain" value="${esc(p.domain || "")}" placeholder="render.com"></label>
            <label class="f">${t("payment.end")}<input name="end" type="date" value="${p.end || ""}"></label>
            <label class="f">${t("payment.match")}<input name="match" value="${esc(p.match || "")}" placeholder="${t("payment.matchPlaceholder")}"><span class="hint">${t("payment.matchHint")}</span></label>
            <label class="f span2">${t("payment.billing")}<input name="billing" value="${esc(p.billing || "")}" placeholder="${esc(p.billingUrl && !p.billing ? p.billingUrl : "https://…")}"><span class="hint">${p.billingUrl && !p.billing ? t("payment.billingKnown") : t("payment.billingHint")}</span></label>
            ${projectField(p)}
            <label class="f ${S.st.projects.length ? "" : "span2"}">${t("payment.note")}<input name="note" value="${esc(p.note || "")}"></label>
            <label class="check span2"><input type="checkbox" name="expectInvoice" ${p.expectInvoice === false ? "" : "checked"}>${t("payment.expectInvoice")}</label>
        </div>
        <footer>
            ${isNew ? "" : `<button type="button" class="btn ghost danger" data-action="delete-payment" data-id="${p.id}">${icon("trash", 15)}${t("common.delete")}</button>`}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button>
            <button class="btn primary">${t("common.save")}</button>
        </footer>
    </form>`,
        async (data) => {
            const amount = parseFloat(String(data.amount).replace(",", ".")) || 0;
            const { project = "", ...fields } = data;
            await api("POST", "/api/payments", {
                ...fields,
                id: p.id || "",
                amount,
                amountChanged: amount !== shown || fields.currency !== (shownCurrency || "EUR"),
                expectInvoice: "expectInvoice" in fields,
                auto: !!p.auto,
                suggestKey: p.suggestKey || suggestion?.key || "",
            });
            if (S.st.projects.length && project !== chosenProject(p)) {
                await api("POST", "/api/assign", { kind: "service", ref: fields.name, project });
            }
            toast(t(isNew ? "payment.added" : "payment.saved"));
            await load();
        },
    );
}
