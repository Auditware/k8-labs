import { Lock, Send } from "lucide-react";
import { HeroEscapeBackground } from "./HeroEscapeBackground";

// Where to send users who want more labs unlocked - the same @forefy_t Telegram
// the Footer and ServerIssueBadge already hand off to.
const ADMIN_TELEGRAM_URL = "https://t.me/forefy_t";

// MoreLabsNotice is the "unlock more" screen. Every lab is now on the progression
// ladder (see labs.ProgressionTiers), so the rest of the catalog is self-serve:
// clearing the labs you have opens the next set automatically, no request needed.
// Reached from the sidebar's locked "Encrypted" entry (which only shows while
// labs remain locked), this tells the user to keep climbing, styled to read the
// gated state as intentional rather than empty. Telegram stays as optional help.
export default function MoreLabsNotice() {
  return (
    <div className="relative flex h-full items-center justify-center overflow-hidden px-4 py-10">
      <HeroEscapeBackground className="pointer-events-none absolute inset-0 h-full w-full opacity-70" />
      <div className="pointer-events-none absolute left-1/2 top-1/2 h-[40rem] w-[40rem] -translate-x-1/2 -translate-y-1/2 rounded-full bg-accent/10 blur-[120px]" />

      <div className="animate-slide-up relative z-10 w-full max-w-md">
        <div className="rounded-2xl border border-hairline bg-surface/80 p-8 shadow-2xl backdrop-blur-sm ring-1 ring-white/5">
          <div className="flex items-center gap-2 font-mono text-11 uppercase tracking-[0.25em] text-accent">
            <Lock size={13} className="shrink-0" />
            Encrypted
          </div>

          <h1 className="mt-5 text-3xl font-bold leading-tight tracking-tight text-slate-100">
            Keep climbing to unlock the rest.
          </h1>
          <p className="mt-3 text-sm leading-relaxed text-slate-400">
            You&rsquo;re in - and the full catalog opens as you go. Clear the labs you have
            and the next set unlocks on its own, no request needed. Stuck, or after something
            specific? Message us and we&rsquo;ll point you the right way.
          </p>

          <a
            href={ADMIN_TELEGRAM_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-6 flex h-12 w-full items-center justify-center gap-2 rounded-lg bg-accent px-4 text-15 font-semibold text-slate-950 transition-colors hover:bg-accent/90"
          >
            <Send size={17} className="shrink-0" />
            Message us on Telegram
          </a>

          <p className="mt-4 text-center text-11 text-slate-600">
            Every solved lab opens the next - the ladder does the unlocking for you.
          </p>
        </div>
      </div>
    </div>
  );
}
