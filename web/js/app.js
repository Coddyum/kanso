// Loading and drawing: the server's state, the current screen's own data, and the redraw.

import { api, sendFiles } from "./api.js";
import { $, $$ } from "./dom.js";
import { iso } from "./format.js";
import { t } from "./i18n.js";
import { rootName, S } from "./state.js";
import { toast } from "./ui/toast.js";
import { viewAnalytics } from "./views/analytics.js";
import { fitCalendar, gridRange, viewCalendar } from "./views/calendar.js";
import { viewFiles } from "./views/files.js";
import { viewInvoices } from "./views/invoices.js";
import { viewMissing } from "./views/missing.js";
import { viewSettings } from "./views/settings.js";
import { viewWelcome } from "./views/welcome.js";
import { hidePop } from "./views/popover.js";

const VIEWS = { calendar: viewCalendar, analytics: viewAnalytics, invoices: viewInvoices, missing: viewMissing, files: viewFiles, settings: viewSettings, welcome: viewWelcome };

export async function loadCalendar() {
    const { from, to } = gridRange(S.month);
    S.events = await api("GET", `/api/calendar?from=${iso(from)}&to=${iso(to)}`);
}

// loadFiles reads the folder shown; a folder gone since falls back to the root.
async function loadFiles() {
    try {
        S.listing = await api("GET", "/api/ls?dir=" + encodeURIComponent(S.dir));
    } catch (err) {
        if (!S.dir) throw err;
        toast(t("toast.folderGone", { name: S.dir }), true);
        S.dir = "";
        history.replaceState(null, "", "#files");
        S.listing = await api("GET", "/api/ls?dir=");
    }
}

// fetchView loads what the current screen needs besides the state.
export async function fetchView() {
    if (S.view === "calendar") await loadCalendar();
    else if (S.view === "analytics") S.an.data = await api("GET", "/api/analytics");
    else if (S.view === "files") await loadFiles();
    else if (S.view === "settings" || S.view === "welcome") S.settings = await api("GET", "/api/settings");
}

// load reads everything again, then redraws; the Mac app is told so its reminders follow.
export async function load() {
    try {
        const [sg, st] = await Promise.all([api("GET", "/api/sig"), api("GET", "/api/state"), fetchView()]);
        S.sig = sg.sig;
        S.boot ??= sg.boot;
        S.st = st;
        const added = S.st.autoAdded;
        if (added.length) toast(t("toast.autoAdded", { count: added.length, names: added.join(", ") }));
        render();
        window.webkit?.messageHandlers?.inc?.postMessage("changed");
    } catch (err) {
        toast(err.message, true);
    }
}

export function render() {
    hidePop();
    $$(".nav a").forEach((a) => a.classList.toggle("on", a.dataset.view === S.view));
    $("#n-missing").textContent = S.st.missing.length || "";
    $("#nav-files .tip").textContent = t("nav.folder", { name: rootName() });
    $("#nav-files").setAttribute("aria-label", t("nav.folder", { name: rootName() }));
    $("#sync").title = t("nav.synced", { path: S.st.root.replace(/^\/Users\/[^/]+/, "~") });
    const main = $("#main");
    main.classList.toggle("fit", S.view === "calendar");
    main.innerHTML = VIEWS[S.view]();
    if (S.view === "calendar") fitCalendar();
}

// upload sends files to the folder, says where they went, and reloads.
export async function upload(files, params) {
    if (!files.length) return;
    const res = await sendFiles(files, params);
    const n = res.saved.length;
    toast(t("toast.filed", { count: n, name: res.dir || rootName() }));
    await load();
}
