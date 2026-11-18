// What a click, a typed search, a picked option or a sent form does. Every
// button carries data-action; one listener per kind of event runs it.

import { api } from "./api.js";
import { load, loadCalendar, render, upload } from "./app.js";
import { $ } from "./dom.js";
import { encPath, fileURL, firstOfMonth } from "./format.js";
import { t } from "./i18n.js";
import { paymentModal } from "./modals/payment.js";
import { dayModal, deadlinesModal, invoiceModal, projectModal, reserveModal } from "./modals/others.js";
import { findInvoice, projectById, rootName, S } from "./state.js";
import { onSettingChange, SETTINGS_ACTIONS } from "./settings-actions.js";
import { closeModal, modalOpen, promptModal } from "./ui/modal.js";
import { toast } from "./ui/toast.js";
import { invoiceList } from "./views/invoices.js";
import { hidePop } from "./views/popover.js";

const dismiss = (key) => api("POST", "/api/dismiss", { key, on: true });

const ACTIONS = {
    month: async (el) => {
        const delta = Number(el.dataset.delta);
        S.month = delta === 0 ? firstOfMonth(new Date()) : new Date(S.month.getFullYear(), S.month.getMonth() + delta, 1);
        await loadCalendar();
        render();
    },
    "new-payment": (el) => paymentModal({ start: el.dataset.date }),
    "edit-payment": (el) => paymentModal(S.st.payments.find((p) => p.id === el.dataset.id)),
    "open-invoice": (el) => {
        const inv = findInvoice(el.dataset.id);
        if (inv) window.open(fileURL(inv.files[0].path), "_blank", "noopener");
    },
    "edit-invoice": (el) => invoiceModal(findInvoice(el.dataset.id)),
    "day-list": (el) => dayModal(el.dataset.date),
    aside: (el) => {
        S.asideTab = el.dataset.tab;
        render();
    },
    reserve: () => reserveModal(),
    reveal: (el) => api("POST", "/api/reveal", { path: el.dataset.path }),
    "open-native": (el) => api("POST", "/api/open", { path: el.dataset.path }),
    "accept-sugg": (el) => {
        const s = S.st.suggestions.find((x) => x.key === el.dataset.key);
        const { name, vendor, match, domain, amount, currency, cycle, start } = s;
        paymentModal({ name, vendor, match, domain, amount, currency, cycle, start, expectInvoice: true }, s);
    },
    "dismiss-sugg": async (el) => {
        await dismiss(el.dataset.key);
        await load();
    },
    "dismiss-due": async (el) => {
        await dismiss(el.dataset.key);
        toast(t("toast.dismissed"));
        await load();
    },
    "toggle-todo": async (el) => {
        const t = S.st.todos.find((x) => x.id === el.dataset.id);
        await api("POST", "/api/todos", { ...t, done: !t.done });
        await load();
    },
    "delete-todo": async (el) => {
        await api("DELETE", "/api/todos/" + el.dataset.id);
        await load();
    },
    cd: (el) => {
        location.hash = "files" + (el.dataset.path ? "/" + encPath(el.dataset.path) : "");
    },
    mkdir: () =>
        promptModal(t("files.newFolder"), t("files.inFolder", { name: S.dir || rootName() }), "", async (name) => {
            await api("POST", "/api/mkdir", { path: S.dir, name });
            await load();
        }),
    rename: (el) =>
        promptModal(t("files.rename"), t("files.newName"), el.dataset.name, async (name) => {
            await api("POST", "/api/rename", { path: el.dataset.path, name });
            await load();
        }),
    "mark-invoice": async (el) => {
        await api("POST", "/api/override", { id: el.dataset.hash, override: { kind: "invoice" } });
        toast(t("toast.markedInvoice"));
        await load();
    },
    close: () => closeModal(),
    "clear-reserve": async () => {
        await api("POST", "/api/reserve", { amount: null });
        closeModal();
        await load();
    },
    "delete-payment": async (el) => {
        await api("DELETE", "/api/payments/" + el.dataset.id);
        closeModal();
        toast(t("toast.paymentDeleted"));
        await load();
    },
    deadlines: () => deadlinesModal(),
    "toggle-deadline": async (el) => {
        const scroll = $(".dl-list")?.scrollTop || 0;
        await api("POST", "/api/deadline", { key: el.dataset.key, done: el.dataset.done === "1" });
        await load();
        deadlinesModal();
        if ($(".dl-list")) $(".dl-list").scrollTop = scroll;
    },
    "an-period": (el) => {
        S.an.period = el.dataset.p;
        S.an.month = "";
        render();
    },
    "an-month": (el) => {
        S.an.month = S.an.month === el.dataset.key ? "" : el.dataset.key;
        render();
    },
    "an-project": (el) => {
        const id = el.dataset.id;
        S.an.project = id === S.an.project ? "all" : id;
        render();
    },
    "an-tab": (el) => {
        S.an.tab = el.dataset.tab;
        render();
    },
    "new-project": () => projectModal(),
    "edit-project": (el) => projectModal(projectById(el.dataset.id)),
    "quick-project": async (el) => {
        await api("POST", "/api/projects", { name: el.dataset.name });
        toast(t("toast.projectCreated", { name: el.dataset.name }));
        await load();
    },
    "delete-project": async (el) => {
        await api("DELETE", "/api/projects/" + el.dataset.id);
        if (S.an.project === el.dataset.id) S.an.project = "all";
        closeModal();
        toast(t("toast.projectDeleted"));
        await load();
    },
    "reset-invoice": async (el) => {
        await api("POST", "/api/override", { id: el.dataset.id, override: null });
        closeModal();
        await load();
    },
    ...SETTINGS_ACTIONS,
};

