// Analyse: what the business costs, in euros, by month, by project and by service.
// Every amount comes from /api/analytics in euros: an invoice, a past due date
// whose invoice is missing (its estimate), or a due date to come. The page adds
// them up; nothing is stored here.

import { esc } from "../dom.js";
import { euros, eurosExact, fmtDay, groupBy, iso, money, monthLabel, monthName, num, pad, parseISO } from "../format.js";
import { t } from "../i18n.js";
import { icon } from "../icons.js";
import { eurOf, projectById, projectName, S, toneOf } from "../state.js";
import { dateBadge, foreignIn, kpi, logo, pageHead, rateList } from "../ui/parts.js";
import { amountKind, cycleWord, PER_MONTH } from "../words.js";

const ym = (date) => date.slice(0, 7);
const shortMonth = (key) => monthName(Number(key.slice(5, 7)) - 1, true);
const sumEur = (list) => list.reduce((t, s) => t + s.eur, 0);

function monthsBetween(a, b) {
    const out = [];
    let [y, m] = a.split("-").map(Number);
    const [y2, m2] = b.split("-").map(Number);
    while (y < y2 || (y === y2 && m <= m2)) {
        out.push(`${y}-${pad(m)}`);
        if (++m > 12) (m = 1), y++;
    }
    return out;
}

