import { describe, it, expect, vi, afterEach } from "vitest";
import { createSession, type StartSessionError } from "./api";

// createSession is where a lab start's failures get turned into what the student
// reads. The point of these tests is that raw plumbing never leaks: not the
// browser's "Failed to fetch" (a rejected fetch), not "failed to start session:
// 500" (a 5xx the proxy returns when the backend is down or erroring) - while the
// backend's own actionable messages (conflict, quota) still pass through intact.

function stubFetch(impl: () => Promise<unknown> | never) {
  vi.stubGlobal("fetch", vi.fn(impl) as unknown as typeof fetch);
}
const resp = (status: number, body: unknown) => ({
  ok: status >= 200 && status < 300,
  status,
  json: () => (body === undefined ? Promise.reject(new Error("no body")) : Promise.resolve(body)),
});
// Await a rejection and hand back the typed error (a plain .catch widens the type
// to Session | StartSessionError, which won't let us read .message/.reason).
async function rejectionOf(p: Promise<unknown>): Promise<StartSessionError> {
  try {
    await p;
  } catch (e) {
    return e as StartSessionError;
  }
  throw new Error("expected createSession to reject but it resolved");
}

afterEach(() => vi.unstubAllGlobals());

describe("createSession error messages", () => {
  it("turns a network rejection into a reachability message, not 'Failed to fetch'", async () => {
    stubFetch(() => Promise.reject(new TypeError("Failed to fetch")));
    const err = await rejectionOf(createSession("ns-escape"));
    expect(err.message).not.toMatch(/failed to fetch/i);
    expect(err.message).toMatch(/reach the server/i);
    expect(err.reason).toBe("network");
  });

  it("turns a 5xx with no body into a calm server-error message, not 'failed to start session: 500'", async () => {
    stubFetch(() => Promise.resolve(resp(500, undefined)));
    const err = await rejectionOf(createSession("ns-escape"));
    expect(err.message).not.toMatch(/500/);
    expect(err.message).toMatch(/something went wrong/i);
    expect(err.reason).toBe("server_error"); // drives the "contact us" affordance
  });

  it("passes the backend's own actionable message straight through", async () => {
    stubFetch(() =>
      Promise.resolve(
        resp(409, {
          error: "You already have a lab running.",
          reason: "user_session_limit",
          conflictSessionId: "abc123",
          conflictModule: "suid-privesc",
        }),
      ),
    );
    const err = await rejectionOf(createSession("ns-escape"));
    expect(err.message).toBe("You already have a lab running.");
    expect(err.reason).toBe("user_session_limit");
    expect(err.conflictSessionId).toBe("abc123");
    expect(err.conflictModule).toBe("suid-privesc");
  });

  it("returns the session on success", async () => {
    stubFetch(() => Promise.resolve(resp(200, { id: "sess1", moduleSlug: "ns-escape", status: "starting" })));
    const s = await createSession("ns-escape");
    expect(s.id).toBe("sess1");
  });
});
