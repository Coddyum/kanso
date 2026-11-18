// Small DOM helpers.

export const $ = (sel, el = document) => el.querySelector(sel);
export const $$ = (sel, el = document) => [...el.querySelectorAll(sel)];

const ESC = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

// esc makes any value safe inside HTML text or a quoted attribute.
export const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ESC[c]);
