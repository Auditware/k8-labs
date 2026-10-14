import { cleanup, fireEvent, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import DriverPicker from "./DriverPicker";
import type { Driver } from "../lib/api";

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const VERMAGIC = "6.18.35 SMP mod_unload";

const driver = (o: Partial<Driver>): Driver => ({
  id: "x",
  sha256: "a".repeat(64),
  filename: "x.ko",
  sizeBytes: 18432,
  modName: "x",
  vermagic: VERMAGIC,
  license: "GPL v2",
  depends: "",
  description: "",
  isSigned: false,
  status: "approved",
  visibility: "catalog",
  uploadedBy: "u1",
  uploadedAt: "2026-09-10T10:00:00Z",
  ...o,
});

// The picker seeds its list from the ["drivers"] query, so tests prime the cache
// rather than stubbing fetch - the component's contract is what it renders from
// that data, not how the data arrived.
function renderPicker(drivers: Driver[], props: Partial<Parameters<typeof DriverPicker>[0]> = {}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  });
  queryClient.setQueryData(["drivers"], drivers);
  const onChange = vi.fn();
  const utils = render(
    <QueryClientProvider client={queryClient}>
      <DriverPicker selected={[]} onChange={onChange} expectedVermagic={VERMAGIC} {...props} />
    </QueryClientProvider>,
  );
  return { ...utils, onChange };
}

describe("DriverPicker", () => {
  // Vermagic is the only load-time compatibility check on the research kernels
  // (MODVERSIONS is off), so a mismatch is a guaranteed insmod failure. Getting
  // this verdict backwards is worse than showing nothing: it would either hide a
  // usable driver or bless one that cannot load.
  it("flags a driver built against a different kernel as a mismatch", () => {
    renderPicker([
      driver({ id: "ok", modName: "kuaf" }),
      driver({ id: "bad", modName: "stale", vermagic: "6.12.1 SMP" }),
    ]);
    const rows = screen.getAllByRole("row");
    const okRow = rows.find((r) => within(r).queryByText("kuaf"))!;
    const badRow = rows.find((r) => within(r).queryByText("stale"))!;
    expect(within(okRow).getByText("match")).toBeTruthy();
    expect(within(badRow).getByText("mismatch")).toBeTruthy();
  });

  // modinfo prints vermagic with a trailing space and the stored value keeps it,
  // so an exact comparison would mark every legitimate driver as unloadable.
  it("ignores the trailing whitespace modinfo emits", () => {
    renderPicker([driver({ id: "ok", modName: "kuaf", vermagic: `${VERMAGIC} ` })]);
    expect(screen.getByText("match")).toBeTruthy();
    expect(screen.queryByText("mismatch")).toBeNull();
  });

  // A wrong verdict is worse than no verdict, so with nothing authoritative to
  // compare against the column is dropped rather than guessed.
  it("hides the compatibility column when the lab reports no expected vermagic", () => {
    renderPicker([driver({ id: "ok", modName: "kuaf" })], { expectedVermagic: undefined });
    expect(screen.queryByText("match")).toBeNull();
    expect(screen.queryByText("mismatch")).toBeNull();
    expect(screen.getByText("kuaf")).toBeTruthy();
  });

  // The denial reason is the only channel telling a researcher why their upload
  // was rejected. It is in the payload already; failing to render it sends them
  // to support for something the API answered in full.
  it("shows the admin's full reason for a denied upload", () => {
    const reason = "Built against 6.12.1; the research guest runs 6.18.35. Rebuild and re-upload.";
    renderPicker([driver({ id: "d", modName: "broken", status: "denied", vetReason: reason })]);
    expect(screen.getByText("denied")).toBeTruthy();
    expect(screen.getByText(reason)).toBeTruthy();
  });

  // Unapproved modules cannot be mounted (the backend refuses to resolve them),
  // so offering a checkbox for one would promise something the session won't do.
  it("does not offer pending or denied uploads for selection", async () => {
    renderPicker([
      driver({ id: "ok", modName: "kuaf" }),
      driver({ id: "p", modName: "waiting", status: "pending" }),
      driver({ id: "n", modName: "broken", status: "denied" }),
    ]);
    expect(screen.getAllByRole("checkbox")).toHaveLength(1);
  });

  // fireEvent rather than userEvent here: userEvent's full pointer sequence does
  // not produce a React change event for a CONTROLLED checkbox under jsdom (the
  // value tracker never sees the flip, because `checked` is pinned by the prop
  // and the parent is a spy that never updates it). Verified in a real browser
  // that clicking the box, its label, and any other cell in the row all toggle;
  // fireEvent.click is what reproduces that here, and it still goes through
  // React's own handler rather than calling the prop directly.
  it("reports the toggled driver to its parent", () => {
    const { onChange } = renderPicker([driver({ id: "ok", modName: "kuaf" })]);
    fireEvent.click(screen.getByRole("checkbox"));
    expect(onChange).toHaveBeenCalledWith(["ok"]);
  });

  // The whole row is a click target, so this guards the other half of that: the
  // row handler and the checkbox must not both fire and cancel each other out.
  it("toggles from a click anywhere else in the row, exactly once", () => {
    const { onChange } = renderPicker([driver({ id: "ok", modName: "kuaf" })]);
    fireEvent.click(screen.getByText("match"));
    expect(onChange).toHaveBeenCalledTimes(1);
    expect(onChange).toHaveBeenCalledWith(["ok"]);
  });

  it("deselects a driver that is already selected", () => {
    const { onChange } = renderPicker([driver({ id: "ok", modName: "kuaf" })], { selected: ["ok"] });
    fireEvent.click(screen.getByRole("checkbox"));
    expect(onChange).toHaveBeenCalledWith([]);
  });

  // Drivers are mounted at provision time, so a selection change after the
  // session exists would silently do nothing - the lock has to actually hold.
  it("accepts no selection changes once the session is running", () => {
    const { onChange } = renderPicker([driver({ id: "ok", modName: "kuaf" })], { disabled: true });
    const box = screen.getByRole("checkbox") as HTMLInputElement;
    expect(box.disabled).toBe(true);
    fireEvent.click(box);
    fireEvent.click(screen.getByText("match"));
    expect(onChange).not.toHaveBeenCalled();
  });

  // Regression: the mount column used to be named with an `sr-only` span, which
  // Tailwind implements as `position: absolute`. With no positioned ancestor it
  // escaped the table, landed below the fold, and extended the document - making
  // the whole PAGE scrollable, so scrolling the research guide dragged the entire
  // app upward. Only the research labs render this component, which is why it
  // looked like a research-only layout bug. The column keeps its accessible name
  // via aria-label instead, so there is no positioned element to escape.
  it("names the mount column without an absolutely positioned sr-only element", () => {
    const { container } = renderPicker([driver({ id: "ok", modName: "kuaf" })]);
    expect(container.querySelector(".sr-only")).toBeNull();
    expect(screen.getByRole("columnheader", { name: "mount" })).toBeTruthy();
  });

  it("points a user with an empty catalog at the sample driver", () => {
    renderPicker([]);
    expect(screen.getByText(/No vetted drivers yet/)).toBeTruthy();
  });
});
