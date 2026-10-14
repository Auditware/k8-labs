import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import CoursePlayer, { type Step } from "./CoursePlayer";
import { TerminalRunContext, type TerminalRun } from "./terminalRun";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// A single concept step whose server-rendered body carries one runnable-command
// chip (as prose.go emits it) and one plain term chip.
const steps: Step[] = [
  {
    slug: "enumerate",
    index: 0,
    kind: "concept",
    ey: "CONCEPT 1",
    eyt: "Enumerate",
    title: "Ask the cluster",
    body:
      '<p>Run <span class="k cmd" data-cmd="kubectl auth can-i --list">kubectl auth can-i --list</span> ' +
      'and read your <span class="k">token</span>.</p>',
  },
];

function renderPlayer(runCommand = vi.fn()) {
  const bridge: TerminalRun = { runCommand, registerTerminal: () => () => {} };
  render(
    <TerminalRunContext.Provider value={bridge}>
      <CoursePlayer steps={steps} answers={{}} onAnswer={() => {}} moduleName="ns-escape" />
    </TerminalRunContext.Provider>,
  );
  return { runCommand };
}

describe("CoursePlayer runnable-command chips", () => {
  it("adds copy + run affordances to a .cmd chip but not to a plain term", async () => {
    renderPlayer();
    // The buttons are added in a post-paint effect, so wait for them.
    expect(await screen.findByRole("button", { name: "Run in the lab terminal" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Copy command" })).toBeInTheDocument();
    // Exactly one command chip got enhanced (the plain `token` term did not).
    expect(screen.getAllByRole("button", { name: "Run in the lab terminal" })).toHaveLength(1);
  });

  it("runs the command through the terminal bridge, verbatim", async () => {
    const { runCommand } = renderPlayer();
    await userEvent.click(await screen.findByRole("button", { name: "Run in the lab terminal" }));
    expect(runCommand).toHaveBeenCalledWith("kubectl auth can-i --list");
  });

  it("copies the command to the clipboard", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, "clipboard", { value: { writeText }, configurable: true });
    renderPlayer();
    await userEvent.click(await screen.findByRole("button", { name: "Copy command" }));
    expect(writeText).toHaveBeenCalledWith("kubectl auth can-i --list");
  });
});
