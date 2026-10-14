import type { HTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/cn";

// The surface panel. Pages had hand-written `rounded-xl border border-slate-800
// bg-slate-900/40 p-6` and it drifted (rounded-lg vs -xl, /40 vs /60, p-4 vs
// p-6). One recipe: `tint` picks the fill, `pad` the inset. Override any single
// class via className.
export const cardVariants = cva("rounded-xl border border-slate-800", {
  variants: {
    tint: {
      base: "bg-slate-900/40",
      raised: "bg-slate-900/60",
      bare: "",
    },
    pad: {
      none: "",
      sm: "p-4",
      md: "p-6",
    },
  },
  defaultVariants: { tint: "base", pad: "md" },
});

export type CardProps = HTMLAttributes<HTMLDivElement> & VariantProps<typeof cardVariants>;

export function Card({ tint, pad, className, ...props }: CardProps) {
  return <div className={cn(cardVariants({ tint, pad }), className)} {...props} />;
}
