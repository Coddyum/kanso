// The pieces every screen is built with: logos, headers, key numbers, date badges, tags.

import { esc } from "../dom.js";
import { MONTHS_SHORT, parseISO } from "../format.js";
import { icon } from "../icons.js";
import { STATUS_TAG } from "../words.js";

// The colours of a service's initials: the Tailwind palette in 400, like the tiles.
const PALETTE = ["#fb7185", "#38bdf8", "#a78bfa", "#34d399", "#fbbf24", "#818cf8", "#2dd4bf", "#f472b6", "#fb923c", "#a3e635", "#e879f9", "#60a5fa"];
const hashStr = (s) => [...s].reduce((h, c) => (h * 31 + c.codePointAt(0)) | 0, 7);
const colorOf = (name) => PALETTE[Math.abs(hashStr((name || "?").trim().toLowerCase())) % PALETTE.length];

// logo is a service's initials on its colour, under its icon when the server finds one.
export function logo(name, domain, size = "") {
    const n = (name || "?").trim();
    const parts = n.split(/[\s\-_]+/).filter((w) => /^[\p{L}\p{N}]/u.test(w));
    const initials = (parts.length > 1 ? parts[0][0] + parts[1][0] : n[0] || "?").toUpperCase();
    const img = domain ? `<img src="/api/logo?d=${encodeURIComponent(domain)}" alt="" loading="lazy" onerror="this.remove()">` : "";
    return `<span class="logo ${size}" style="--c:${colorOf(n)}">${esc(initials)}${img}</span>`;
}

// pageHead is the top of a screen: a title, one line, the tools on the right.
export const pageHead = ({ title, sub = "", tools = "" }) => `
    <div class="head">
        <div class="ht"><h1>${title}</h1>${sub ? `<p>${sub}</p>` : ""}</div>
        <span class="grow"></span>
        ${tools ? `<div class="toolbar">${tools}</div>` : ""}
    </div>`;

// kpi is a key number: what it is, the number, one line under it; a link or an action when given one.
export const kpi = ({ label, value, sub = "", href = "", action = "", bad = false }) => {
    const attrs = href ? `href="${href}"` : action ? `data-action="${action}"` : "";
    const tag = href || action ? "a" : "div";
    return `<${tag} class="kpi" ${attrs}><span class="k-label">${label}</span><span class="k-val${bad ? " neg" : ""}">${value}</span>${sub ? `<span class="k-sub">${sub}</span>` : ""}</${tag}>`;
};

export const dateBadge = (date) => {
    const d = parseISO(date);
    return `<span class="datebadge"><small>${MONTHS_SHORT[d.getMonth()].replace(".", "")}</small><b>${d.getDate()}</b></span>`;
};

export const tagOf = (status) => `<span class="tag s-${status}">${STATUS_TAG[status] || status}</span>`;

// dlIcon is the flag of an administrative deadline, in place of a logo.
export const dlIcon = (size = "sm") => `<span class="logo ${size} dl">${icon("flag", size === "sm" ? 12 : 14)}</span>`;

export const currencyOptions = (cur) =>
    ["EUR", "USD", "GBP"].map((c) => `<option ${c === (cur || "EUR") ? "selected" : ""}>${c}</option>`).join("");
