// Dropping whole folders of invoices: their files are sent to the local
// server, which copies them aside and says where each would go. The user
// checks the plan, then the files are copied into the folder — the originals
// never move — and the copy can be undone.

import { api, sendForm } from "../api.js";
import { load } from "../app.js";
import { esc } from "../dom.js";
import { fmtMod, groupBy } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { closeModal, openModal } from "../ui/modal.js";
import { toast } from "../ui/toast.js";

// entriesOf lists a dropped folder; the browser hands its content in batches.
async function entriesOf(dir) {
    const reader = dir.createReader();
    const all = [];
    for (;;) {
        const batch = await new Promise((ok, ko) => reader.readEntries(ok, ko));
        if (!batch.length) return all;
        all.push(...batch);
    }
}

async function walk(entry, out) {
    if (entry.isFile) {
        const file = await new Promise((ok, ko) => entry.file(ok, ko));
        out.push({ file, path: entry.fullPath.replace(/^\//, "") });
        return;
    }
    for (const child of await entriesOf(entry)) await walk(child, out);
}

// droppedEntries must be read during the drop itself: the browser forgets them after.
export const droppedEntries = (e) =>
    [...(e.dataTransfer.items || [])]
        .filter((i) => i.kind === "file")
        .map((i) => i.webkitGetAsEntry?.())
        .filter(Boolean);

// intakeEntries sends what was dropped: folders with everything inside, or loose files.
export async function intakeEntries(entries) {
    const list = [];
    for (const e of entries) await walk(e, list);
    await send(list);
}

// intakeInput sends the folder picked in « Choose a folder… ».
export async function intakeInput(input) {
    const list = [...input.files].map((file) => ({ file, path: file.webkitRelativePath || file.name }));
    input.value = "";
    await send(list);
}

async function send(list) {
    if (!list.length) return;
    openModal(`<form class="form"><h2>${icon("folderPlus", 18)}${t("intake.title")}</h2><p class="sub">${t("intake.reading", { count: list.length })}</p></form>`, () => {});
    const fd = new FormData();
    for (const { file, path } of list) {
        fd.append("path", path);
        fd.append("file", file, file.name);
    }
    try {
        planScreen(await sendForm("/api/intake", fd));
    } catch (err) {
        openModal(`<form class="form"><h2>${t("intake.title")}</h2><p class="sub">${esc(err.message)}</p><footer><span class="grow"></span><button type="button" class="btn primary" data-action="close">${t("common.close")}</button></footer></form>`, () => {});
    }
}

const count = (n) => t("intake.files", { count: n });

// sortedTree draws YEAR › Vendor › months, with the number of files of each.
function sortedTree(items) {
    const byYear = groupBy(items, (it) => it.target.split("/")[0]);
    return Object.keys(byYear)
        .sort()
        .reverse()
        .map((year) => {
            const byVendor = groupBy(byYear[year], (it) => it.target.split("/")[1]);
            const vendors = Object.keys(byVendor)
                .sort()
                .map((v) => {
                    const months = groupBy(byVendor[v], (it) => it.target.split("/")[2]);
                    // In calendar order: a month folder's files share its month.
                    const list = Object.keys(months)
                        .sort((a, b) => months[a][0].date.localeCompare(months[b][0].date))
                        .map((m) => `${esc(m)} <small>${months[m].length}</small>`)
                        .join(" · ");
                    return `<li><b>${esc(v)}</b><span>${list}</span></li>`;
                })
                .join("");
            return `<div class="in-year"><h4>${esc(year)} <small>${count(byYear[year].length)}</small></h4><ul>${vendors}</ul></div>`;
        })
        .join("");
}

function keptList(items) {
    const byDir = groupBy(items, (it) => it.target.split("/").slice(0, -1).join("/") || "/");
    return `<ul class="in-kept">${Object.keys(byDir)
        .sort()
        .map((d) => `<li><code>${esc(d)}</code><small>${count(byDir[d].length)}</small></li>`)
        .join("")}</ul>`;
}

function dupList(items) {
    const rows = items.map((it) => `<li>${t("intake.dupOf", { name: esc(it.path), path: esc(it.duplicate) })}</li>`).join("");
    return `<details class="in-dups"><summary>${t("intake.dups")} <small>${count(items.length)}</small></summary><ul>${rows}</ul></details>`;
}

// planScreen shows where each file would go; nothing is in the folder before « Copy ».
function planScreen(b) {
    const sorted = b.items.filter((it) => it.target && it.sorted);
    const kept = b.items.filter((it) => it.target && !it.sorted);
    const dups = b.items.filter((it) => !it.target);
    const n = sorted.length + kept.length;
    const body = [
        sorted.length ? `<h3 class="bank-h">${t("intake.sorted")}</h3><div class="in-tree">${sortedTree(sorted)}</div>` : "",
        kept.length ? `<h3 class="bank-h">${t("intake.kept")}</h3>${keptList(kept)}` : "",
        dups.length ? dupList(dups) : "",
        n ? "" : `<p class="set-note">${t("intake.nothing")}</p>`,
    ].join("");
    openModal(
        `<form class="form wide">
        <h2>${icon("folderPlus", 18)}${t("intake.title")}</h2>
        <p class="sub">${t("intake.summary", { count: b.items.length })}</p>
        <div class="in-plan">${body}</div>
        <p class="note">${icon("lock", 13)} ${t("intake.private")}</p>
        <footer><span class="grow"></span><button type="button" class="btn ghost" data-action="intake-cancel" data-id="${b.id}">${t("common.cancel")}</button>
        ${n ? `<button class="btn primary">${t("intake.copy", { count: n })}</button>` : ""}</footer>
    </form>`,
        async () => {
            const res = await api("POST", `/api/intake/${b.id}/copy`);
            await load();
            // The plan's window closes once this returns: the next one opens just after.
            setTimeout(() => doneScreen(res));
        },
    );
}

// doneScreen says what was copied, with the undo at hand.
function doneScreen(res) {
    openModal(
        `<form class="form">
        <h2>${icon("check", 18)}${t("intake.title")}</h2>
        <p class="sub">${t("intake.done", { count: res.written })}</p>
        <footer><button type="button" class="btn ghost" data-action="intake-undo" data-id="${res.id}">${t("intake.undo")}</button>
        <span class="grow"></span><button class="btn primary">${t("common.close")}</button></footer>
    </form>`,
        () => {},
    );
}

async function undo(id) {
    const res = await api("POST", `/api/intake/${id}/undo`);
    toast(t("intake.undone", { count: res.removed }) + (res.kept ? " · " + t("intake.undoKept", { count: res.kept }) : ""));
    await load();
}

// lastIntakeLabel names the latest copy that can still be undone: « 12 files, 19 Nov 2026 ».
export const lastIntakeLabel = (last) => t("intake.last", { count: last.written, date: fmtMod(last.copied) });

// A click on « invoice found » opens the app on that file's plan.
window.kansoIntake = (id) =>
    api("GET", `/api/intake/${encodeURIComponent(id)}`)
        .then(planScreen)
        .catch((err) => toast(err.message, true));

export const INTAKE_ACTIONS = {
    "intake-cancel": async (el) => {
        closeModal();
        await api("DELETE", `/api/intake/${el.dataset.id}`);
    },
    "intake-undo": async (el) => {
        closeModal();
        await undo(el.dataset.id);
    },
};
