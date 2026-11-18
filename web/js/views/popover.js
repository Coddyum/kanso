// The bubble over a calendar day: its payments, deadlines and the reserve left that evening.

import { esc } from "../dom.js";
import { approx, capitalize, fmtTotals, longDay, money, totals } from "../format.js";
import { t } from "../i18n.js";
import { balanceOn, findInvoice, S } from "../state.js";
import { dlIcon, logo, tagOf } from "../ui/parts.js";
import { cycleShort, DL_CLASS, deadlineState, kindName } from "../words.js";

const pop = document.createElement("div");
pop.className = "pop";
pop.hidden = true;
let popDay = null;
let popTimer;
let inPop = false;

function popItem(e) {
    if (e.type === "deadline") {
        return `<div class="pop-item" data-action="deadlines">${dlIcon("")}
            <div class="pm"><b>${esc(e.name)}</b><span class="wrap">${esc(e.detail)}</span></div>
            <div class="pop-r"><span class="tag ${DL_CLASS[e.status]}">${deadlineState(e.status)}</span></div>
        </div>`;
    }
    const inv = e.invoiceId ? findInvoice(e.invoiceId) : null;
    const p = e.paymentId ? S.st.payments.find((x) => x.id === e.paymentId) : null;
    const files = inv ? inv.files.map((f) => kindName(f.kind)).join(" + ") : "";
    const lines = [
        [p && cycleShort(p.cycle), inv?.label].filter(Boolean).join(" · "),
        inv ? [files, inv.number && t("invoices.number", { n: inv.number })].filter(Boolean).join(" · ") : "",
        e.estimate && p ? `${t("popover.estimated")} · ${p.recent.map((a) => money(a, p.currentCurrency)).join(" · ")}` : "",
        e.status === "missing" ? t("popover.dropIn") : "",
    ].filter(Boolean);
    const action = e.type === "payment" ? `data-action="edit-payment" data-id="${esc(e.paymentId)}"` : `data-action="open-invoice" data-id="${esc(e.invoiceId)}"`;
    return `<div class="pop-item" ${action}>${logo(e.name, e.domain)}
        <div class="pm"><b>${esc(e.name)}</b>${lines.map((l) => `<span>${esc(l)}</span>`).join("")}</div>
        <div class="pop-r"><b>${approx(e)}${money(e.amount, e.currency)}</b>${tagOf(e.status)}</div>
    </div>`;
}

function showPop(day) {
    const date = day.dataset.date;
    const evs = S.events.filter((e) => e.date === date);
    const balance = balanceOn(date);
    if (!evs.length && balance === null) return hidePop();
    popDay = day;
    const title = capitalize(longDay(date));
    const total = evs.some((e) => e.amount != null) ? fmtTotals(totals(evs.filter((e) => e.amount != null && e.status !== "skipped"))) : "";
    const foot = balance === null ? "" : `<footer class="${balance < 0 ? "neg" : ""}">${t("popover.reserveThatEvening")}<b>${money(balance, "EUR")}</b></footer>`;
    pop.innerHTML = `<header><b>${title}</b><span>${total}</span></header>${evs.map(popItem).join("")}${foot}`;
    pop.hidden = false;
    const r = day.getBoundingClientRect();
    const w = pop.offsetWidth;
    const h = pop.offsetHeight;
    const left = Math.min(Math.max(8, r.left + r.width / 2 - w / 2), innerWidth - w - 8);
    const above = r.top - h - 8;
    pop.classList.toggle("below", above < 8);
    pop.style.left = `${left}px`;
    pop.style.top = `${above >= 8 ? above : Math.min(r.bottom + 8, innerHeight - h - 8)}px`;
    pop.style.setProperty("--arrow", `${r.left + r.width / 2 - left}px`);
}

export function hidePop() {
    clearTimeout(popTimer);
    inPop = false;
    popDay = null;
    pop.hidden = true;
}

// initPopover: the bubble stays open while the mouse is on it and closes the
// moment it leaves. The only wait: 90 ms when the mouse slips from its day onto
// a neighbouring one, time enough to reach the bubble across the corner.
export function initPopover() {
    document.body.append(pop);
    document.addEventListener("mouseover", (e) => {
        clearTimeout(popTimer);
        if (e.target.closest?.(".pop")) {
            inPop = true;
            return;
        }
        const fromPop = inPop;
        inPop = false;
        const day = e.target.closest?.(".day");
        if (day === popDay) return;
        if (!day) return hidePop();
        if (!popDay || fromPop) return showPop(day);
        popTimer = setTimeout(() => showPop(day), 90);
    });
    document.documentElement.addEventListener("mouseleave", hidePop);
    document.addEventListener("mousedown", (e) => {
        if (!e.target.closest?.(".pop")) hidePop();
    });
}
