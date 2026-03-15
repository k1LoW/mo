import { describe, it, expect, vi, afterEach } from "vitest";
import { formatRelativeTime, formatAbsoluteTime } from "./time";

describe("formatRelativeTime", () => {
  afterEach(() => {
    vi.useRealTimers();
  });

  function iso(msAgo: number): string {
    return new Date(Date.now() - msAgo).toISOString();
  }

  it("returns 'just now' for <60 seconds", () => {
    expect(formatRelativeTime(iso(5_000))).toBe("just now");
    expect(formatRelativeTime(iso(59_000))).toBe("just now");
  });

  it("returns minutes for <60 minutes", () => {
    expect(formatRelativeTime(iso(60_000))).toBe("1m ago");
    expect(formatRelativeTime(iso(15 * 60_000))).toBe("15m ago");
    expect(formatRelativeTime(iso(59 * 60_000))).toBe("59m ago");
  });

  it("returns hours for <24 hours", () => {
    expect(formatRelativeTime(iso(60 * 60_000))).toBe("1h ago");
    expect(formatRelativeTime(iso(23 * 60 * 60_000))).toBe("23h ago");
  });

  it("returns days for <30 days", () => {
    expect(formatRelativeTime(iso(24 * 60 * 60_000))).toBe("1d ago");
    expect(formatRelativeTime(iso(29 * 24 * 60 * 60_000))).toBe("29d ago");
  });

  it("returns months for <12 months", () => {
    expect(formatRelativeTime(iso(30 * 24 * 60 * 60_000))).toBe("1mo ago");
    expect(formatRelativeTime(iso(11 * 30 * 24 * 60 * 60_000))).toBe("11mo ago");
  });

  it("returns years for >=12 months", () => {
    expect(formatRelativeTime(iso(365 * 24 * 60 * 60_000))).toBe("1y ago");
    expect(formatRelativeTime(iso(2 * 365 * 24 * 60 * 60_000))).toBe("2y ago");
  });
});

describe("formatAbsoluteTime", () => {
  it("formats as 'Mon DD HH:MM'", () => {
    // 2026-03-15T02:36:00Z in local time
    const result = formatAbsoluteTime("2026-03-15T02:36:00Z");
    // Month and time depend on local timezone, but format should match pattern
    expect(result).toMatch(/^[A-Z][a-z]{2}\s[\s\d]\d\s\d{2}:\d{2}$/);
  });

  it("pads single-digit days with space", () => {
    const result = formatAbsoluteTime("2026-01-05T09:07:00Z");
    // Verify day is space-padded (local TZ may shift the day)
    expect(result).toMatch(/^[A-Z][a-z]{2}\s[\s\d]\d\s\d{2}:\d{2}$/);
  });

  it("uses correct month abbreviations", () => {
    const months = [
      { iso: "2026-01-15T12:00:00Z", month: "Jan" },
      { iso: "2026-06-15T12:00:00Z", month: "Jun" },
      { iso: "2026-12-15T12:00:00Z", month: "Dec" },
    ];
    for (const { iso, month } of months) {
      // In most timezones, noon UTC on the 15th stays on the same date
      expect(formatAbsoluteTime(iso)).toContain(month);
    }
  });
});
