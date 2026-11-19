// Importing a bank statement (CSV or OFX): the file is read by the local
// server, in memory, then forgotten. When its columns cannot be told apart,
// the user says which is which; then the balance and the subscriptions read
// are shown, ticked, and added only once confirmed.

import { api, sendBody } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtDay, money } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { openModal } from "../ui/modal.js";
import { logo } from "../ui/parts.js";
import { toast } from "../ui/toast.js";
import { cycleShort } from "../words.js";

// The file chosen stays in the page while the window is open: never sent anywhere else.
let file = null;
let last = null;

const ROLES = ["date", "label", "amount", "debit", "credit", "balance", "currency"];

const footer = (main) => `<footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button>${main}</footer>`;

// bankModal opens the first screen: what to export from the bank, and the file picker.
export function bankModal() {
    file = null;
    openModal(
        `<form class="form wide">
        <h2>${icon("wallet", 18)}${t("bank.title")}</h2>
        <p class="sub">${t("bank.intro")}</p>
        <label class="bank-drop">${icon("upload", 18)}<span>${t("bank.pick")}</span><small>${t("bank.formats")}</small>
            <input type="file" data-bank="file" accept=".csv,.ofx,.qfx,.txt,text/csv"></label>
        <p class="note">${icon("lock", 13)} ${t("bank.private")}</p>
        ${footer("")}
    </form>`,
        () => {},
    );
}

// onBankFile reads the file picked, then shows what it holds.
export async function onBankFile(input) {
    file = input.files[0];
    if (!file) return;
    await preview("");
}

async function preview(map) {
    last = await sendBody("/api/bank/preview" + (map ? "?map=" + map : ""), file);
    if (last.needsMapping) return mappingScreen(last);
    resultScreen(last);
}

function columnName(p, i) {
    return p.columns[i] || t("bank.column", { n: i + 1 });
}

// mappingScreen asks which column is the date, the label, the amount (or the debit and the credit), the balance.
function mappingScreen(p) {
    const select = (role) => {
        const opts = [`<option value="-1">${t("bank.none")}</option>`]
            .concat([...Array(p.width).keys()].map((i) => `<option value="${i}" ${p.mapping[role] === i ? "selected" : ""}>${esc(columnName(p, i))}</option>`))
            .join("");
        return `<label class="f">${t(`bank.role.${role}`)}<select name="${role}">${opts}</select></label>`;
    };
    const head = [...Array(p.width).keys()].map((i) => `<th>${esc(columnName(p, i))}</th>`).join("");
    const rows = p.sample.map((r) => `<tr>${r.map((c) => `<td>${esc(c)}</td>`).join("")}</tr>`).join("");
    openModal(
        `<form class="form wide">
        <h2>${t("bank.mapTitle")}</h2>
        <p class="sub">${t("bank.mapIntro")}</p>
        <div class="bank-sample"><table><thead><tr>${head}</tr></thead><tbody>${rows}</tbody></table></div>
        <div class="grid2">${ROLES.map(select).join("")}</div>
        <p class="set-note">${t("bank.mapHint")}</p>
        ${footer(`<button class="btn primary">${t("bank.read")}</button>`)}
    </form>`,
        async (data) => {
            const map = ROLES.map((r) => data[r]).join(",");
            // The next screen opens once this one has closed.
            setTimeout(() => preview(map).catch((err) => toast(err.message, true)));
        },
    );
}

function subRow(s, i) {
    const known = s.known ? `<small class="bank-known">${t("bank.known")}</small>` : "";
    return `<label class="bank-sub ${s.known ? "is-known" : ""}">
        <input type="checkbox" name="sub${i}" ${s.known ? "disabled" : "checked"}>
        ${logo(s.name, s.domain, "sm")}
        <span class="bank-name"><b>${esc(s.name)}</b><small>${t("bank.seen", { count: s.count, date: fmtDay(s.start) })}</small></span>
        ${known}
        <span class="bank-amount">${money(s.amount, s.currency)} <small>${cycleShort(s.cycle)}</small></span>
    </label>`;
}

function balanceRow(b) {
    if (!b) return "";
    const amount = b.eur != null ? money(b.eur, "EUR") : money(b.amount, b.currency);
    return `<label class="check bank-balance"><input type="checkbox" name="reserve" ${b.eur != null ? "checked" : "disabled"}>
        ${t("bank.reserve", { amount, date: fmtDay(b.date) })}</label>`;
}

// resultScreen shows what the file holds; nothing is written before « Add ».
function resultScreen(p) {
    const subs = p.subscriptions;
    const list = subs.length ? subs.map(subRow).join("") : `<p class="set-note">${t("bank.noSub")}</p>`;
    const remap = p.format === "csv" ? `<button type="button" class="btn ghost" data-action="bank-remap">${t("bank.remap")}</button>` : "";
    openModal(
        `<form class="form wide">
        <h2>${icon("wallet", 18)}${t("bank.title")}</h2>
        <p class="sub">${t("bank.summary", { count: p.count, from: fmtDay(p.from), to: fmtDay(p.to) })}</p>
        ${balanceRow(p.balance)}
        <h3 class="bank-h">${t("bank.subsTitle")}</h3>
        <div class="bank-subs">${list}</div>
        <footer>${remap}<span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button><button class="btn primary">${t("bank.add")}</button></footer>
    </form>`,
        async (data) => {
            const chosen = subs.filter((s, i) => !s.known && data[`sub${i}`]);
            const reserve = data.reserve && p.balance?.eur != null ? { amount: p.balance.eur, date: p.balance.date } : null;
            const res = await api("POST", "/api/bank/apply", { subscriptions: chosen, reserve });
            toast(t("bank.added", { count: res.added }));
            file = null;
            await load();
        },
    );
}

export const BANK_ACTIONS = {
    "bank-import": () => bankModal(),
    "bank-remap": () => mappingScreen({ ...last, mapping: last.mapping }),
};
