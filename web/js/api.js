// Calls to the local server. Every write carries X-Inc: the server refuses a
// write without it, which another website cannot send.

export async function api(method, url, body) {
    const opts = { method, headers: { "X-Inc": "1" } };
    if (body !== undefined) {
        opts.headers["Content-Type"] = "application/json";
        opts.body = JSON.stringify(body);
    }
    const r = await fetch(url, opts);
    if (!r.ok) throw new Error((await r.text()).trim() || r.statusText);
    return r.json();
}

// sendFiles uploads files to the folder: { dir } or, for a due date, { vendor, date, key }.
export async function sendFiles(files, params) {
    const fd = new FormData();
    for (const f of files) fd.append("file", f, f.name);
    const r = await fetch("/api/upload?" + new URLSearchParams(params), {
        method: "POST",
        headers: { "X-Inc": "1" },
        body: fd,
    });
    if (!r.ok) throw new Error((await r.text()).trim());
    return r.json();
}
