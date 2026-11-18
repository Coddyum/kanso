// What the Settings screen does. Each control saves as soon as it changes;
// what only the Mac app can do (a test notification, opening at login,
// picking a folder) is asked of it, and it answers through window.kanso*.

import { api } from "./api.js";
import { load, render } from "./app.js";
import { esc } from "./dom.js";
import { t } from "./i18n.js";
import { S } from "./state.js";
import { openModal, promptModal } from "./ui/modal.js";
import { toast } from "./ui/toast.js";
import { inApp } from "./views/settings.js";

const toApp = (message) => window.webkit?.messageHandlers?.inc?.postMessage(message);

async function reloadSettings() {
    S.settings = await api("GET", "/api/settings");
    if (S.view === "settings") render();
}

// confirmModal asks before an action that changes a lot: OK runs onOk.
function confirmModal(title, text, onOk) {
    openModal(
        `<form class="form">
        <h2>${esc(title)}</h2>
        <p class="sub">${esc(text)}</p>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.cancel")}</button><button class="btn primary">${t("common.ok")}</button></footer>
    </form>`,
        onOk,
    );
}

// useFolder points the app at path (or moves the folder into it); the server restarts and the page reloads.
function useFolder(path, move) {
    const key = move ? "settings.folder.moveConfirm" : "settings.folder.otherConfirm";
    confirmModal(t(move ? "settings.folder.move" : "settings.folder.other"), t(key, { path }), async () => {
        await api("POST", "/api/folder", { path, move });
        toast(t("settings.folder.restarting"));
    });
}

window.kansoFolderPicked = (path, move) => path && useFolder(path, !!move);
window.kansoAppChanged = () => reloadSettings().catch(() => {});

function diagnosticModal(d) {
    const rows = Object.entries(d).map(([k, v]) => `<div class="set-diag"><b>${esc(k)}</b><span>${esc(typeof v === "object" ? JSON.stringify(v) : String(v))}</span></div>`);
    openModal(
        `<form class="form wide">
        <h2>${t("settings.data.diagnostic")}</h2>
        <p class="sub">${t("settings.data.diagnosticSub")}</p>
        <div class="set-diags">${rows.join("")}</div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">${t("common.close")}</button><button class="btn primary">${t("settings.data.copy")}</button></footer>
    </form>`,
        async () => {
            await navigator.clipboard.writeText(JSON.stringify(d, null, 2));
            toast(t("settings.data.copied"));
        },
    );
}

export const SETTINGS_ACTIONS = {
    "set-folder": (el) => {
        const move = !!el.dataset.move;
        if (inApp()) return toApp({ type: "pickFolder", move });
        // The prompt closes itself once its answer is handled: the confirmation opens just after.
        promptModal(t(move ? "settings.folder.move" : "settings.folder.other"), t(move ? "settings.folder.moveLabel" : "settings.folder.otherLabel"), "", (path) => {
            setTimeout(() => useFolder(path, move));
        });
    },
    "set-test": () => {
        toApp({ type: "test" });
        toast(t("settings.notify.testSent"));
    },
    "set-open-notifications": () => toApp({ type: "openNotifications" }),
    "set-export": async () => {
        await api("POST", "/api/export");
        toast(t("settings.data.exported"));
    },
    "set-restore": (el) => {
        const b = S.settings.backups.find((x) => x.name === el.dataset.name);
        confirmModal(t("settings.data.restore"), t("settings.data.restoreConfirm", { name: b.name }), async () => {
            await api("POST", "/api/backups/restore", { name: b.name });
            toast(t("settings.data.restored"));
            await load();
            await reloadSettings();
        });
    },
    "set-diagnostic": async () => diagnosticModal(await api("GET", "/api/diagnostic")),
};

// save writes one group of settings: the field changed, the rest as it was.
async function save(group, field, value) {
    const next = { ...S.settings[group], [field]: value };
    if (group === "admin") await api("POST", "/api/admin", next);
    else await api("POST", "/api/settings", { [group]: next });
    S.settings[group] = next;
    toast(t("settings.saved"));
    await load();
}

// onSettingChange runs for a control of the Settings screen (data-set="group.field").
export async function onSettingChange(el) {
    const [group, field] = el.dataset.set.split(".");
    const value = el.type === "checkbox" ? el.checked : el.dataset.set === "notify.hour" ? Number(el.value) : el.value;
    if (group === "lang") {
        await api("POST", "/api/locale", { lang: value });
        return location.reload();
    }
    if (group === "app") return toApp({ type: "loginItem", on: value });
    await save(group, field, value);
}
