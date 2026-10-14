import { useEffect, useState } from "react";

// True below `breakpoint` px (default 768 = Tailwind md). Used where a layout
// can't be expressed with CSS alone because it changes what mounts, not just how
// it looks: the sidebar switches between an in-flow rail and an off-canvas drawer
// (one instance, not two - avoids duplicate nav landmarks), and the lab collapses
// its course/terminal split into a single tabbed pane. matchMedia so it also
// reacts to phone rotation, not just resize.
export function useIsNarrow(breakpoint = 768): boolean {
  const [narrow, setNarrow] = useState(
    () => typeof window !== "undefined" && window.innerWidth < breakpoint,
  );
  useEffect(() => {
    if (typeof window === "undefined" || !window.matchMedia) return;
    const mq = window.matchMedia(`(max-width: ${breakpoint - 1}px)`);
    const on = () => setNarrow(mq.matches);
    on();
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, [breakpoint]);
  return narrow;
}
