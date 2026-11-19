// Kanso's page: plain modules, no framework and no build step. The Go server
// is the only source of truth; this file starts everything and keeps the page
// in step with the folder.

import { initActions } from "./actions.js";
import { api } from "./api.js";
import { fetchView, load, render } from "./app.js";
import { initDragDrop } from "./dnd.js";
import { $$ } from "./dom.js";
import { icon } from "./icons.js";
import { loadLocale, translatePage } from "./i18n.js";
import { readHash, S } from "./state.js";
import { modalOpen } from "./ui/modal.js";
import { toast } from "./ui/toast.js";
import { initPopover } from "./views/popover.js";

// isTyping: a filled field of the page has the focus; a reload would lose it.
const isTyping = () => {
    const a = document.activeElement;
    return a && a.closest("#main") && (a.tagName === "INPUT" || a.tagName === "SELECT") && a.value;
};

// poll asks the server every few seconds whether the folder changed (reload
// the data) or the server was updated (reload the page).
async function poll() {
    if (document.hidden || !S.st || modalOpen() || isTyping()) return;
    try {
        const { sig, boot } = await api("GET", "/api/sig");
        if (S.boot && boot !== S.boot) return location.reload();
        if (sig === S.sig) return;
        await load();
    } catch {
        /* the server may be restarting: next tick */
    }
}

// Switching screens keeps the state already loaded: only the screen's own data is fetched.
async function onHashChange() {
    readHash();
    if (!S.st) return load();
    try {
        await fetchView();
        render();
        window.scrollTo(0, 0);
    } catch (err) {
        toast(err.message, true);
    }
}

// start reads the language first: every text of the page comes from it.
async function start() {
    const { lang } = await api("GET", "/api/locale").catch(() => ({ lang: "fr" }));
    await loadLocale(lang);
    translatePage();
    initPopover();
    initActions();
    initDragDrop();
    $$("[data-icon]").forEach((el) => (el.outerHTML = icon(el.dataset.icon, 17)));
    window.addEventListener("hashchange", onHashChange);
    document.addEventListener("visibilitychange", poll);
    let resizeTimer;
    window.addEventListener("resize", () => {
        clearTimeout(resizeTimer);
        resizeTimer = setTimeout(() => S.st && S.view === "calendar" && render(), 120);
    });
    setInterval(poll, 4000);
    readHash();
    await load();
    // A new install opens on the first-run checklist, unless the address names a screen.
    if (!location.hash && S.st?.onboarding.show) location.hash = "welcome";
}

start();
