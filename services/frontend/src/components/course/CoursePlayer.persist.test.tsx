import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import CoursePlayer, { type Step } from "./CoursePlayer";
import { TerminalRunContext, type TerminalRun } from "./terminalRun";

// jsdom doesn't implement Element.scrollTo, which go() calls on every page turn.
beforeEach(() => {
  Element.prototype.scrollTo = vi.fn();
  localStorage.clear();
});
afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const steps: Step[] = [
  { slug: "one", index: 0, kind: "concept", ey: "CONCEPT 1", eyt: "One", title: "Screen One", body: "<p>one</p>" },
  { slug: "two", index: 1, kind: "concept", ey: "CONCEPT 2", eyt: "Two", title: "Screen Two", body: "<p>two</p>" },
  { slug: "three", index: 2, kind: "concept", ey: "CONCEPT 3", eyt: "Three", title: "Screen Three", body: "<p>three</p>" },
];

function renderPlayer(courseKey?: string) {
  const bridge: TerminalRun = { runCommand: vi.fn(), registerTerminal: () => () => {} };
  return render(
    <TerminalRunContext.Provider value={bridge}>
      <CoursePlayer steps={steps} answers={{}} onAnswer={() => {}} moduleName="demo" courseKey={courseKey} />
    </TerminalRunContext.Provider>,
  );
}

describe("CoursePlayer remembers the current page per lab", () => {
  it("persists the current screen to localStorage under the lab's key", async () => {
    renderPlayer("lab-a");
    await userEvent.keyboard("{ArrowRight}{ArrowRight}");
    expect(screen.getByText("Screen Three")).toBeInTheDocument();
    expect(localStorage.getItem("k8labs:course-page:lab-a")).toBe("2");
  });

  it("restores the saved screen on remount for the same lab", () => {
    localStorage.setItem("k8labs:course-page:lab-a", "2");
    renderPlayer("lab-a");
    expect(screen.getByText("Screen Three")).toBeInTheDocument();
  });

  it("keeps each lab's page separate - a different lab starts at its own (unset) page 0", () => {
    localStorage.setItem("k8labs:course-page:lab-a", "2");
    renderPlayer("lab-b");
    expect(screen.getByText("Screen One")).toBeInTheDocument();
  });

  it("clamps a stale saved index if the course now has fewer screens", () => {
    localStorage.setItem("k8labs:course-page:lab-a", "99");
    renderPlayer("lab-a");
    // Last screen, not a crash on an out-of-range index.
    expect(screen.getByText("Screen Three")).toBeInTheDocument();
  });

  it("does not touch localStorage when no courseKey is given", async () => {
    renderPlayer(undefined);
    await userEvent.keyboard("{ArrowRight}");
    expect(screen.getByText("Screen Two")).toBeInTheDocument();
    expect(localStorage.length).toBe(0);
  });
});
