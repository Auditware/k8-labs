import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import LabAdminPage from "./LabAdminPage";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

// retry: false so a failing query settles into `error` immediately. Without it
// react-query's default backoff leaves the query pending for seconds, which is
// exactly the window the isPending/isError branch has to cover - and a test
// that waits it out is a test nobody will keep running.
function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <LabAdminPage />
    </QueryClientProvider>,
  );
}

const settings = [
  { moduleSlug: "ns-escape", durationMinutes: 60, idleMinutes: 20, isDefault: true },
  {
    moduleSlug: "custom-lab",
    durationMinutes: 45,
    idleMinutes: 15,
    isDefault: false,
    updatedAt: "2026-07-20T10:00:00Z",
  },
];

describe("LabAdminPage", () => {
  it("renders a row per lab and distinguishes default from customised", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify(settings), { status: 200 })),
    );
    renderPage();

    expect(await screen.findByText("ns-escape")).toBeInTheDocument();
    // An admin has to be able to tell "nobody configured this" from "someone
    // chose 60" - otherwise the defaults look like decisions.
    expect(screen.getByText("using the built-in default")).toBeInTheDocument();
    expect(screen.getByText(/customised/)).toBeInTheDocument();
  });

  it("surfaces the API's own reason when a save is rejected", async () => {
    // The backend refuses an idle timeout above the duration, because it could
    // never fire - and says so in plain text. That reason is far more useful
    // than a status code, so it must reach the screen.
    const reason = "idle timeout (90m) cannot exceed the session duration (60m)";
    vi.stubGlobal(
      "fetch",
      vi.fn(async (_url: string, init?: RequestInit) => {
        if (init?.method === "PUT") return new Response(reason, { status: 400 });
        return new Response(JSON.stringify(settings), { status: 200 });
      }),
    );
    renderPage();

    const idle = await screen.findByLabelText(/Idle timeout for ns-escape/);
    await userEvent.clear(idle);
    await userEvent.type(idle, "90");
    await userEvent.click(screen.getAllByRole("button", { name: "Save" })[0]);

    expect(await screen.findByText(reason)).toBeInTheDocument();
  });

  it("shows a loading state rather than a blank page while the query is unsettled", async () => {
    // Never resolves - this is the state an isLoading/isError pair renders
    // blank during react-query's retry backoff.
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
    renderPage();
    expect(await screen.findByText("Loading…")).toBeInTheDocument();
  });

  it("reports a failure instead of an empty table", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("nope", { status: 500 })));
    renderPage();
    await waitFor(() => expect(screen.getByText(/Failed to load lab settings/)).toBeInTheDocument());
    expect(screen.queryByText("Loading…")).not.toBeInTheDocument();
  });

  it("only offers Reset on a lab that has been customised", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn(async () => new Response(JSON.stringify(settings), { status: 200 })),
    );
    renderPage();

    await screen.findByText("ns-escape");
    const resets = screen.getAllByRole("button", { name: "Reset" });
    // Row order matches the fixture: default first, customised second.
    expect(resets[0]).toBeDisabled();
    expect(resets[1]).toBeEnabled();
  });
});
