import { describe, expect, it } from "vitest";

import { formatSolve, joinedAgo, topPercentile } from "./format";

describe("formatSolve", () => {
  it("renders sub-hour times as m:ss", () => {
    expect(formatSolve(252)).toBe("4:12");
    expect(formatSolve(59)).toBe("0:59");
    expect(formatSolve(600)).toBe("10:00");
  });
  it("renders an hour or more as h mm", () => {
    expect(formatSolve(3661)).toBe("1h 01m");
    expect(formatSolve(7320)).toBe("2h 02m");
  });
});

describe("topPercentile", () => {
  it("is the rank as a share of the board", () => {
    expect(topPercentile(1, 4)).toBe("25%");
    expect(topPercentile(2, 4)).toBe("50%");
  });
  it("keeps one decimal for elite fractions", () => {
    expect(topPercentile(1, 1000)).toBe("0.1%");
  });
  it("is null for a board of one, where it means nothing", () => {
    expect(topPercentile(1, 1)).toBeNull();
    expect(topPercentile(1, 0)).toBeNull();
  });
});

describe("joinedAgo", () => {
  const daysAgo = (n: number) => new Date(Date.now() - n * 86_400_000).toISOString();

  it("uses the coarsest honest unit", () => {
    expect(joinedAgo(daysAgo(0))).toBe("joined today");
    expect(joinedAgo(daysAgo(5))).toBe("joined 5 days ago");
    expect(joinedAgo(daysAgo(70))).toBe("joined 2 months ago");
    expect(joinedAgo(daysAgo(400))).toBe("joined 1 year ago");
  });
  it("is empty for an unparseable timestamp", () => {
    expect(joinedAgo("not-a-date")).toBe("");
  });
});
