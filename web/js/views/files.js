// Dossier: the synced folder as it is, with its folders, files and what was read in them.

import { esc } from "../dom.js";
import { fileURL, fmtMod, fmtSize } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { rootName, S } from "../state.js";
import { pageHead } from "../ui/parts.js";
import { kindName } from "../words.js";

const rowActs = (e) => `
    <span class="r-act">
        ${!e.dir && e.kind === "document" ? `<button class="btn sm ghost icon" data-action="mark-invoice" data-hash="${esc(e.hash)}" title="${t("files.markInvoice")}">${icon("receipt", 14)}</button>` : ""}
        <button class="btn sm ghost icon" data-action="rename" data-path="${esc(e.path)}" data-name="${esc(e.name)}" title="${t("files.rename")}">${icon("edit", 14)}</button>
        ${e.dir ? "" : `<button class="btn sm ghost icon" data-action="open-native" data-path="${esc(e.path)}" title="${t("files.openNative")}">${icon("external", 14)}</button>`}
        <button class="btn sm ghost icon" data-action="reveal" data-path="${esc(e.path)}" title="${t("common.showInFinder")}">${icon("finder", 14)}</button>
    </span>`;

const fileRow = (e) =>
    e.dir
        ? `<div class="frow is-dir" data-action="cd" data-path="${esc(e.path)}" data-dropdir="${esc(e.path)}" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("folder", 18)}</span><span class="fname">${esc(e.name)}</span><span class="fkind"></span>
            <span class="fmeta">${t("files.items", { count: e.items })}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`
        : `<div class="frow" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("file", 18)}</span>
            <span class="fname"><a href="${fileURL(e.path)}" target="_blank" rel="noopener">${esc(e.name)}</a></span>
            <span class="fkind">${e.kind && e.kind !== "document" ? `<span class="pill k-${e.kind}">${kindName(e.kind)}</span>` : ""}</span>
            <span class="fmeta">${fmtSize(e.size)}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`;

export function viewFiles() {
    const L = S.listing;
    const parts = L.dir ? L.dir.split("/") : [];
    const crumbs = [
        `<a data-action="cd" data-path="" data-dropdir="">${esc(rootName())}</a>`,
        ...parts.map((p, i) => {
            const path = parts.slice(0, i + 1).join("/");
            return `<span class="sep">/</span><a data-action="cd" data-path="${esc(path)}" data-dropdir="${esc(path)}">${esc(p)}</a>`;
        }),
    ].join("");
    return `
    ${pageHead({
        title: esc(t("files.title", { name: rootName() })),
        sub: t("files.sub"),
        tools: `<button class="btn" data-action="mkdir">${icon("folderPlus", 15)}${t("files.newFolder")}</button>
            <label class="btn">${icon("upload", 15)}${t("files.import")}<input type="file" hidden multiple data-upload="dir"></label>
            <button class="btn icon" data-action="reveal" data-path="${esc(L.dir)}" title="${t("files.openInFinder")}">${icon("finder", 15)}</button>`,
    })}
    <div class="crumbs">${crumbs}</div>
    <div class="card files">${L.entries.length ? L.entries.map(fileRow).join("") : `<div class="files-empty">${t("files.empty")}</div>`}</div>
    <div class="dropzone-hint">${icon("upload", 16)}${t("files.dropHint")}</div>`;
}
