// The smaller forms: a project, an invoice's corrections, the reserve, the
// administrative deadlines, and the list of a crowded calendar day.

import { api } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtDay } from "../format.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { openModal } from "../ui/modal.js";
import { currencyOptions, dateBadge, logo } from "../ui/parts.js";
import { toast } from "../ui/toast.js";
import { upItem } from "../views/calendar.js";
import { DL, PROJECT_TONES } from "../words.js";

export function projectModal(p = {}) {
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
            const keywords = String(data.keywords || "")
                .split(",")
                .map((k) => k.trim())
                .filter(Boolean);
            await api("POST", "/api/projects", { id: p.id || "", name: data.name, color: data.color, keywords });
            toast(isNew ? "Projet créé" : "Projet modifié");
            await load();
        },
    );
}

// invoiceModal corrects what was read in an invoice; only the fields changed are sent.
export function invoiceModal(i) {
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

export function reserveModal() {
    const f = S.st.forecast;
    const rate = f.usdToEur
        ? `Les montants en dollars sont convertis au taux de la Banque centrale européenne du ${fmtDay(f.rateDate)} : 1 $ = ${f.usdToEur.toLocaleString("fr-FR")} €.`
        : "Taux de change indisponible : les montants en dollars ne sont pas comptés.";
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
export function deadlinesModal() {
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
    const urssaf = [
        ["quarterly", "Chaque trimestre"],
        ["monthly", "Chaque mois"],
        ["off", "Pas de déclaration"],
    ]
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

// dayModal lists every event of a day the calendar cell has no room for.
export function dayModal(date) {
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
