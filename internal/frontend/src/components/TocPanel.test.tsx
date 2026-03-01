import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TocPanel } from "./TocPanel";
import type { TocHeading } from "./TocPanel";

const headings: TocHeading[] = [
  { id: "intro", text: "Introduction", level: 1 },
  { id: "setup", text: "Setup", level: 2 },
  { id: "config", text: "Configuration", level: 3 },
  { id: "usage", text: "Usage", level: 2 },
];

beforeEach(() => {
  localStorage.clear();
});

describe("TocPanel", () => {
  it("renders all headings", () => {
    render(
      <TocPanel headings={headings} activeHeadingId={null} onHeadingClick={() => {}} />,
    );
    expect(screen.getByText("Introduction")).toBeInTheDocument();
    expect(screen.getByText("Setup")).toBeInTheDocument();
    expect(screen.getByText("Configuration")).toBeInTheDocument();
    expect(screen.getByText("Usage")).toBeInTheDocument();
  });

  it("shows 'No headings' when list is empty", () => {
    render(
      <TocPanel headings={[]} activeHeadingId={null} onHeadingClick={() => {}} />,
    );
    expect(screen.getByText("No headings")).toBeInTheDocument();
  });

  it("highlights the active heading", () => {
    render(
      <TocPanel headings={headings} activeHeadingId="setup" onHeadingClick={() => {}} />,
    );
    const activeButton = screen.getByText("Setup").closest("button")!;
    expect(activeButton.className).toContain("bg-gh-bg-active");
    expect(activeButton.className).toContain("font-semibold");

    const inactiveButton = screen.getByText("Introduction").closest("button")!;
    expect(inactiveButton.className).toContain("bg-transparent");
  });

  it("calls onHeadingClick with the heading id", async () => {
    const user = userEvent.setup();
    const onHeadingClick = vi.fn();
    render(
      <TocPanel headings={headings} activeHeadingId={null} onHeadingClick={onHeadingClick} />,
    );

    await user.click(screen.getByText("Configuration"));
    expect(onHeadingClick).toHaveBeenCalledWith("config");
  });

  it("applies different indentation per heading level", () => {
    render(
      <TocPanel headings={headings} activeHeadingId={null} onHeadingClick={() => {}} />,
    );
    const h1Button = screen.getByText("Introduction").closest("button")!;
    const h2Button = screen.getByText("Setup").closest("button")!;
    const h3Button = screen.getByText("Configuration").closest("button")!;

    expect(h1Button.className).toContain("pl-3");
    expect(h2Button.className).toContain("pl-6");
    expect(h3Button.className).toContain("pl-9");
  });

  it("shows heading text as title attribute", () => {
    render(
      <TocPanel headings={headings} activeHeadingId={null} onHeadingClick={() => {}} />,
    );
    expect(screen.getByTitle("Introduction")).toBeInTheDocument();
    expect(screen.getByTitle("Setup")).toBeInTheDocument();
  });
});
