import type { ReactNode } from "react";

// The shell every full-page view sits in.
//
// This exists because the admin pages had each grown their own answer to the
// same three questions - how wide, how much padding, how big is the title -
// and drifted to max-w-3xl / max-w-4xl / max-w-6xl with subtly different
// heading markup. Switching between Users and Settings moved the title and
// resized the content, which reads as four pages from four apps.
//
// So width, padding, scroll and the title live here, once. A page supplies its
// name and its content; it does not get a say in the frame. The one deliberate
// non-user of this is LabPage - the student lab is a two-pane split that fills
// the viewport and scrolls internally, not a titled document.
//
// The stretch on 2xl is global on purpose. It started as an AdminPage-only
// override so the topology map got more canvas on an ultrawide, but a width
// that changes per page is exactly the drift this component removes: every
// page now grows the same way, and the widest content sets the rule.
export function Page({ title, actions, children }: {
  title: string;
  // Rendered on the title row, right-aligned - for a page-level control that
  // belongs to the page rather than to any one section.
  actions?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="h-full overflow-y-auto p-6">
      <div className="mx-auto max-w-6xl 2xl:max-w-[86rem]">
        <div className="flex min-h-8 items-center justify-between gap-4">
          {/* h2, not h1 - the app's own brand mark is the h1, so a page title
              is one level down. Keeps the heading order real for a screen
              reader instead of restarting at h1 on every route. */}
          <h2 className="text-lg font-semibold text-slate-100">{title}</h2>
          {actions}
        </div>
        {children}
      </div>
    </div>
  );
}

// A titled block within a page: the "Topology" under "Admin".
//
// The uppercase label is the only sub-heading style in the app now. Settings
// used to render its section titles as sentence-case 14px with an icon, which
// made a Settings section look like a different kind of thing from an Admin
// section when they are the same thing.
export function PageSection({ title, icon, children, className = "" }: {
  title: string;
  // Optional leading glyph. Settings leans on these to tell its cards apart at
  // a glance; Admin and Users don't need them.
  icon?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`mt-6 ${className}`}>
      <h3 className="mb-3 flex items-center gap-2 text-xs font-semibold uppercase tracking-wide text-slate-500">
        {icon}
        {title}
      </h3>
      {children}
    </section>
  );
}
