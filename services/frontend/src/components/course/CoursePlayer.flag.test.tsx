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

// A minimal course ending in a flag capstone, the shape every real course now has.
const steps: Step[] = [
  { slug: "start", index: 0, kind: "concept", ey: "START", eyt: "Intro", title: "Screen One", body: "<p>one</p>" },
  {
    slug: "flag",
    index: 1,
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
      <CoursePlayer
        steps={steps}
        answers={{}}
        onAnswer={() => {}}
        moduleName="demo"
        {...props}
      />
    </TerminalRunContext.Provider>,
  );
}

describe("CoursePlayer flag capstone", () => {
  it("submits the flag and shows the not-quite hint on a wrong value", async () => {
    const onSubmitFlag = vi.fn().mockResolvedValue({ correct: false, complete: false });
    renderPlayer({ onSubmitFlag });
    await userEvent.keyboard("{ArrowRight}"); // advance to the flag step

    // userEvent treats "{" as a special-key delimiter, so "{{" types a literal one.
    await userEvent.type(screen.getByPlaceholderText("k8labs{...}"), "k8labs{{wrong}");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));

    expect(onSubmitFlag).toHaveBeenCalledWith("k8labs{wrong}");
    expect(await screen.findByText(/not quite/i)).toBeInTheDocument();
  });

  it("clears the wrong hint as soon as the field is edited again", async () => {
    const onSubmitFlag = vi.fn().mockResolvedValue({ correct: false, complete: false });
    renderPlayer({ onSubmitFlag });
    await userEvent.keyboard("{ArrowRight}");

    await userEvent.type(screen.getByPlaceholderText("k8labs{...}"), "nope");
    await userEvent.click(screen.getByRole("button", { name: "Submit" }));
    expect(await screen.findByText(/not quite/i)).toBeInTheDocument();

    await userEvent.type(screen.getByPlaceholderText("k8labs{...}"), "x");
    expect(screen.queryByText(/not quite/i)).not.toBeInTheDocument();
  });

  it("shows the solved panel and no form when complete", async () => {
    renderPlayer({ complete: true });
    await userEvent.keyboard("{ArrowRight}");

    expect(screen.getByText(/solved - demo complete/i)).toBeInTheDocument();
    expect(screen.queryByPlaceholderText("k8labs{...}")).not.toBeInTheDocument();
  });

  it("requires two clicks to restart, then calls onReset", async () => {
    const onReset = vi.fn();
    renderPlayer({ onReset });
    await userEvent.keyboard("{ArrowRight}");

    await userEvent.click(screen.getByRole("button", { name: "↻ Restart lab" }));
    expect(onReset).not.toHaveBeenCalled(); // first click only arms it

    await userEvent.click(screen.getByRole("button", { name: /click again to clear/i }));
    expect(onReset).toHaveBeenCalledTimes(1);
  });

  it("does not fire onReset if the armed restart is cancelled", async () => {
    const onReset = vi.fn();
    renderPlayer({ onReset });
    await userEvent.keyboard("{ArrowRight}");

    await userEvent.click(screen.getByRole("button", { name: "↻ Restart lab" }));
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(onReset).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "↻ Restart lab" })).toBeInTheDocument();
  });
});
