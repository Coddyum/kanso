// The modal sheet: one form at a time. onSubmit receives the form's fields;
// the sheet closes when it succeeds, and shows the error when it fails.

import { $, esc } from "../dom.js";
import { toast } from "./toast.js";

export function openModal(html, onSubmit) {
    const m = $("#modal");
    m.innerHTML = `<div class="sheet" role="dialog" aria-modal="true">${html}</div>`;
    m.hidden = false;
    const form = $("form", m);
    form.addEventListener("submit", async (e) => {
        e.preventDefault();
        const btn = $("button.primary", form);
        btn.disabled = true;
        try {
            await onSubmit(Object.fromEntries(new FormData(form)));
            closeModal();
        } catch (err) {
            toast(err.message, true);
            btn.disabled = false;
        }
    });
    $("[autofocus]", m)?.focus();
}

export function closeModal() {
    const m = $("#modal");
    m.hidden = true;
    m.innerHTML = "";
}

export const modalOpen = () => !$("#modal").hidden;

// promptModal asks for one value; onOk receives it trimmed.
export function promptModal(title, label, value, onOk) {
    openModal(
        `<form class="form">
        <h2>${esc(title)}</h2>
        <div class="grid2"><label class="f span2">${esc(label)}<input name="value" value="${esc(value)}" required autofocus></label></div>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="close">Annuler</button><button class="btn primary">OK</button></footer>
    </form>`,
        (data) => onOk(data.value.trim()),
    );
}
