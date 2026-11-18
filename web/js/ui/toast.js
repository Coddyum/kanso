// toast shows a short message at the bottom of the screen; an error stays longer.

import { $ } from "../dom.js";

let timer;

export function toast(msg, isError = false) {
    const t = $("#toast");
    t.textContent = msg;
    t.className = "toast" + (isError ? " err" : "");
    t.hidden = false;
    clearTimeout(timer);
    timer = setTimeout(() => (t.hidden = true), isError ? 5000 : 2600);
}
