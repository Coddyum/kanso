// What the first-run checklist does: open a step, tick or skip it, save the
// reserve, change the language, put the list away or open it again.

import { api } from "./api.js";
import { load, render } from "./app.js";
import { $ } from "./dom.js";
import { t } from "./i18n.js";
import { S } from "./state.js";
import { toast } from "./ui/toast.js";

// setStep saves a step's state; the next step still to do opens.
export async function setStep(step, state) {
    await api("POST", "/api/onboarding", { step, state });
    S.ob.open = "";
    await load();
}

export const WELCOME_ACTIONS = {
    "ob-open": (el) => {
        S.ob.open = S.ob.open === el.dataset.step ? "-" : el.dataset.step;
        render();
    },
    "ob-step": (el) => setStep(el.dataset.step, el.dataset.state),
    "ob-reserve": async () => {
        const field = $("#ob-amount");
        if (field.value === "" || Number.isNaN(Number(field.value))) return toast(t("welcome.steps.reserve.missing"), true);
        await api("POST", "/api/reserve", { amount: Number(field.value), date: $("#ob-date").value });
        await setStep("reserve", "done");
    },
    "ob-close": async () => {
        await api("POST", "/api/onboarding", { show: false });
        location.hash = "calendar";
    },
    "ob-reopen": async () => {
        await api("POST", "/api/onboarding", { show: true });
        S.ob.open = "";
        location.hash = "welcome";
    },
};

// onWelcomeChange: the language picked in the first step; the page reloads in it.
export async function onWelcomeChange(el) {
    await api("POST", "/api/locale", { lang: el.value });
    await api("POST", "/api/onboarding", { step: "lang", state: "done" });
    location.reload();
}
