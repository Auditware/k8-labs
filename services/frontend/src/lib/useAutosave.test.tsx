import { StrictMode } from "react";
import { renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useAutosave } from "./useAutosave";

// The autosave hook earns a test because its two failure modes are both
// silent: it either writes when it shouldn't (a wasted PUT of the server's own
// value on every page load) or never writes at all. Neither throws, so only an
// assertion catches a regression.

beforeEach(() => vi.useFakeTimers());
afterEach(() => vi.useRealTimers());

describe("useAutosave", () => {
  it("does not save the value it was seeded with", () => {
    const save = vi.fn();
    renderHook(({ v }) => useAutosave(v, save, 500), {
      initialProps: { v: { bio: "from server" } },
    });
    vi.advanceTimersByTime(2000);
    expect(save).not.toHaveBeenCalled();
  });

  it("saves once the value changes, after the debounce", () => {
    const save = vi.fn();
    const { rerender } = renderHook(({ v }) => useAutosave(v, save, 500), {
      initialProps: { v: { bio: "a" } },
    });
    rerender({ v: { bio: "ab" } });
    expect(save).not.toHaveBeenCalled(); // still within the debounce window
    vi.advanceTimersByTime(500);
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith({ bio: "ab" });
  });

  it("collapses a burst of changes into a single save", () => {
    const save = vi.fn();
    const { rerender } = renderHook(({ v }) => useAutosave(v, save, 500), {
      initialProps: { v: { bio: "a" } },
    });
    // Each change lands before the timer elapses, so the timer keeps resetting.
    rerender({ v: { bio: "ab" } });
    vi.advanceTimersByTime(200);
    rerender({ v: { bio: "abc" } });
    vi.advanceTimersByTime(200);
    rerender({ v: { bio: "abcd" } });
    expect(save).not.toHaveBeenCalled();
    vi.advanceTimersByTime(500);
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith({ bio: "abcd" });
  });

  it("does not fire a spurious save under StrictMode's double-invoked effects", () => {
    // The regression this guards: a "skip the first run" flag is consumed by
    // StrictMode's first effect pass, so the second pass sees an armed hook and
    // saves the seed value. Comparing values instead of counting runs must be
    // immune to the double mount.
    const save = vi.fn();
    renderHook(({ v }) => useAutosave(v, save, 500), {
      wrapper: StrictMode,
      initialProps: { v: { bio: "from server" } },
    });
    vi.advanceTimersByTime(2000);
    expect(save).not.toHaveBeenCalled();
  });

  it("treats re-seeding via sync() as not-an-edit", () => {
    const save = vi.fn();
    const { result, rerender } = renderHook(({ v }) => useAutosave(v, save, 500), {
      initialProps: { v: { bio: "" } },
    });
    // Server responds late: declare its value, then render the form with it.
    result.current.sync({ bio: "loaded" });
    rerender({ v: { bio: "loaded" } });
    vi.advanceTimersByTime(2000);
    expect(save).not.toHaveBeenCalled();

    // A real edit after that still saves.
    rerender({ v: { bio: "loaded!" } });
    vi.advanceTimersByTime(500);
    expect(save).toHaveBeenCalledWith({ bio: "loaded!" });
  });
});
