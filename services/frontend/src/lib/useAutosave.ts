import { useEffect, useRef } from "react";

// Autosave, debounced, with no Save button and no "Saved" flash.
//
// The rule this enforces: write when the form differs from what the server is
// known to hold, and never otherwise.
//
// That phrasing matters. The obvious implementation - a "skip the first run"
// boolean - is wrong under React StrictMode, which mounts, runs effects, tears
// them down and runs them again. The first invocation consumes the flag, the
// second sees an armed hook and saves, so merely opening the page PUT the
// server's own answer straight back to it. Comparing VALUES instead of
// counting invocations is immune to that: an effect that runs twice with the
// same value still has nothing to save.
//
// The debounce is the second half - the timer resets on every change, so a
// burst of typing collapses into one request once the user pauses.
//
// The trade is that a save can be in flight when the tab closes, losing at
// most the last `delayMs` of typing. Acceptable for a website and a bio.
export function useAutosave<T>(value: T, save: (v: T) => void, delayMs = 800) {
  const saveRef = useRef(save);
  saveRef.current = save;
  const latest = useRef(value);
  latest.current = value;

  // The serialized value we believe the server holds. Adopted from the first
  // value the hook sees, so a form that is seeded synchronously needs no setup:
  // the seed is the baseline and matching it is not an edit. For a form seeded
  // LATE from a fetch, the caller re-declares the baseline with sync() when the
  // data lands (see below) so populating the inputs still isn't an edit.
  const synced = useRef<string | null>(null);

  // Keyed on the CONTENTS, not the reference. Callers pass an object literal
  // ({website, bio}), which is a new identity every render - depending on the
  // reference would restart the timer forever and never save.
  const key = JSON.stringify(value);

  useEffect(() => {
    if (synced.current === null) {
      // First run adopts the current value as the baseline. This is what makes
      // the hook StrictMode-safe: the mount-unmount-remount double invoke runs
      // this effect twice with the same value, and the second run sees a
      // baseline equal to key and does nothing - where a "skip first run"
      // boolean would have been consumed by the first pass and saved on the
      // second.
      synced.current = key;
      return;
    }
    if (synced.current === key) return;
    const t = setTimeout(() => {
      // Recorded before the request, not after: it marks intent, so a render
      // that happens while the save is in flight doesn't queue a duplicate.
      synced.current = key;
      saveRef.current(latest.current);
    }, delayMs);
    return () => clearTimeout(t);
  }, [key, delayMs]);

  // Declares what the server holds. Call this with the fetched profile before
  // seeding the form, so populating the inputs isn't mistaken for an edit.
  return {
    sync: (v: T) => {
      synced.current = JSON.stringify(v);
    },
  };
}
