// The smaller forms: a project, an invoice's corrections, the reserve, the
// administrative deadlines, and the list of a crowded calendar day.

import { api } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtDay } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { openModal } from "../ui/modal.js";
import { currencyOptions, dateBadge, foreignIn, logo, rateList } from "../ui/parts.js";
import { toast } from "../ui/toast.js";
import { upItem } from "../views/calendar.js";
import { DL_CLASS, deadlineState, PROJECT_TONES } from "../words.js";

export function projectModal(p = {}) {
    const isNew = !p.id;
    const used = new Set(S.an.data.projects.filter((x) => x.id !== p.id).map((x) => x.color));
    const color = p.color || PROJECT_TONES.find((c) => !used.has(c)) || PROJECT_TONES[0];
    openModal(
        `<form class="form">
        <h2>${isNew ? t("project.new") : esc(p.name)}</h2>
        <p class="sub">${t("project.intro")}</p>
        <div class="grid2">
            <label class="f span2">${t("project.name")}<input name="name" required value="${esc(p.name || "")}" placeholder="${t("project.namePlaceholder")}" autofocus></label>
            <div class="f span2">${t("project.color")}<div class="colors">${PROJECT_TONES.map((c) => `<label><input type="radio" name="color" value="${c}" ${c === color ? "checked" : ""}><span class="swatch c-${c}"></span></label>`).join("")}</div></div>
            <label class="f span2">${t("project.keywords")}<input name="keywords" value="${esc((p.keywords || []).join(", "))}" placeholder="${t("project.keywordsPlaceholder")}"><span class="hint">${t("project.keywordsHint")}</span></label>
        </div>
        <footer>
            ${isNew ? "" : `<button type="button" class="btn ghost danger" data-action="delete-project" data-id="${p.id}">${icon("trash", 15)}${t("common.delete")}</button>`}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button>
            <button class="btn primary">${t(isNew ? "common.create" : "common.save")}</button>
        </footer>
    </form>`,
        async (data) => {
            const keywords = String(data.keywords || "")
                .split(",")
                .map((k) => k.trim())
                .filter(Boolean);
            await api("POST", "/api/projects", { id: p.id || "", name: data.name, color: data.color, keywords });
            toast(t(isNew ? "project.created" : "project.saved"));
            await load();
        },
    );
}

// invoiceModal corrects what was read in an invoice; only the fields changed are sent.
export function invoiceModal(i) {
    openModal(
        `<form class="form">
        <h2>${logo(i.vendor || "?", i.domain, "lg")}${t("invoice.title")}</h2>
        <p class="sub">${i.files.map((f) => esc(f.path)).join("<br>")}</p>
        <div class="grid2">
            <label class="f">${t("invoice.vendor")}<input name="vendor" value="${esc(i.vendor || "")}" autofocus></label>
            <label class="f">${t("invoice.label")}<input name="label" value="${esc(i.label || "")}"></label>
            <label class="f">${t("invoice.date")}<input name="date" type="date" value="${i.date}"></label>
            <label class="f">${t("invoice.amount")}<input name="amount" type="number" step="0.01" value="${i.amount ?? ""}"></label>
            <label class="f">${t("payment.currency")}<select name="currency">${currencyOptions(i.currency)}</select></label>
            <span></span>
            <label class="check span2"><input type="checkbox" name="notInvoice">${t("invoice.notInvoice")}</label>
        </div>
        <footer>
            ${i.edited ? `<button type="button" class="btn ghost" data-action="reset-invoice" data-id="${i.id}">${t("invoice.reset")}</button>` : ""}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button>
            <button class="btn primary">${t("common.save")}</button>
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
            toast(t("invoice.saved"));
            await load();
        },
    );
}

export function reserveModal() {
    const f = S.st.forecast;
    const foreign = foreignIn(S.st.payments.map((p) => p.currentCurrency));
    const rate = !foreign.length
        ? ""
        : f.perEUR
          ? t("reserve.rates", { date: fmtDay(f.rateDate), rates: rateList(f.perEUR, foreign) })
          : t("reserve.noRates");
    openModal(
        `<form class="form">
        <h2>${t("reserve.title")}</h2>
        <p class="sub">${t("reserve.intro")}</p>
        <div class="grid2">
            <label class="f">${t("reserve.balance")}<input name="amount" type="number" step="0.01" required value="${f.set ? f.balance : ""}" autofocus></label>
            <label class="f">${t("reserve.date")}<input name="date" type="date" value="${S.st.today}"></label>
        </div>
        ${rate ? `<p class="sub" style="margin-top: 14px">${rate}</p>` : ""}
        <footer>
            ${f.set ? `<button type="button" class="btn ghost danger" data-action="clear-reserve">${t("reserve.clear")}</button>` : ""}
            <span class="grow"></span>
            <button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button>
            <button class="btn primary">${t("common.save")}</button>
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
        const done = d.status === "done";
        return `<div class="dl-row">
            ${dateBadge(d.date)}
            <span class="pm"><b>${esc(d.title)}${d.approx ? `<span class="mini">${t("deadlines.approx")}</span>` : ""}</b><span>${esc(d.detail)}</span></span>
            <span class="tag ${DL_CLASS[d.status]}">${deadlineState(d.status)}</span>
            <a class="btn sm ghost icon" href="${esc(d.url)}" target="_blank" rel="noopener" title="${t("deadlines.openSite")}">${icon("external", 14)}</a>
            <button type="button" class="btn sm" data-action="toggle-deadline" data-key="${esc(d.key)}" data-done="${done ? "" : "1"}">${t(done ? "common.cancel" : "deadlines.done")}</button>
        </div>`;
    };
    const urssaf = ["quarterly", "monthly", "off"].map((k) => `<option value="${k}" ${a.urssaf === k ? "selected" : ""}>${t(`deadlines.urssaf.${k}`)}</option>`).join("");
    openModal(
        `<form class="form wide">
        <h2>${t("deadlines.title")}</h2>
        <p class="sub">${t("deadlines.intro")}</p>
        <div class="dl-list">${S.st.deadlines.length ? S.st.deadlines.map(row).join("") : `<div class="empty-mini">${t("deadlines.none")}</div>`}</div>
        <div class="grid2 dl-settings">
            <label class="f">${t("deadlines.urssafLabel")}<select name="urssaf">${urssaf}</select></label>
            <div class="f">${t("deadlines.reminders")}
                <label class="check"><input type="checkbox" name="cfe" ${a.cfe ? "checked" : ""}>${t("deadlines.cfe")}</label>
                <label class="check"><input type="checkbox" name="ir" ${a.ir ? "checked" : ""}>${t("deadlines.ir")}</label>
            </div>
        </div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.close")}</button><button class="btn primary">${t("common.save")}</button></footer>
    </form>`,
        async (data) => {
            await api("POST", "/api/admin", { urssaf: data.urssaf, cfe: "cfe" in data, ir: "ir" in data });
            toast(t("deadlines.saved"));
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
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.close")}</button></footer>
    </form>`,
        async () => {},
    );
}