async function onClick(e) {
    const el = e.target.closest("[data-action]");
    if (!el || !ACTIONS[el.dataset.action]) return;
    if (el.closest(".pop")) hidePop();
    if (el.tagName === "A" && el.getAttribute("href")) return;
    e.preventDefault();
    try {
        await ACTIONS[el.dataset.action](el);
    } catch (err) {
        toast(err.message, true);
    }
}

// onChange: a project picked for a service or an invoice, a filter of the invoices, files picked to send.
async function onChange(e) {
    const t = e.target;
    try {
        if (t.dataset.set) {
            await onSettingChange(t);
        } else if (t.dataset.assign) {
            await api("POST", "/api/assign", { kind: t.dataset.assign, ref: t.dataset.ref, project: t.value });
            await load();
        } else if (t.id === "inv-year" || t.id === "inv-vendor") {
            S[t.id === "inv-year" ? "year" : "vendor"] = t.value;
            $("#inv-list").innerHTML = invoiceList();
        } else if (t.dataset.upload === "missing") {
            const card = t.closest(".mcard");
            await upload([...t.files], { vendor: card.dataset.vendor, date: card.dataset.date, key: card.dataset.key });
        } else if (t.dataset.upload === "dir") {
            await upload([...t.files], { dir: S.dir });
        }
    } catch (err) {
        toast(err.message, true);
    }
}

// onSubmit: the reminder form of « À récupérer »; the modal forms handle their own.
async function onSubmit(e) {
    const form = e.target;
    if (form.dataset.form !== "todo") return;
    e.preventDefault();
    const text = form.elements.text.value.trim();
    if (!text) return;
    try {
        await api("POST", "/api/todos", { text });
        await load();
        $('[data-form="todo"] input')?.focus();
    } catch (err) {
        toast(err.message, true);
    }
}

export function initActions() {
    document.addEventListener("click", onClick);
    document.addEventListener("input", (e) => {
        if (e.target.id !== "inv-q") return;
        S.q = e.target.value;
        $("#inv-list").innerHTML = invoiceList();
    });
    document.addEventListener("change", onChange);
    document.addEventListener("submit", onSubmit);
    $("#modal").addEventListener("mousedown", (e) => {
        if (e.target.id === "modal") closeModal();
    });
    document.addEventListener("keydown", (e) => {
        if (e.key === "Escape" && modalOpen()) closeModal();
    });
}
