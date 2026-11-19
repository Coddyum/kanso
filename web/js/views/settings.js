// Réglages: the language, the folder, the administrative deadlines, the
// reminders, what the app may download, and the data (export, backups,
// diagnostic). Everything saves as soon as it changes.

import { esc } from "../dom.js";
import { capitalize, fmtMod, fmtSize } from "../format.js";
import { lang, t } from "../i18n.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { pageHead } from "../ui/parts.js";

// inApp says whether the page runs in the Mac app, which alone can test a notification or pick a folder.
export const inApp = () => !!window.webkit?.messageHandlers?.inc;

// notifyState says whether macOS lets the app notify: on, off, or unknown while the app has not reported.
export const notifyState = (app) =>
    !app.seen || app.seen.startsWith("0001") ? "unknown" : app.notifications === "authorized" || app.notifications === "provisional" ? "on" : "off";

const section = (id, title, body) => `
    <section class="card set-card" id="set-${id}">
        <h2>${t(`settings.${id}.title`)}</h2>
        ${title ? `<p class="set-sub">${title}</p>` : ""}
        ${body}
    </section>`;

const row = (label, control, hint = "") => `
    <div class="set-row"><div class="set-label">${label}${hint ? `<small>${hint}</small>` : ""}</div><div class="set-control">${control}</div></div>`;

const toggle = (name, on, attrs = "") =>
    `<label class="switch"><input type="checkbox" data-set="${name}" ${on ? "checked" : ""} ${attrs}><span></span></label>`;

function langSection() {
    const s = S.settings;
    const native = (code) => capitalize(new Intl.DisplayNames([code], { type: "language" }).of(code));
    const opts = [`<option value="" ${s.chosen ? "" : "selected"}>${t("settings.lang.auto")}</option>`]
        .concat(s.langs.map((c) => `<option value="${c}" ${s.chosen === c ? "selected" : ""}>${esc(native(c))}</option>`))
        .join("");
    return section("lang", "", row(t("settings.lang.label"), `<select class="select" data-set="lang">${opts}</select>`));
}

function folderSection() {
    const s = S.settings;
    return section(
        "folder",
        t("settings.folder.sub"),
        `${row(t("settings.folder.where"), `<code class="set-path" title="${esc(s.root)}">${esc(s.root.replace(/^\/Users\/[^/]+/, "~"))}</code>`)}
        <div class="set-actions">
            <button class="btn" data-action="reveal" data-path="">${icon("finder", 14)}${t("files.openInFinder")}</button>
            <button class="btn ghost" data-action="set-folder" data-move="">${t("settings.folder.other")}</button>
            <button class="btn ghost" data-action="set-folder" data-move="1">${t("settings.folder.move")}</button>
        </div>`,
    );
}

function adminSection() {
    const a = S.settings.admin;
    const urssaf = ["quarterly", "monthly", "off"].map((k) => `<option value="${k}" ${a.urssaf === k ? "selected" : ""}>${t(`deadlines.urssaf.${k}`)}</option>`).join("");
    return section(
        "admin",
        t("settings.admin.sub"),
        `${row(t("deadlines.urssafLabel"), `<select class="select" data-set="admin.urssaf">${urssaf}</select>`)}
        ${row(t("deadlines.cfe"), toggle("admin.cfe", a.cfe))}
        ${row(t("deadlines.ir"), toggle("admin.ir", a.ir))}
        ${row(t("settings.admin.created"), `<input class="field" type="date" data-set="admin.created" value="${esc(a.created || "")}">`, t("settings.admin.createdHint"))}`,
    );
}

function notifySection() {
    const { notify: n, app } = S.settings;
    const hours = [...Array(24).keys()].map((h) => `<option value="${h}" ${n.hour === h ? "selected" : ""}>${new Intl.DateTimeFormat(lang, { hour: "numeric", minute: "2-digit" }).format(new Date(2026, 0, 1, h))}</option>`).join("");
    const state = notifyState(app);
    const appRows = inApp()
        ? `<div class="set-actions">
            <button class="btn" data-action="set-test">${t("settings.notify.test")}</button>
            ${state === "off" ? `<button class="btn" data-action="set-open-notifications">${t("settings.notify.openSystem")}</button>` : ""}
        </div>
        ${row(t("settings.notify.login"), toggle("app.loginItem", app.loginItem), t("settings.notify.loginHint"))}`
        : `<p class="set-note">${t("settings.notify.appOnly")}</p>`;
    return section(
        "notify",
        `<span class="set-state ${state}">${t(`settings.notify.state.${state}`)}</span>`,
        `${["payments", "deadlines", "missing", "reserve"].map((k) => row(t(`settings.notify.${k}`), toggle(`notify.${k}`, n[k]))).join("")}
        ${row(t("settings.notify.hour"), `<select class="select" data-set="notify.hour">${hours}</select>`)}
        ${appRows}
        <p class="set-note">${t("settings.notify.focus")}</p>`,
    );
}

function privacySection() {
    const p = S.settings.privacy;
    return section(
        "privacy",
        t("settings.privacy.sub"),
        `${row(t("settings.privacy.rates"), toggle("privacy.rates", p.rates), t("settings.privacy.ratesHint"))}
        ${row(t("settings.privacy.logos"), toggle("privacy.logos", p.logos), t("settings.privacy.logosHint"))}`,
    );
}

function dataSection() {
    const { backups } = S.settings;
    const list = backups.length
        ? backups
              .slice(0, 10)
              .map((b) => `<div class="set-backup"><span>${fmtMod(b.date)}</span><small>${esc(b.name)} · ${fmtSize(b.size)}</small><button class="btn sm ghost" data-action="set-restore" data-name="${esc(b.name)}">${t("settings.data.restore")}</button></div>`)
              .join("")
        : `<p class="set-note">${t("settings.data.noBackup")}</p>`;
    return section(
        "data",
        t("settings.data.sub"),
        `<div class="set-actions">
            <button class="btn" data-action="bank-import">${icon("wallet", 14)}${t("settings.data.bank")}</button>
            <button class="btn" data-action="set-export">${icon("upload", 14)}${t("settings.data.export")}</button>
            <button class="btn ghost" data-action="set-diagnostic">${t("settings.data.diagnostic")}</button>
        </div>
        <h3>${t("settings.data.backups")}</h3>${list}`,
    );
}

function startSection() {
    return section("start", t("settings.start.sub"), `<div class="set-actions"><button class="btn" data-action="ob-reopen">${t("settings.start.reopen")}</button></div>`);
}

export function viewSettings() {
    return `
    ${pageHead({ title: t("settings.title"), sub: t("settings.sub", { version: esc(S.settings.version) }) })}
    <div class="settings">${langSection()}${folderSection()}${adminSection()}${notifySection()}${privacySection()}${dataSection()}${startSection()}</div>`;
}
