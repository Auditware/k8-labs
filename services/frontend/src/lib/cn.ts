import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

// Merge Tailwind classes so a later class wins over an earlier one of the same
// property (twMerge) while still supporting conditional/array/object inputs
// (clsx). This is what lets a shared primitive carry a sensible base and a
// caller override just one thing via className without specificity fights.
export function cn(...inputs: ClassValue[]): string {
  return twMerge(clsx(inputs));
}
