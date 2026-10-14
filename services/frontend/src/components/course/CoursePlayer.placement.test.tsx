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

// A minimal course ending in a flag capstone, the shape every real course has:
// the last screen is the "Solved / what this proves" step.
const steps: Step[] = [
  { slug: "start", index: 0, kind: "concept", ey: "START", eyt: "Intro", title: "Screen One", body: "<p>one</p>" },
  { slug: "mid", index: 1, kind: "concept", ey: "MID", eyt: "Middle", title: "Screen Two", body: "<p>two</p>" },
  {
    slug: "flag",
    index: 2,
    kind: "flag",
    ey: "CAPTURE",
    eyt: "The flag",
    title: "Submit what you captured",
    body: "<p>paste it</p>",
  },
];

function renderPlayer(props: Partial<React.ComponentProps<typeof CoursePlayer>> = {}) {
  const bridge: TerminalRun = { runCommand: vi.fn(), registerTerminal: () => () => {} };
  return render(
    <TerminalRunContext.Provider value={bridge}>
      <CoursePlayer steps={steps} answers={{}} onAnswer={() => {}} moduleName="demo" courseKey="lab-a" {...props} />
    </TerminalRunContext.Provider>,
  );
}

describe("CoursePlayer opening placement for a solved lab", () => {
  it("opens a solved lab on the final capstone, ignoring the saved page", () => {
    localStorage.setItem("k8labs:course-page:lab-a", "0"); // last left on the intro
    renderPlayer({ complete: true, progressReady: true });
    // The solved panel only renders on the flag capstone (the last screen).
    expect(screen.getByText(/solved - demo complete/i)).toBeInTheDocument();
  });

  it("does not jump until completion has actually loaded (progressReady)", () => {
    // complete is the pre-fetch default here, so we must not strand the reader.
    renderPlayer({ complete: true, progressReady: false });
    expect(screen.getByText("Screen One")).toBeInTheDocument();
    expect(screen.queryByText(/solved - demo complete/i)).not.toBeInTheDocument();
  });

  it("places on the capstone once progress lands after mount", () => {
    const { rerender } = renderPlayer({ complete: true, progressReady: false });
    expect(screen.getByText("Screen One")).toBeInTheDocument();

    const bridge: TerminalRun = { runCommand: vi.fn(), registerTerminal: () => () => {} };
    rerender(
      <TerminalRunContext.Provider value={bridge}>
        <CoursePlayer
          steps={steps}
          answers={{}}
          onAnswer={() => {}}
          moduleName="demo"
          courseKey="lab-a"
          complete
          progressReady
        />
      </TerminalRunContext.Provider>,
    );
    expect(screen.getByText(/solved - demo complete/i)).toBeInTheDocument();
  });

  it("leaves an unsolved lab on its restored page", () => {
    localStorage.setItem("k8labs:course-page:lab-a", "1");
    renderPlayer({ complete: false, progressReady: true });
    expect(screen.getByText("Screen Two")).toBeInTheDocument();
  });
});

describe("CoursePlayer restart resets the view", () => {
  it("returns to the first screen and forgets the saved page on restart", async () => {
    const onReset = vi.fn();
    renderPlayer({ complete: true, progressReady: true, onReset });
    // Opens on the capstone; the two-click restart lives there.
    await userEvent.click(screen.getByRole("button", { name: "↻ Restart lab" }));
    await userEvent.click(screen.getByRole("button", { name: /click again to clear/i }));

    expect(onReset).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Screen One")).toBeInTheDocument();
    // The saved page no longer points at the capstone we just cleared.
    expect(localStorage.getItem("k8labs:course-page:lab-a")).not.toBe("2");
  });
});
