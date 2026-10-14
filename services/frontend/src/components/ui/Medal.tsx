// Rank medal (coin + star on a ribbon), tinted by tier. Shared so the landing's
// track-record cards and the leaderboard's rank badges render the exact same
// medal - the two surfaces read as one.
const MEDAL_TONES = {
  gold: { a: "#FCE39A", b: "#E7B24A", c: "#B47A28", r1: "#D9A63E", r2: "#8A5E20", star: "#FCE7A6", starStroke: "#9a6d24", ring: "#7d541c", shine: "#fff6dc" },
  silver: { a: "#EEF3F8", b: "#C7D2DE", c: "#8A99AB", r1: "#B8C4D0", r2: "#6B7684", star: "#F4F8FC", starStroke: "#5c6672", ring: "#586472", shine: "#ffffff" },
  bronze: { a: "#F1C79A", b: "#C88A4E", c: "#8A5A2A", r1: "#C1854A", r2: "#7A4E22", star: "#F6DBB4", starStroke: "#7a4e22", ring: "#5f3f1c", shine: "#ffe9cf" },
} as const;

export type Tone = keyof typeof MEDAL_TONES;

export function Medal({ tone, size = 40 }: { tone: Tone; size?: number }) {
  const t = MEDAL_TONES[tone];
  const fid = `k8-medal-face-${tone}`;
  const rid = `k8-medal-ring-${tone}`;
  return (
    <svg width={size} height={size} viewBox="0 0 44 44" fill="none" aria-hidden className="drop-shadow-[0_1px_2px_rgba(0,0,0,0.4)]">
      <defs>
        <radialGradient id={fid} cx="42%" cy="34%" r="72%">
          <stop offset="0" stopColor={t.a} />
          <stop offset="0.5" stopColor={t.b} />
          <stop offset="1" stopColor={t.c} />
        </radialGradient>
        <linearGradient id={rid} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0" stopColor={t.r1} />
          <stop offset="1" stopColor={t.r2} />
        </linearGradient>
      </defs>
      {/* Ribbon tails, drawn first so the coin overlaps their tops - two fishtail
          strips splaying from behind the coin. */}
      <path d="M22 25.5 L17.0 25.5 L14.6 41.5 L17.5 38.4 L20.4 41.5 L22 29 Z" fill={`url(#${rid})`} stroke={t.ring} strokeWidth="0.5" strokeLinejoin="round" />
      <path d="M22 25.5 L27.0 25.5 L29.4 41.5 L26.5 38.4 L23.6 41.5 L22 29 Z" fill={`url(#${rid})`} stroke={t.ring} strokeWidth="0.5" strokeLinejoin="round" />
      <circle cx="22" cy="19" r="12.5" fill={`url(#${rid})`} />
      <circle cx="22" cy="19" r="9.6" fill={`url(#${fid})`} stroke={t.ring} strokeWidth="0.6" />
      <path
        d="M22 11.9 L23.73 16.61 L28.75 16.81 L24.81 19.91 L26.17 24.74 L22 21.95 L17.83 24.74 L19.19 19.91 L15.25 16.81 L20.27 16.61 Z"
        fill={t.star}
        stroke={t.starStroke}
        strokeWidth="0.5"
        strokeLinejoin="round"
      />
      <path d="M15.5 13.2 A9.6 9.6 0 0 1 24 10.4" stroke={t.shine} strokeOpacity="0.55" strokeWidth="1.2" strokeLinecap="round" />
    </svg>
  );
}
