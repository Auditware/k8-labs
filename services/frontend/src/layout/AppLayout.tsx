import { useEffect, useState } from "react";
import { Outlet, useLocation } from "react-router-dom";
import { Menu } from "lucide-react";
import SideNav from "./SideNav";
import { useSeo } from "../lib/seo";

export default function AppLayout() {
  // Everything behind the auth gate (labs, settings, admin) is per-user and
  // must never be indexed. Child pages can still set their own tab title; this
  // just guarantees the robots default for the whole authenticated shell.
  useSeo({ noindex: true });

  // Mobile-only nav drawer. On md+ the sidebar is an in-flow rail and this state
  // is inert (the drawer + backdrop + top bar are all md:hidden). Closing on
  // route change is what lets nav links skip a per-link close handler.
  const [navOpen, setNavOpen] = useState(false);
  const { pathname } = useLocation();
  useEffect(() => setNavOpen(false), [pathname]);

  return (
    <div className="flex h-screen bg-slate-950 text-slate-100">
      {/* One SideNav: in-flow rail on md+, off-canvas drawer below md. */}
      <SideNav mobileOpen={navOpen} onNavigate={() => setNavOpen(false)} />

      {/* Drawer backdrop (mobile only; hidden on md+) */}
      <div
        className={`fixed inset-0 z-40 bg-black/60 transition-opacity md:hidden ${
          navOpen ? "opacity-100" : "pointer-events-none opacity-0"
        }`}
        onClick={() => setNavOpen(false)}
        aria-hidden
      />

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Mobile top bar: the only way to reach the drawer once the rail is
            hidden. md:hidden, so it adds no chrome on desktop. */}
        <header className="flex h-14 shrink-0 items-center gap-3 border-b border-slate-800 px-3 md:hidden">
          <button
            onClick={() => setNavOpen(true)}
            aria-label="Open menu"
            className="rounded-md p-2 text-slate-300 transition-colors hover:bg-slate-800"
          >
            <Menu size={20} />
          </button>
          <img src="/logo.svg" alt="" className="h-7 w-auto" />
          <span className="font-semibold tracking-tight text-slate-100">K8 Labs</span>
        </header>

        <main className="min-w-0 flex-1 overflow-hidden">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
