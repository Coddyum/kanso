// The words the app shows for the server's codes: cycles, kinds of document,
// states. The codes and numbers live here, the words in web/locales.

import { t } from "./i18n.js";

export const CYCLE_CODES = ["once", "weekly", "monthly", "quarterly", "yearly"];
export const cycleLong = (c) => t(`cycle.long.${c}`);
export const cycleShort = (c) => t(`cycle.short.${c}`);
// cycleWord says what kind of expense a cycle makes: « Abonnement mensuel », « Facture hors abonnement ».
export const cycleWord = (c) => t(`cycle.word.${c || "none"}`);
// How much of a payment falls in one month.
export const PER_MONTH = { weekly: 52 / 12, monthly: 1, quarterly: 1 / 3, yearly: 1 / 12, once: 0 };

export const PROJECT_TONES = ["violet", "sky", "emerald", "amber", "rose", "teal", "orange", "indigo", "pink", "lime", "fuchsia", "blue"];

const KINDS = ["invoice", "receipt", "order", "statement", "document"];
export const kindName = (k) => t(`kind.${KINDS.includes(k) ? k : "file"}`);
// What an amount of the analytics is: an invoice, an estimate, a planned payment.
export const amountKind = (k) => t(`amountKind.${k}`);

// The state of a due date, in a sentence and as a tag: its colour comes from its s-* class.
const STATUSES = ["upcoming", "today", "matched", "missing", "skipped", "paid", "invoice"];
export const statusSentence = (s) => t(`status.sentence.${s}`);
export const statusTag = (s) => (STATUSES.includes(s) ? t(`status.tag.${s}`) : "");

// A deadline's state: its colour class and its word.
export const DL_CLASS = { todo: "s-deadline", done: "s-matched", past: "s-paid" };
export const deadlineState = (s) => t(`deadlineState.${s}`);
