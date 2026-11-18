// Dossier: the synced folder as it is, with its folders, files and what was read in them.

import { esc } from "../dom.js";
import { fileURL, fmtMod, fmtSize } from "../format.js";
import { icon } from "../icons.js";
import { S } from "../state.js";
import { pageHead } from "../ui/parts.js";
import { KIND } from "../words.js";

const rowActs = (e) => `
    <span class="r-act">
        ${!e.dir && e.kind === "document" ? `<button class="btn sm ghost icon" data-action="mark-invoice" data-hash="${esc(e.hash)}" title="C'est une facture">${icon("receipt", 14)}</button>` : ""}
        <button class="btn sm ghost icon" data-action="rename" data-path="${esc(e.path)}" data-name="${esc(e.name)}" title="Renommer">${icon("edit", 14)}</button>
        ${e.dir ? "" : `<button class="btn sm ghost icon" data-action="open-native" data-path="${esc(e.path)}" title="Ouvrir avec l'app du Mac">${icon("external", 14)}</button>`}
        <button class="btn sm ghost icon" data-action="reveal" data-path="${esc(e.path)}" title="Afficher dans le Finder">${icon("finder", 14)}</button>
    </span>`;

const fileRow = (e) =>
    e.dir
        ? `<div class="frow is-dir" data-action="cd" data-path="${esc(e.path)}" data-dropdir="${esc(e.path)}" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("folder", 18)}</span><span class="fname">${esc(e.name)}</span><span class="fkind"></span>
            <span class="fmeta">${e.items} élément${e.items > 1 ? "s" : ""}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`
        : `<div class="frow" draggable="true" data-drag="${esc(e.path)}">
            <span class="fi">${icon("file", 18)}</span>
            <span class="fname"><a href="${fileURL(e.path)}" target="_blank" rel="noopener">${esc(e.name)}</a></span>
            <span class="fkind">${e.kind && e.kind !== "document" ? `<span class="pill k-${e.kind}">${KIND[e.kind]}</span>` : ""}</span>
            <span class="fmeta">${fmtSize(e.size)}</span><span class="fmeta">${fmtMod(e.mod)}</span>${rowActs(e)}
        </div>`;

export function viewFiles() {
    const L = S.listing;
    const parts = L.dir ? L.dir.split("/") : [];
    const crumbs = [
        `<a data-action="cd" data-path="" data-dropdir="">Inc</a>`,
        ...parts.map((p, i) => {
            const path = parts.slice(0, i + 1).join("/");
            return `<span class="sep">/</span><a data-action="cd" data-path="${esc(path)}" data-dropdir="${esc(path)}">${esc(p)}</a>`;
        }),
    ].join("");
    return `
    ${pageHead({
        title: "Dossier Inc",
        sub: "Le même dossier que dans le Finder : ce que tu ajoutes ici y apparaît, et inversement.",
        tools: `<button class="btn" data-action="mkdir">${icon("folderPlus", 15)}Nouveau dossier</button>
            <label class="btn">${icon("upload", 15)}Importer<input type="file" hidden multiple data-upload="dir"></label>
            <button class="btn icon" data-action="reveal" data-path="${esc(L.dir)}" title="Ouvrir dans le Finder">${icon("finder", 15)}</button>`,
    })}
    <div class="crumbs">${crumbs}</div>
    <div class="card files">${L.entries.length ? L.entries.map(fileRow).join("") : '<div class="files-empty">Dossier vide : glisse des fichiers ici.</div>'}</div>
    <div class="dropzone-hint">${icon("upload", 16)}Glisse des fichiers du Finder ici pour les ajouter, ou sur un dossier de la liste pour les y ranger.</div>`;
}
