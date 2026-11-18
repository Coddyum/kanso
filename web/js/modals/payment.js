// The payment form: create one, or change its amount, cycle, project, invoice page…

import { api } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtDay, money } from "../format.js";
import { icon } from "../icons.js";
import { projectName, S } from "../state.js";
import { openModal } from "../ui/modal.js";
import { currencyOptions, logo } from "../ui/parts.js";
import { toast } from "../ui/toast.js";
import { CYCLES } from "../words.js";

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

// paymentModal edits p, or creates it when it has no id; a suggestion says where it was found.
export function paymentModal(p = {}, suggestion = null) {
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
            toast(isNew ? "Paiement ajouté" : "Paiement modifié");
            await load();
        },
    );
}
