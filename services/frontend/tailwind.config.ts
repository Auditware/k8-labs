import type { Config } from "tailwindcss";
import { palette } from "./src/lib/tokens";

export default {
  content: ["./index.html", "./src/**/*.{ts,tsx}"],
  theme: {
    extend: {
      colors: {
        surface: palette.surface,
        hairline: { DEFAULT: palette.hairline, strong: palette.hairlineStrong },
        accent: palette.accent,
        status: { ok: palette.ok, warn: palette.warn, danger: palette.danger },
      },
      // Micro type-scale. The design leans on sizes below Tailwind's xs(12)/sm(14)
      // plus one at 13. Named by their pixel value so the class states its own
      // size, and defined once so a future bump is a single edit rather than 60
      // scattered text-[11px] strings. text-11 == 11px, and so on.
      fontSize: {
        "10": ["10px", { lineHeight: "14px" }],
        "11": ["11px", { lineHeight: "16px" }],
        "13": ["13px", { lineHeight: "18px" }],
        "15": ["15px", { lineHeight: "22px" }],
        "17": ["17px", { lineHeight: "24px" }],
        "18": ["18px", { lineHeight: "26px" }],
        "22": ["22px", { lineHeight: "28px" }],
        "26": ["26px", { lineHeight: "32px" }],
      },
    },
  },
  plugins: [],
} satisfies Config;
