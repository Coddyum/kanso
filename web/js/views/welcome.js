// Premiers pas: the first-run checklist. One card per step, the open one with
// its controls; every step can be skipped and taken up again. The list opens
// by itself on a new install, and again from Settings.

import { esc } from "../dom.js";
import { capitalize } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { pageHead } from "../ui/parts.js";
import { inApp, notifyState } from "./settings.js";

const step = (id, state, body) => `
    <div class="ob-actions">
        ${state === "todo" ? `<button class="btn ghost" data-action="ob-step" data-step="${id}" data-state="skipped">${t("welcome.skip")}</button>` : ""}
        ${body}
    </div>`;

const doneButton = (id, label) => `<button class="btn primary" data-action="ob-step" data-step="${id}" data-state="done">${label}</button>`;

function langStep(state) {
    const s = S.settings;
    const native = (code) => capitalize(new Intl.DisplayNames([code], { type: "language" }).of(code));
    const opts = [`<option value="" ${s.chosen ? "" : "selected"}>${t("settings.lang.auto")}</option>`]
        .concat(s.langs.map((c) => `<option value="${c}" ${s.chosen === c ? "selected" : ""}>${esc(native(c))}</option>`))
        .join("");
    return `<div class="ob-fields"><select class="select" data-ob="lang" aria-label="${t("settings.lang.label")}">${opts}</select></div>
        ${step("lang", state, doneButton("lang", t("welcome.next")))}`;
}

function folderStep(state) {
    const root = S.settings.root;
    return `<div class="ob-fields"><code class="set-path" title="${esc(root)}">${esc(root.replace(/^\/Users\/[^/]+/, "~"))}</code></div>
        <div class="set-actions">
            <button class="btn" data-action="reveal" data-path="">${icon("finder", 14)}${t("files.openInFinder")}</button>
            <button class="btn ghost" data-action="set-folder" data-move="">${t("settings.folder.other")}</button>
        </div>
        ${step("folder", state, doneButton("folder", t("welcome.steps.folder.ok")))}`;
}

function reserveStep(state) {
    const f = S.st.forecast;
    return `<div class="ob-fields">
            <label class="f">${t("reserve.balance")}<input id="ob-amount" type="number" step="0.01" value="${f.set ? f.balance : ""}"></label>
            <label class="f">${t("reserve.date")}<input id="ob-date" type="date" value="${S.st.today}"></label>
        </div>
        ${step("reserve", state, `<button class="btn primary" data-action="ob-reserve">${t("welcome.steps.reserve.save")}</button>`)}`;
}

function notifyStep(state) {
    const app = S.settings.app;
    const appPart = inApp()
        ? `<div class="set-actions"><button class="btn" data-action="set-test">${t("settings.notify.test")}</button></div>
           <div class="set-row"><div class="set-label">${t("settings.notify.login")}<small>${t("settings.notify.loginHint")}</small></div>
           <div class="set-control"><label class="switch"><input type="checkbox" data-set="app.loginItem" ${app.loginItem ? "checked" : ""}><span></span></label></div></div>`
        : `<p class="set-note">${t("settings.notify.appOnly")}</p>`;
    return `<p class="set-state ${notifyState(app)}">${t(`settings.notify.state.${notifyState(app)}`)}</p>
        ${appPart}
        <p class="set-note">${t("settings.notify.focus")}</p>
        ${step("notify", state, doneButton("notify", t("welcome.steps.notify.ok")))}`;
}

const BODIES = { lang: langStep, folder: folderStep, reserve: reserveStep, notify: notifyStep };

// openStep is the card shown open: the one clicked, or the first step still to do.
const openStep = (steps) => (S.ob.open ? S.ob.open : steps.find((s) => s.state === "todo")?.id || "");

function card(s, i, open) {
    const mark = s.state === "done" ? icon("check", 13) : String(i + 1);
    return `
    <section class="card ob-step is-${s.state} ${open ? "is-open" : ""}">
        <button class="ob-head" data-action="ob-open" data-step="${s.id}" aria-expanded="${open}">
            <span class="ob-num">${mark}</span>
            <h2>${t(`welcome.steps.${s.id}.title`)}</h2>
            <small>${t(`welcome.state.${s.state}`)}</small>
        </button>
        ${open ? `<div class="ob-body"><p>${t(`welcome.steps.${s.id}.text`)}</p>${BODIES[s.id](s.state)}</div>` : ""}
    </section>`;
}

export function viewWelcome() {
    const steps = S.st.onboarding.steps;
    const open = openStep(steps);
    const settled = steps.filter((s) => s.state !== "todo").length;
    const finished = settled === steps.length;
    return `
    ${pageHead({ title: t("welcome.title"), sub: t("welcome.sub") })}
    <div class="welcome">
        <div class="card ob-local">${icon("lock", 18)}<div><b>${t("welcome.local")}</b><p>${t("settings.privacy.sub")}</p></div></div>
        <div class="ob-steps">${steps.map((s, i) => card(s, i, s.id === open)).join("")}</div>
        <div class="ob-foot">
            <span>${t("welcome.progress", { done: settled, total: steps.length })}</span>
            <span class="grow"></span>
            <button class="btn ${finished ? "primary" : "ghost"}" data-action="ob-close">${t(finished ? "welcome.start" : "welcome.later")}</button>
        </div>
    </div>`;
}
