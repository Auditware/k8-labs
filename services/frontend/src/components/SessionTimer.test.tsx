import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import SessionTimer from "./SessionTimer";

afterEach(() => {
  cleanup();
  vi.useRealTimers();
});

// Fixed "now" so the countdown is deterministic. The component reads Date.now()
// on mount and on an interval, both of which fake timers control.
const NOW = new Date("2026-07-21T12:00:00Z");
const inMinutes = (m: number) => new Date(NOW.getTime() + m * 60_000).toISOString();

function renderAt(props: {
  expiresAt?: string;
  idleExpiresAt?: string;
  onExtend?: () => void;
  extending?: boolean;
}) {
  vi.useFakeTimers();
  vi.setSystemTime(NOW);
  return render(<SessionTimer {...props} />);
}

describe("SessionTimer", () => {
  it("counts down to the server's hard cap", () => {
    renderAt({ expiresAt: inMinutes(45), idleExpiresAt: inMinutes(20) });
    expect(screen.getByText("45:00 left")).toBeInTheDocument();
  });

  it("switches to h:mm:ss past an hour", () => {
    renderAt({ expiresAt: inMinutes(90), idleExpiresAt: inMinutes(20) });
    expect(screen.getByText("1:30:00 left")).toBeInTheDocument();
  });

  // The whole reason both deadlines are sent: a session dies to whichever
  // fires first, so a student about to be reaped for inactivity must be told,
  // even with plenty of time left on the main clock.
  it("warns when the idle deadline is imminent and will fire first", () => {
    renderAt({ expiresAt: inMinutes(40), idleExpiresAt: inMinutes(2) });
    expect(screen.getByText("40:00 left")).toBeInTheDocument();
    expect(screen.getByText("idle 02:00")).toBeInTheDocument();
  });

  // Near the end of a lab the hard cap kills the session anyway, so an idle
  // warning would be noise rather than information.
  it("does not warn when the hard cap will fire first", () => {
    renderAt({ expiresAt: inMinutes(1), idleExpiresAt: inMinutes(2) });
    expect(screen.queryByText(/^idle /)).not.toBeInTheDocument();
  });

  it("clamps a past deadline to 'expired' rather than a negative countdown", () => {
    renderAt({ expiresAt: inMinutes(-5), idleExpiresAt: inMinutes(-5) });
    expect(screen.getByText("expired")).toBeInTheDocument();
  });

  // A missing or malformed timestamp must render nothing at all - "NaN:NaN"
  // on a countdown reads as the session being broken.
  it("renders nothing without a usable expiry", () => {
    const { container } = renderAt({});
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing for an unparseable expiry", () => {
    const { container } = renderAt({ expiresAt: "not-a-date" });
    expect(container).toBeEmptyDOMElement();
  });

  // The extend affordance is opt-in (research labs only): no onExtend, no button.
  it("shows no extend button without an onExtend handler", () => {
    renderAt({ expiresAt: inMinutes(45) });
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders the extend button and fires onExtend when clicked", () => {
    const onExtend = vi.fn();
    renderAt({ expiresAt: inMinutes(45), onExtend });
    const btn = screen.getByRole("button", { name: "+2h" });
    fireEvent.click(btn);
    expect(onExtend).toHaveBeenCalledOnce();
  });

  it("shows a pending label and disables the button while extending", () => {
    const onExtend = vi.fn();
    renderAt({ expiresAt: inMinutes(45), onExtend, extending: true });
    const btn = screen.getByRole("button", { name: "extending…" });
    expect(btn).toBeDisabled();
  });

  // Once expired the session is being reaped; extending it is meaningless, so the
  // button is gone even if a handler is passed.
  it("hides the extend button once expired", () => {
    renderAt({ expiresAt: inMinutes(-1), onExtend: vi.fn() });
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });
});
