const COLORS = [
  ["#6366f1", "#8b5cf6"],
  ["#06b6d4", "#3b82f6"],
  ["#f59e0b", "#ef4444"],
  ["#10b981", "#14b8a6"],
  ["#ec4899", "#f43f5e"],
  ["#8b5cf6", "#a855f7"],
  ["#f97316", "#eab308"],
  ["#14b8a6", "#06b6d4"],
];

function hashString(str: string): number {
  let hash = 5381;
  for (let i = 0; i < str.length; i++) {
    hash = ((hash << 5) + hash + str.charCodeAt(i)) | 0;
  }
  return Math.abs(hash);
}

function articlePlaceholder(title: string): string {
  const idx = hashString(title) % COLORS.length;
  const [c1, c2] = COLORS[idx];
  const chars = title.slice(0, 2).toUpperCase() || "??";
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <defs><linearGradient id="g" x1="0%" y1="0%" x2="100%" y2="100%"><stop offset="0%" stop-color="${c1}"/><stop offset="100%" stop-color="${c2}"/></linearGradient></defs>
  <rect width="300" height="400" fill="url(#g)"/>
  <text x="150" y="220" text-anchor="middle" fill="white" font-family="system-ui,sans-serif" font-size="64" font-weight="700" opacity="0.9">${chars}</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function videoPlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#1e293b"/>
  <circle cx="150" cy="190" r="48" fill="#334155" stroke="#64748b" stroke-width="2"/>
  <polygon points="136,168 136,212 172,190" fill="#94a3b8"/>
  <text x="150" y="280" text-anchor="middle" fill="#64748b" font-family="system-ui,sans-serif" font-size="14">Video Preview</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function sheetMusicPlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#faf5ff"/>
  <g transform="translate(40,100)" stroke="#a855f7" stroke-width="1.5" fill="none">
    <line x1="0" y1="0" x2="220" y2="0"/><line x1="0" y1="16" x2="220" y2="16"/><line x1="0" y1="32" x2="220" y2="32"/><line x1="0" y1="48" x2="220" y2="48"/><line x1="0" y1="64" x2="220" y2="64"/>
  </g>
  <g fill="#c084fc">
    <ellipse cx="70" cy="132" rx="10" ry="8"/><ellipse cx="130" cy="116" rx="10" ry="8"/><ellipse cx="80" cy="172" rx="10" ry="8"/><ellipse cx="140" cy="188" rx="10" ry="8"/><ellipse cx="60" cy="212" rx="10" ry="8"/>
  </g>
  <line x1="85" y1="132" x2="85" y2="108" stroke="#a855f7" stroke-width="1.5"/>
  <line x1="145" y1="116" x2="145" y2="92" stroke="#a855f7" stroke-width="1.5"/>
  <line x1="95" y1="172" x2="95" y2="148" stroke="#a855f7" stroke-width="1.5"/>
  <text x="150" y="320" text-anchor="middle" fill="#c084fc" font-family="system-ui,sans-serif" font-size="14">Sheet Music</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function modPlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#0f172a"/>
  <g transform="translate(110,130)" fill="none" stroke="#334155" stroke-width="3">
    <circle cx="40" cy="40" r="36"/>
    <path d="M16,16 L64,64 M64,16 L16,64" stroke="#475569" stroke-width="2"/>
  </g>
  <circle cx="150" cy="170" r="12" fill="#475569"/>
  <text x="150" y="260" text-anchor="middle" fill="#64748b" font-family="system-ui,sans-serif" font-size="14">Mod Package</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function templatePlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#f0fdf4"/>
  <g transform="translate(60,80)" fill="none" stroke="#10b981" stroke-width="1.5">
    <rect x="0" y="0" width="180" height="200" rx="8"/>
    <rect x="8" y="8" width="164" height="36" rx="4" fill="#d1fae5"/>
    <rect x="8" y="56" width="80" height="12" rx="3" fill="#d1fae5"/>
    <rect x="8" y="74" width="120" height="12" rx="3" fill="#d1fae5"/>
    <rect x="8" y="96" width="164" height="52" rx="4" fill="#a7f3d0"/>
  </g>
  <text x="150" y="320" text-anchor="middle" fill="#10b981" font-family="system-ui,sans-serif" font-size="14">Template</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function promptPlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#1e1b4b"/>
  <g transform="translate(100,100)" fill="none" stroke="#6366f1" stroke-width="2">
    <rect x="0" y="12" width="100" height="76" rx="8"/>
    <path d="M20,30 L32,18 M68,18 L80,30 M80,58 L68,70 M32,70 L20,58" stroke-width="2"/>
    <circle cx="50" cy="44" r="10" stroke="#818cf8"/>
    <circle cx="46" cy="40" r="3" fill="#818cf8"/>
    <line x1="42" y1="54" x2="38" y2="62" stroke="#818cf8" stroke-width="1.5"/>
    <line x1="50" y1="54" x2="50" y2="62" stroke="#818cf8" stroke-width="1.5"/>
    <line x1="58" y1="54" x2="62" y2="62" stroke="#818cf8" stroke-width="1.5"/>
  </g>
  <text x="150" y="260" text-anchor="middle" fill="#818cf8" font-family="system-ui,sans-serif" font-size="14">AI Prompt</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

function genericPlaceholder(): string {
  const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="400" viewBox="0 0 300 400">
  <rect width="300" height="400" fill="#f8fafc"/>
  <g transform="translate(110,140)" fill="none" stroke="#94a3b8" stroke-width="2">
    <path d="M8,0 L72,0 L80,8 L80,80 L0,80 Z"/>
    <line x1="16" y1="24" x2="56" y2="24" stroke="#cbd5e1" stroke-width="2"/>
    <line x1="16" y1="36" x2="56" y2="36" stroke="#cbd5e1" stroke-width="2"/>
    <line x1="16" y1="48" x2="40" y2="48" stroke="#cbd5e1" stroke-width="2"/>
  </g>
  <text x="150" y="280" text-anchor="middle" fill="#94a3b8" font-family="system-ui,sans-serif" font-size="14">Content</text>
</svg>`;
  return `data:image/svg+xml,${encodeURIComponent(svg)}`;
}

export type PlaceholderType =
  | "article"
  | "image"
  | "video"
  | "audio"
  | "sheet_music"
  | "mod"
  | "prompt"
  | "template"
  | "other";

export function getCoverPlaceholder(
  contentType: PlaceholderType | string,
  title?: string
): string {
  switch (contentType) {
    case "article":
      return articlePlaceholder(title || "Article");
    case "video":
      return videoPlaceholder();
    case "sheet_music":
      return sheetMusicPlaceholder();
    case "mod":
      return modPlaceholder();
    case "prompt":
      return promptPlaceholder();
    case "template":
      return templatePlaceholder();
    case "image":
      return genericPlaceholder();
    case "audio":
      return genericPlaceholder();
    default:
      return genericPlaceholder();
  }
}
