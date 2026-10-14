import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { AuthProvider } from "./context/AuthContext";
import App from "./App";

const modules = [
  {
    slug: "ns-escape",
    displayName: "Namespace Escape 101",
    description: "Read a Secret in a namespace you were never meant to reach.",
    hint: "Enumerate what your service account can do.",
    nested: { entryPort: 0 },
  },
];

const loggedInUser = {
  id: "u1",
  email: "test@example.com",
  username: "testuser",
  oauthProvider: "google",
};

function mockFetch(meResponse: { status: number; body: unknown }) {
  vi.stubGlobal(
    "fetch",
    vi.fn((url: string) => {
      if (url.toString().includes("/api/v1/auth/me")) {
        return Promise.resolve({
          ok: meResponse.status === 200,
          status: meResponse.status,
          json: () => Promise.resolve(meResponse.body),
        });
      }
      // The landing's track-record deck fetches the public operator showcase;
      // an empty board is a valid state and keeps these routing tests focused.
      if (url.toString().includes("/api/v1/operators")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve([]),
        });
      }
      // The sidebar's leaderboard target queries the board; an unauthenticated,
      // empty board renders nothing and keeps these routing tests focused.
      if (url.toString().includes("/api/v1/leaderboard")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () =>
            Promise.resolve({
              entries: [],
              revealed: 0,
              locked: true,
              viewer: { authenticated: false, hasClearedLab: false },
            }),
        });
      }
      // The catalog (/api/v1/modules): a real Response carries headers, and
      // getModulesWithMeta reads X-Modules-Locked off them. Give the mock a
      // headers.get so the sidebar's locked-teaser signal resolves (none here).
      return Promise.resolve({
        ok: true,
        status: 200,
        headers: new Headers(),
        json: () => Promise.resolve(modules),
      });
    }) as unknown as typeof fetch,
  );
}

function renderApp(initialPath = "/") {
  const queryClient = new QueryClient();
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={[initialPath]}>
        <AuthProvider>
          <App />
        </AuthProvider>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("App", () => {
  afterEach(cleanup);

  it("redirects the labs home to the first entitled lab when signed in", async () => {
    mockFetch({ status: 200, body: loggedInUser });
    renderApp("/labs");
    expect(await screen.findByRole("heading", { name: "K8 Labs" })).toBeInTheDocument();
    expect(
      await screen.findByRole("heading", { name: "Namespace Escape 101", level: 2 }),
    ).toBeInTheDocument();
  });

  it("shows the marketing landing at / - not a lab - even when signed out", async () => {
    mockFetch({ status: 401, body: null });
    renderApp("/");
    expect(
      await screen.findByRole("heading", { name: /cloud security research/i }),
    ).toBeInTheDocument();
    expect(screen.queryByRole("heading", { name: "Namespace Escape 101" })).toBeNull();
    // Providers live behind the CTA (on /login), not on the landing itself.
    expect(screen.queryByText("Continue with Google")).toBeNull();
  });

  it("routes the entry-challenge CTA to the sign-in page", async () => {
    mockFetch({ status: 401, body: null });
    renderApp("/");
    const cta = (
      await screen.findAllByRole("button", { name: /can you solve the entry lab/i })
    )[0];
    fireEvent.click(cta);
    // /login is the single sign-in surface - no in-page modal.
    expect(await screen.findByText("Continue with Google")).toBeInTheDocument();
    expect(screen.getByText("Continue with GitHub")).toBeInTheDocument();
  });
});
