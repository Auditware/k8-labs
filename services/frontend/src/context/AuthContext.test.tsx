import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuthProvider, useAuth } from "./AuthContext";

// Drive the probe directly at the api boundary so we control reachable vs.
// unreachable without a 6s real timeout, and fast-forward the retry backoff.
vi.mock("../lib/api", () => ({
  getMe: vi.fn(),
  logout: vi.fn(() => Promise.resolve()),
}));
import { getMe } from "../lib/api";
const mockGetMe = getMe as unknown as ReturnType<typeof vi.fn>;

function Probe() {
  const { serverDown, isLoading } = useAuth();
  return <div data-testid="state">{isLoading ? "loading" : serverDown ? "down" : "ok"}</div>;
}

const renderAuth = () =>
  render(
    <AuthProvider>
      <Probe />
    </AuthProvider>,
  );

describe("AuthContext recovery", () => {
  afterEach(() => {
    vi.useRealTimers();
    cleanup();
    mockGetMe.mockReset();
  });

  it("clears the stale server-down banner when the backend recovers", async () => {
    vi.useFakeTimers();
    // Initial probe: unreachable on every attempt -> latches serverDown.
    mockGetMe.mockRejectedValue(new Error("network"));
    renderAuth();
    // Ride through the retry backoff (PROBE_RETRY_DELAYS = 1000 + 2000).
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });
    expect(screen.getByTestId("state").textContent).toBe("down");

    // Backend comes back: a reachable 401 (getMe -> null). A tab focus should
    // re-probe and clear the banner on its own, no reload required.
    mockGetMe.mockResolvedValue(null);
    await act(async () => {
      window.dispatchEvent(new Event("focus"));
    });
    expect(screen.getByTestId("state").textContent).toBe("ok");
  });

  it("stays down while the server is still unreachable on re-probe", async () => {
    vi.useFakeTimers();
    mockGetMe.mockRejectedValue(new Error("network"));
    renderAuth();
    await act(async () => {
      await vi.advanceTimersByTimeAsync(3100);
    });
    expect(screen.getByTestId("state").textContent).toBe("down");

    // Fallback interval fires but the server is still down -> banner persists.
    await act(async () => {
      await vi.advanceTimersByTimeAsync(15000);
    });
    expect(screen.getByTestId("state").textContent).toBe("down");
  });
});
