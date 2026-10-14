import { describe, it, expect } from "vitest";
import { provisionHud } from "./ProvisioningLoader";

// provisionHud is the pure bar/label/ETA decision behind the provisioning screen.
// The rAF draw that consumes it can't be asserted in jsdom (no canvas), so the
// honesty guarantees the whole rework exists for are pinned here instead.
const EST = 42;

describe("provisionHud", () => {
  it("anchors the bar to the reported backend milestone, not the clock", () => {
    // A brand-new session sitting in the microVM-boot gate: barely any time in
    // phase, so the bar should sit right at that phase's earned floor (0.05),
    // regardless of how much wall-clock has passed.
    const early = provisionHud({ reduce: false, phase: "pod-created", elapsedSec: 1, phaseElapsedSec: 0, estSec: EST });
    expect(early.pct).toBeCloseTo(0.05, 2);
    expect(early.label).toBe("Booting your isolated microVM");

    // Same phase, but the clock has run way past the whole estimate (busy hours).
    // The bar must NOT sprint ahead - it stays inside this phase's band, capped
    // below the next milestone's floor (0.49).
    const slow = provisionHud({ reduce: false, phase: "pod-created", elapsedSec: 120, phaseElapsedSec: 60, estSec: EST });
    expect(slow.pct).toBeGreaterThan(0.05);
    expect(slow.pct).toBeLessThan(0.49);
  });

  it("advances the bar when a later milestone arrives", () => {
    const boot = provisionHud({ reduce: false, phase: "pod-created", elapsedSec: 8, phaseElapsedSec: 6, estSec: EST });
    const foothold = provisionHud({ reduce: false, phase: "manifests-applied", elapsedSec: 20, phaseElapsedSec: 0, estSec: EST });
    // Reaching the foothold gate jumps the bar to its floor (0.60), well past
    // wherever the boot phase had crept to.
    expect(foothold.pct).toBeGreaterThan(boot.pct);
    expect(foothold.pct).toBeCloseTo(0.6, 2);
    expect(foothold.label).toBe("Bringing your foothold online");
  });

  it("never renders as 100% while still provisioning", () => {
    // Even at the last pre-running gate, held long, the bar caps below full.
    const done = provisionHud({ reduce: false, phase: "foothold-ready", elapsedSec: 30, phaseElapsedSec: 20, estSec: EST });
    expect(Math.min(96, done.pct * 100)).toBeLessThanOrEqual(96);
    expect(done.pct).toBeLessThan(1);
  });

  it("counts the estimate down while under way", () => {
    expect(provisionHud({ reduce: false, phase: undefined, elapsedSec: 5, phaseElapsedSec: 5, estSec: EST }).eta).toMatch(/^~\d+s$/);
  });

  it("says 'taking longer than usual' only when stuck EARLY, not near the end", () => {
    // Stuck in an early gate (microVM boot, ceil 0.34) well past the estimate:
    // low real progress + lots of elapsed = something's genuinely wrong, so say so.
    const stuckEarly = provisionHud({ reduce: false, phase: "pod-created", elapsedSec: 120, phaseElapsedSec: 120, estSec: EST });
    expect(stuckEarly.pct).toBeLessThan(0.9);
    expect(stuckEarly.eta).toBe("taking longer than usual…");

    // Stuck late (bar already ~done) past the estimate: "almost there" is the
    // honest read, not an alarm - the pct>=0.9 rule keeps it calm.
    const stuckLate = provisionHud({ reduce: false, phase: "foothold-ready", elapsedSec: 120, phaseElapsedSec: 120, estSec: EST });
    expect(stuckLate.eta).toBe("almost there…");
  });

  it("says 'almost there' once the real foothold gate lands, even early", () => {
    // foothold-ready floor is 0.97 >= 0.9, so the ETA should flip immediately
    // rather than showing a misleading '~20s' from a wide estimate.
    const eta = provisionHud({ reduce: false, phase: "foothold-ready", elapsedSec: 15, phaseElapsedSec: 0, estSec: EST }).eta;
    expect(eta).toBe("almost there…");
  });

  it("falls back to the time estimate before any milestone is reported", () => {
    // Undefined phase (first sub-second, or a legacy module): the bar is driven by
    // elapsed/estimate, so it grows with the clock.
    const t0 = provisionHud({ reduce: false, phase: undefined, elapsedSec: 1, phaseElapsedSec: 1, estSec: EST });
    const t1 = provisionHud({ reduce: false, phase: undefined, elapsedSec: 15, phaseElapsedSec: 15, estSec: EST });
    expect(t1.pct).toBeGreaterThan(t0.pct);
  });
});
