// Drag and drop: files from the Finder onto a missing invoice, a folder or the
// folder view; and a file or folder of the list onto another folder.

import { api } from "./api.js";
import { load, upload } from "./app.js";
import { $, $$ } from "./dom.js";
import { t } from "./i18n.js";
import { droppedEntries, intakeEntries } from "./modals/intake.js";
import { rootName, S } from "./state.js";
import { toast } from "./ui/toast.js";

const INTERNAL = "text/x-inc-path";
const hasFiles = (e) => [...(e.dataTransfer?.types || [])].includes("Files");
const isInternal = (e) => [...(e.dataTransfer?.types || [])].includes(INTERNAL);

// droppedFiles keeps the files of a drop; a dropped folder is skipped.
function droppedFiles(e) {
    const out = [];
    for (const item of e.dataTransfer.items || []) {
        if (item.kind !== "file") continue;
        const entry = item.webkitGetAsEntry?.();
        if (entry && !entry.isFile) continue;
        const f = item.getAsFile();
        if (f) out.push(f);
    }
    return out;
}

function clearDrag() {
    $$(".drag, .drop-target").forEach((el) => el.classList.remove("drag", "drop-target"));
    $("#main").classList.remove("dragging");
}

function onDragOver(e) {
    e.preventDefault();
    if (!hasFiles(e) && !isInternal(e)) return;
    const card = hasFiles(e) && e.target.closest(".mcard[data-drop]");
    const dir = e.target.closest("[data-dropdir]");
    const zone = hasFiles(e) && e.target.closest("[data-intake-zone]");
    if (zone) {
        clearDrag();
        return zone.classList.add("drag");
    }
    const target = card || dir;
    if (target && target.classList.contains(card ? "drag" : "drop-target")) return;
    clearDrag();
    if (card) card.classList.add("drag");
    else if (dir) dir.classList.add("drop-target");
    else if (S.view === "files" && hasFiles(e)) $("#main").classList.add("dragging");
}

async function onDrop(e) {
    e.preventDefault();
    // A dropped folder, wherever it lands, or anything dropped on a « drop your folders here » zone, is sorted before it is copied.
    const entries = droppedEntries(e);
    if (entries.some((x) => x.isDirectory) || (entries.length && e.target.closest("[data-intake-zone]"))) {
        clearDrag();
        return intakeEntries(entries).catch((err) => toast(err.message, true));
    }
    const files = droppedFiles(e);
    const internal = e.dataTransfer.getData(INTERNAL);
    const card = e.target.closest(".mcard[data-drop]");
    const dir = e.target.closest("[data-dropdir]");
    clearDrag();
    try {
        if (card && files.length) {
            await upload(files, { vendor: card.dataset.vendor, date: card.dataset.date, key: card.dataset.key });
        } else if (dir && internal) {
            if (internal === dir.dataset.dropdir) return;
            await api("POST", "/api/move", { path: internal, toDir: dir.dataset.dropdir });
            toast(t("toast.moved", { name: dir.dataset.dropdir || rootName() }));
            await load();
        } else if (dir && files.length) {
            await upload(files, { dir: dir.dataset.dropdir });
        } else if (S.view === "files" && files.length) {
            await upload(files, { dir: S.dir });
        } else if (files.length) {
            toast(t("toast.dropWhere"), true);
        }
    } catch (err) {
        toast(err.message, true);
    }
}

export function initDragDrop() {
    document.addEventListener("dragstart", (e) => {
        const row = e.target.closest?.("[data-drag]");
        if (!row) return;
        e.dataTransfer.setData(INTERNAL, row.dataset.drag);
        e.dataTransfer.effectAllowed = "move";
    });
    document.addEventListener("dragover", onDragOver);
    document.addEventListener("dragleave", (e) => {
        if (!e.relatedTarget) clearDrag();
    });
    document.addEventListener("dragend", clearDrag);
    document.addEventListener("drop", onDrop);
}