const addMonthsKey = (key, n) => {
    const d = new Date(Number(key.slice(0, 4)), Number(key.slice(5, 7)) - 1 + n, 1);
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}`;
};

// anRange: the months of the chosen period, the ones of them already lived, and the months to come shown in grey.
function anRange() {
    const now = ym(S.st.today);
    const spend = S.an.data.spend;
    const p = S.an.period;
    if (p === "12m") {
        const first = addMonthsKey(now, -11);
        return { months: monthsBetween(first, now), ahead: monthsBetween(addMonthsKey(now, 1), addMonthsKey(now, 3)), label: t("analytics.over12") };
    }
    if (p === "all") {
        const first = spend.length ? ym(spend[0].date) : now;
        return { months: monthsBetween(first < now ? first : now, now), ahead: monthsBetween(addMonthsKey(now, 1), addMonthsKey(now, 3)), label: t("analytics.sinceStart") };
    }
    const all = monthsBetween(`${p}-01`, `${p}-12`);
    return { months: all.filter((k) => k <= now), ahead: all.filter((k) => k > now), label: t("analytics.inYear", { year: p }) };
}

const inProject = (s) => S.an.project === "all" || (S.an.project === "none" ? !s.project : s.project === S.an.project);

// anScope: the amounts the lists add up — the period, the project, and the month picked on the chart.
function anScope(withProject = true) {
    const { months, ahead } = anRange();
    const keep = new Set(S.an.month ? [S.an.month] : [...months, ...ahead]);
    return S.an.data.spend.filter((s) => keep.has(ym(s.date)) && (!withProject || inProject(s)));
}

function anFigures() {
    const { months, label } = anRange();
    const lived = new Set(months);
    const today = S.st.today;
    const spend = S.an.data.spend.filter(inProject);
    const past = spend.filter((s) => s.kind !== "planned" && lived.has(ym(s.date)));
    const total = sumEur(past);
    const guessed = sumEur(past.filter((s) => s.kind === "estimate"));
    const yearAhead = iso(new Date(parseISO(today).getFullYear() + 1, parseISO(today).getMonth(), parseISO(today).getDate()));
    const in90 = iso(new Date(parseISO(today).getTime() + 90 * 86400000));
    const planned = spend.filter((s) => s.kind === "planned");
    const subs = sumEur(planned.filter((s) => s.cycle && s.cycle !== "once" && s.date <= yearAhead)) / 12;
    const soon = sumEur(planned.filter((s) => s.date <= in90));
    return `<div class="kpis">
        ${kpi({ label: t("analytics.kpi.spent"), value: euros(total), sub: `${label}${guessed >= 1 ? ` · ${t("analytics.kpi.guessed", { amount: euros(guessed) })}` : ""}` })}
        ${kpi({ label: t("analytics.kpi.average"), value: euros(months.length ? total / months.length : 0), sub: t("analytics.kpi.perMonthOver", { count: months.length }) })}
        ${kpi({ label: t("analytics.kpi.subs"), value: `≈ ${euros(subs)}`, sub: t("analytics.kpi.subsSub") })}
        ${kpi({ label: t("analytics.kpi.toPay"), value: euros(soon), sub: t("analytics.kpi.toPaySub") })}
    </div>`;
}

// anChart: one bar per month, from zero; what is still to come in grey (« prévu »).
function anChart() {
    const { months, ahead } = anRange();
    const keys = [...months, ...ahead];
    const now = ym(S.st.today);
    const spend = S.an.data.spend.filter(inProject);
    const byMonth = groupBy(spend, (s) => ym(s.date));
    const bars = keys.map((key) => {
        const list = byMonth[key] || [];
        return { key, spent: sumEur(list.filter((s) => s.kind !== "planned")), plan: sumEur(list.filter((s) => s.kind === "planned")), list };
    });
    const hi = Math.max(...bars.map((b) => b.spent + b.plan), 1);
    const step = 10 ** Math.floor(Math.log10(hi));
    const top = Math.ceil(hi / step) * step;
    const tone = S.an.project === "all" ? "c-accent" : toneOf(S.an.project === "none" ? "" : S.an.project);
    const col = (b) => {
        const total = b.spent + b.plan;
        const top3 = Object.entries(groupBy(b.list, (s) => s.service))
            .map(([name, l]) => [name, sumEur(l)])
            .sort((x, y) => y[1] - x[1])
            .slice(0, 4)
            .map(([name, v]) => `${name} ${euros(v)}`)
            .join(" · ");
        const title = [t(b.plan ? "analytics.barTitlePlanned" : "analytics.barTitle", { month: monthLabel(b.key), spent: eurosExact(b.spent), plan: eurosExact(b.plan) }), top3].filter(Boolean).join("\n");
        const stack = total
            ? `<span class="stack" style="height:${(total / top) * 100}%"><span class="val">${euros(total)}</span>${b.plan ? `<i class="plan" style="flex:${b.plan}"></i>` : ""}${b.spent ? `<i class="spent" style="flex:${b.spent}"></i>` : ""}</span>`
            : "";
        return `<button class="col ${S.an.month === b.key ? "sel" : ""}" data-action="an-month" data-key="${b.key}" title="${esc(title)}">${stack}</button>`;
    };
    const label = (b) => {
        const cls = [b.key === now && "now", S.an.month === b.key && "sel"].filter(Boolean).join(" ");
        const yearMark = b.key.endsWith("-01") || b.key === keys[0] ? ` ${b.key.slice(2, 4)}` : "";
        return `<span class="${cls}">${shortMonth(b.key).replace(".", "")}${yearMark}</span>`;
    };
    return `<section class="card an-chart">
        <header>
            <b>${t("analytics.chart.title")}</b><span>${t("analytics.chart.sub")}</span>
            <span class="legend"><span><i class="dot ${tone}"></i>${t("analytics.chart.paid")}</span>${ahead.length || bars.some((b) => b.plan) ? `<span><i class="dot" style="--c:#3a3d43"></i>${t("analytics.chart.planned")}</span>` : ""}</span>
        </header>
        <div class="chart ${tone}">
            <div class="axis"><span style="top:0">${euros(top)}</span><span style="top:50%">${euros(top / 2)}</span><span style="top:100%">${euros(0)}</span></div>
            <div class="plot">
                <i class="gl" style="top:0"></i><i class="gl" style="top:50%"></i><i class="gl" style="bottom:0"></i>
                <div class="cols">${bars.map(col).join("")}</div>
            </div>
            <span></span>
            <div class="months">${bars.map(label).join("")}</div>
        </div>
    </section>`;
}

// projectRows: what each project weighs in the period; a click filters the whole page on it.
function projectRows() {
    const past = anScope(false).filter((s) => s.kind !== "planned");
    const total = sumEur(past) || 1;
    const by = groupBy(past, (s) => s.project || "");
    const rows = S.an.data.projects
        .map((p) => ({ id: p.id, list: by[p.id] || [] }))
        .concat(by[""] ? [{ id: "", list: by[""] }] : [])
        .map((r) => ({ ...r, sum: sumEur(r.list), services: new Set(r.list.map((s) => s.service)).size }))
        .sort((a, b) => (a.id === "") - (b.id === "") || b.sum - a.sum);
    const row = (r) => {
        const key = r.id || "none";
        const p = projectById(r.id);
        return `<div class="lrow ${S.an.project === key ? "on" : ""}" data-action="an-project" data-id="${key}" role="button">
            <i class="dot ${toneOf(r.id)}"></i>
            <span class="lm"><b>${esc(projectName(r.id))}</b><span>${t("analytics.services", { count: r.services })} · ${t("analytics.percent", { n: num((r.sum / total) * 100) })}</span>
                <span class="share-line ${toneOf(r.id)}"><i style="width:${(r.sum / total) * 100}%"></i></span></span>
            <span class="lv"><b>${euros(r.sum)}</b></span>
            ${p ? `<button class="btn ghost sm edit" data-action="edit-project" data-id="${p.id}" title="${t("analytics.editProject")}">${icon("edit", 14)}</button>` : ""}
        </div>`;
    };
    return rows.map(row).join("");
}

// projectIdeas: the domain names of the subscriptions that no project covers yet.
function projectIdeas() {
    const names = S.an.data.projects.map((p) => p.name.toLowerCase());
    const ideas = new Set();
    for (const p of S.st.payments) {
        const m = (p.match || "").toLowerCase().match(/^([a-z0-9-]+)\.[a-z]{2,}$/);
        if (m && !names.includes(m[1])) ideas.add(m[1].replace(/^./, (c) => c.toUpperCase()));
    }
    return [...ideas];
}

function projectsCard() {
    const ideas = projectIdeas();
    const empty = !S.an.data.projects.length;
    return `<section>
        <div class="list-head"><div class="stitle">${t("analytics.projects")}</div><span class="grow"></span>
            <button class="btn sm" data-action="new-project">${icon("plus", 13)}${t("analytics.newProject")}</button></div>
        <div class="card an-list">
            ${empty ? `<div class="an-empty">${t("analytics.noProject")}</div>` : ""}
            ${projectRows()}
            ${ideas.length ? `<div class="list-foot"><small>${t("analytics.ideas")}</small>${ideas.map((n) => `<button class="btn sm" data-action="quick-project" data-name="${esc(n)}">${icon("plus", 12)}${esc(n)}</button>`).join("")}</div>` : ""}
        </div>
    </section>`;
}

// projectSelect files a service or an invoice: « Auto » follows the keywords, « Aucun projet » overrides them.
function projectSelect(kind, ref, value, auto) {
    const projects = S.an.data.projects;
    if (!projects.length) return "";
    const shown = value === "-" ? "" : value || auto;
    const opts = [
        `<option value="">${esc(auto ? t("analytics.autoProject", { name: projectName(auto) }) : t("analytics.unfiled"))}</option>`,
        ...projects.map((p) => `<option value="${p.id}" ${value === p.id ? "selected" : ""}>${esc(p.name)}</option>`),
        auto || value === "-" ? `<option value="-" ${value === "-" ? "selected" : ""}>${t("analytics.noneProject")}</option>` : "",
    ];
    return `<select class="select psel ${shown ? `set ${toneOf(shown)}` : ""}" data-assign="${kind}" data-ref="${esc(ref)}" title="${t("analytics.project")}">${opts.join("")}</select>`;
}

function serviceRows(scope) {
    const items = scope.filter((s) => s.kind !== "planned");
    const total = sumEur(items) || 1;
    const groups = Object.entries(groupBy(items, (s) => s.service))
        .map(([name, list]) => ({ name, list, sum: sumEur(list) }))
        .sort((a, b) => b.sum - a.sum);
    if (!groups.length) return `<div class="an-empty">${t("analytics.noSpend")}</div>`;
    return groups
        .map((g) => {
            const first = g.list[g.list.length - 1];
            const chosen = g.list.find((s) => s.projectBy === "service");
            const value = chosen ? chosen.project || "-" : "";
            const auto = g.list.find((s) => s.projectBy === "keyword")?.project || "";
            const pays = S.st.payments.filter((p) => p.name === g.name && p.cycle !== "once" && (!p.end || p.end >= S.st.today));
            const monthly = pays.reduce((t, p) => t + eurOf(p.current, p.currentCurrency) * PER_MONTH[p.cycle], 0);
            const n = g.list.length;
            const sub = [cycleWord(first.cycle), t("analytics.payments", { count: n }), monthly >= 0.5 ? t("analytics.aboutPerMonth", { amount: euros(monthly) }) : ""].filter(Boolean).join(" · ");
            return `<div class="lrow">
                ${logo(g.name, first.domain)}
                <span class="lm"><b>${esc(g.name)}</b><span>${sub}</span></span>
                ${projectSelect("service", g.name, value, auto)}
                <span class="lv"><b>${euros(g.sum)}</b><small>${t("analytics.percent", { n: num((g.sum / total) * 100) })}</small></span>
            </div>`;
        })
        .join("");
}

function ledgerRows(scope) {
    const list = [...scope].sort((a, b) => b.date.localeCompare(a.date));
    if (!list.length) return `<div class="an-empty">${t("analytics.noSpend")}</div>`;
    const serviceProject = {};
    for (const s of S.an.data.spend) if (s.projectBy !== "invoice") serviceProject[s.service] ??= s.project;
    return list
        .map((s) => {
            const pick =
                s.kind === "invoice"
                    ? projectSelect("invoice", s.invoiceId, s.projectBy === "invoice" ? s.project || "-" : "", serviceProject[s.service] || "")
                    : s.project
                      ? `<span class="tag ${toneOf(s.project)}">${esc(projectName(s.project))}</span>`
                      : "";
            const original = s.currency !== "EUR" ? `<small>${money(s.amount, s.currency)}</small>` : "";
            return `<div class="lrow">
                ${dateBadge(s.date)}${logo(s.service, s.domain)}
                <span class="lm"><b>${esc(s.service)}</b><span>${amountKind(s.kind)}${s.label ? ` · ${esc(s.label)}` : ""}</span></span>
                ${pick}
                <span class="lv"><b>${s.kind === "estimate" ? "≈ " : ""}${eurosExact(s.eur)}</b>${original}</span>
            </div>`;
        })
        .join("");
}

function listsCard() {
    const scope = anScope();
    const tabs = ["services", "ledger"];
    const chips = [
        S.an.month ? `<button class="btn sm" data-action="an-month" data-key="${S.an.month}" title="${t("analytics.wholePeriod")}">${monthLabel(S.an.month)}${icon("x", 12)}</button>` : "",
        S.an.project !== "all" ? `<button class="btn sm" data-action="an-project" data-id="all" title="${t("analytics.allProjects")}"><i class="dot ${toneOf(S.an.project === "none" ? "" : S.an.project)}"></i>${esc(projectName(S.an.project === "none" ? "" : S.an.project))}${icon("x", 12)}</button>` : "",
    ].join("");
    return `<section>
        <div class="list-head">
            <div class="seg">${tabs.map((k) => `<button class="${S.an.tab === k ? "on" : ""}" data-action="an-tab" data-tab="${k}">${t(`analytics.tab.${k}`)}</button>`).join("")}</div>
            <span class="grow"></span>${chips}
        </div>
        <div class="card an-list">${S.an.tab === "services" ? serviceRows(scope) : ledgerRows(scope)}</div>
    </section>`;
}

export function viewAnalytics() {
    const d = S.an.data;
    const years = [...new Set(d.spend.map((s) => s.date.slice(0, 4)))].filter((y) => y <= S.st.today.slice(0, 4)).sort().reverse();
    const periods = [["12m", t("analytics.period12")], ...years.map((y) => [y, y]), ["all", t("analytics.periodAll")]];
    if (!periods.some(([k]) => k === S.an.period)) S.an.period = "12m";
    const tools = `<div class="seg">${periods.map(([k, label]) => `<button class="${S.an.period === k ? "on" : ""}" data-action="an-period" data-p="${k}">${label}</button>`).join("")}</div>`;
    const foreign = foreignIn(d.spend.map((s) => s.currency));
    const notes = [
        foreign.length && d.perEUR ? t("analytics.ratesNote", { date: fmtDay(d.rateDate), rates: rateList(d.perEUR, foreign) }) : "",
        d.noAmount ? t("analytics.noAmountNote", { count: d.noAmount }) : "",
    ].filter(Boolean);
    return `
    ${pageHead({ title: t("analytics.title"), sub: t("analytics.sub"), tools })}
    ${anFigures()}
    ${anChart()}
    <div class="an-cols">${projectsCard()}${listsCard()}</div>
    ${notes.map((n) => `<p class="an-note">${n}</p>`).join("")}`;
}
