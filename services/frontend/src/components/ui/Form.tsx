import { ChevronDown } from "lucide-react";
import { Slot } from "@radix-ui/react-slot";
import { cva, type VariantProps } from "class-variance-authority";
import type { ReactNode, SelectHTMLAttributes, InputHTMLAttributes, TextareaHTMLAttributes, ButtonHTMLAttributes } from "react";
import { cn } from "../../lib/cn";

// The app's form controls, in one place.
//
// Every form had been hand-rolling its own `rounded border border-slate-700
// bg-slate-800 px-3 py-2 text-sm` string, which is why the inputs, the selects
// and the buttons had all drifted to slightly different heights and text
// sizes. They are now one size (h-10 / 14px) so a row of them lines up, and
// one focus treatment so tabbing through a form looks deliberate.
//
// The sizing is up from the old text-xs controls. At 12px with 6px of padding
// the forms read as a dense admin tool from a decade ago; 14px in a 40px
// control is what current web apps use, and it is also the point where a tap
// target is big enough on a touchscreen.

const control =
  "h-10 w-full rounded-lg border border-slate-700/80 bg-slate-800/60 px-3.5 text-sm text-slate-100 " +
  "placeholder:text-slate-500 outline-none transition-colors " +
  "focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 disabled:opacity-50";

export function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-13 font-medium text-slate-300">{label}</span>
      {children}
      {hint && <span className="mt-1.5 block text-xs text-slate-500">{hint}</span>}
    </label>
  );
}

export function Input({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input {...props} className={`${control} ${className}`} />;
}

export function Textarea({ className = "", ...props }: TextareaHTMLAttributes<HTMLTextAreaElement>) {
  // Height comes from rows, so the fixed h-10 is dropped here.
  return <textarea {...props} className={`${control} h-auto resize-y py-2.5 ${className}`} />;
}

// A select that doesn't look like 2004.
//
// The native control's own arrow is the tell: it renders as the OS widget and
// ignores the dark theme entirely, which is what made the expiry dropdown look
// out of place next to the inputs. appearance-none removes it and we draw our
// own chevron, so the control matches every other field.
export function Select({ className = "", children, ...props }: SelectHTMLAttributes<HTMLSelectElement>) {
  return (
    <div className="relative">
      <select {...props} className={`${control} cursor-pointer appearance-none pr-10 ${className}`}>
        {children}
      </select>
      <ChevronDown
        size={16}
        className="pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 text-slate-400"
      />
    </div>
  );
}

// The one button. Every page used to hand-roll `rounded-lg bg-blue-... px-...`
// and they drifted to three paddings and three text sizes; a cva recipe makes
// the shape a choice of (variant x size), and cn() lets a caller override one
// thing via className without a specificity fight. `subtle` is the low-emphasis
// icon/toolbar button; `icon` is the square sizing for a lone glyph.
export const buttonVariants = cva(
  "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-lg font-medium transition-colors " +
    "disabled:opacity-50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-500/30",
  {
    variants: {
      variant: {
        primary: "bg-blue-600 text-white hover:bg-blue-500",
        ghost: "text-slate-300 ring-1 ring-inset ring-slate-700 hover:bg-slate-800",
        danger: "text-red-400 ring-1 ring-inset ring-red-500/30 hover:bg-red-500/10",
        subtle: "text-slate-400 hover:bg-slate-800 hover:text-slate-200",
      },
      size: {
        md: "h-10 px-4 text-sm",
        sm: "h-8 px-3 text-13",
        icon: "h-8 w-8",
      },
    },
    defaultVariants: { variant: "primary", size: "md" },
  }
);

export type ButtonProps = ButtonHTMLAttributes<HTMLButtonElement> &
  VariantProps<typeof buttonVariants> & { asChild?: boolean };

export function Button({ variant, size, className, asChild = false, ...props }: ButtonProps) {
  // asChild renders the caller's own element (e.g. an <a> or router <Link>)
  // carrying the button styles, instead of a nested <button>.
  const Comp = asChild ? Slot : "button";
  return <Comp className={cn(buttonVariants({ variant, size }), className)} {...props} />;
}
