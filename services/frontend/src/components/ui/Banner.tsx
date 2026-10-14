import type { HTMLAttributes } from "react";
import { cva, type VariantProps } from "class-variance-authority";
import { cn } from "../../lib/cn";

// Inline status message (form error, info notice). Replaces the three drifting
// copies of `border border-red-500/30 bg-red-500/10 ...` that disagreed on
// radius, padding and text shade. role=alert only for the error tone so a
// screen reader announces failures, not passive info.
export const bannerVariants = cva("rounded-lg border px-3.5 py-2.5 text-sm", {
  variants: {
    tone: {
      error: "border-red-500/30 bg-red-500/10 text-red-300",
      info: "border-slate-700 bg-slate-800/50 text-slate-300",
    },
  },
  defaultVariants: { tone: "info" },
});

export type BannerProps = HTMLAttributes<HTMLDivElement> & VariantProps<typeof bannerVariants>;

export function Banner({ tone, className, ...props }: BannerProps) {
  return (
    <div
      role={tone === "error" ? "alert" : undefined}
      className={cn(bannerVariants({ tone }), className)}
      {...props}
    />
  );
}
