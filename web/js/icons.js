// The icons: 24×24 strokes, drawn in the current text colour.

const ICONS = {
    calendar: '<rect x="3" y="4.5" width="18" height="16.5" rx="2.5"/><path d="M16 2.5v4M8 2.5v4M3 10h18"/>',
    receipt: '<path d="M6 2.5h12v19l-3-2-3 2-3-2-3 2z"/><path d="M9 7.5h6M9 11.5h6M9 15.5h3"/>',
    alert: '<circle cx="12" cy="12" r="9"/><path d="M12 7.5v5.5M12 16.5v.01"/>',
    folder: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.2h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
    folderPlus: '<path d="M3 7a2 2 0 0 1 2-2h4l2 2.2h8a2 2 0 0 1 2 2V18a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/><path d="M12 11v6M9 14h6"/>',
    file: '<path d="M14 3H7a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2V8z"/><path d="M14 3v5h5"/>',
    plus: '<path d="M12 5v14M5 12h14"/>',
    chevL: '<path d="m15 18-6-6 6-6"/>',
    chevR: '<path d="m9 18 6-6-6-6"/>',
    upload: '<path d="M12 15V3.5M7 8.5l5-5 5 5"/><path d="M4.5 15v3.5a2 2 0 0 0 2 2h11a2 2 0 0 0 2-2V15"/>',
    finder: '<path d="M14 4h6v6M20 4l-8.5 8.5"/><path d="M18 14v4.5a1.5 1.5 0 0 1-1.5 1.5h-11A1.5 1.5 0 0 1 4 18.5v-11A1.5 1.5 0 0 1 5.5 6H10"/>',
    external: '<rect x="4" y="4" width="16" height="16" rx="3"/><path d="M9 15l6-6M10 9h5v5"/>',
    edit: '<path d="M4 20h4L19.5 8.5a2.1 2.1 0 0 0-3-3L5 17z"/>',
    check: '<path d="m5 12.5 4.5 4.5L19 7.5"/>',
    x: '<path d="M6 6l12 12M18 6 6 18"/>',
    search: '<circle cx="11" cy="11" r="7"/><path d="m20 20-3.5-3.5"/>',
    trash: '<path d="M4 7h16M9.5 7V4.5h5V7M6.5 7l1 13h9l1-13"/>',
    repeat: '<path d="M17 2.5 20 5.5l-3 3"/><path d="M4 11.5V10a4.5 4.5 0 0 1 4.5-4.5H20"/><path d="m7 21.5-3-3 3-3"/><path d="M20 12.5V14a4.5 4.5 0 0 1-4.5 4.5H4"/>',
    clock: '<circle cx="12" cy="12" r="9"/><path d="M12 7v5l3 2"/>',
    wallet: '<path d="M4 7.5A2.5 2.5 0 0 1 6.5 5H18v4"/><rect x="3.5" y="7.5" width="17" height="12" rx="2.5"/><path d="M16 13.5h2"/>',
    sparkle: '<path d="M12 3.5 13.8 10.2 20.5 12 13.8 13.8 12 20.5 10.2 13.8 3.5 12 10.2 10.2z"/>',
    chart: '<path d="M4 20.5h16"/><rect x="5" y="11" width="3.6" height="6.5" rx="1"/><rect x="10.2" y="5.5" width="3.6" height="12" rx="1"/><rect x="15.4" y="13.5" width="3.6" height="4" rx="1"/>',
    layers: '<path d="M12 3.5 3 8l9 4.5L21 8z"/><path d="m3 12.5 9 4.5 9-4.5"/><path d="m3 16.5 9 4.5 9-4.5"/>',
    settings: '<path d="M4 6h9M17 6h3M4 12h3M11 12h9M4 18h11M19 18h1"/><circle cx="15" cy="6" r="2"/><circle cx="9" cy="12" r="2"/><circle cx="17" cy="18" r="2"/>',
    flag: '<path d="M5.5 21V4"/><path d="M5.5 4.5h11.5l-2.4 4.2 2.4 4.3H5.5"/>',
};

export const icon = (name, size = 16, cls = "") =>
    `<svg class="i ${cls}" width="${size}" height="${size}" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round">${ICONS[name]}</svg>`;
