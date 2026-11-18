// The words the app shows for the server's codes: cycles, kinds of document, states.

export const CYCLES = {
    once: "Une seule fois",
    weekly: "Chaque semaine",
    monthly: "Chaque mois",
    quarterly: "Chaque trimestre",
    yearly: "Chaque année",
};
export const CYCLE_SHORT = { once: "ponctuel", weekly: "hebdo", monthly: "mensuel", quarterly: "trimestriel", yearly: "annuel" };
export const CYCLE_WORD = {
    monthly: "Abonnement mensuel",
    yearly: "Abonnement annuel",
    quarterly: "Abonnement trimestriel",
    weekly: "Abonnement hebdo",
    once: "Achat ponctuel",
    "": "Facture hors abonnement",
};
// How much of a payment falls in one month.
export const PER_MONTH = { weekly: 52 / 12, monthly: 1, quarterly: 1 / 3, yearly: 1 / 12, once: 0 };

export const PROJECT_TONES = ["violet", "sky", "emerald", "amber", "rose", "teal", "orange", "indigo", "pink", "lime", "fuchsia", "blue"];

export const KIND = { invoice: "Facture", receipt: "Reçu", order: "Commande", statement: "Relevé", document: "Document" };
// What an amount of the analytics is.
export const KIND_WORD = { invoice: "facture", estimate: "facture à récupérer · montant estimé", planned: "prévu" };

// The state of a due date, in a sentence and as a tag: its colour comes from its s-* class.
export const STATUS = {
    upcoming: "à venir",
    today: "aujourd'hui",
    matched: "facture trouvée",
    missing: "facture à récupérer",
    skipped: "sans facture",
    paid: "payé",
    invoice: "facture hors abonnement",
};
export const STATUS_TAG = {
    matched: "Facture trouvée",
    missing: "À récupérer",
    upcoming: "À venir",
    today: "Aujourd'hui",
    paid: "Payé",
    skipped: "Sans facture",
    invoice: "Hors abonnement",
};
// A deadline's state: its colour class and its word.
export const DL = { todo: ["s-deadline", "À faire"], done: ["s-matched", "Faite"], past: ["s-paid", "Passée"] };
