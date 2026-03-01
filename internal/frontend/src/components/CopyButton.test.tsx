import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CopyButton } from "./CopyButton";

beforeEach(() => {
  // Ensure clipboard API is available in jsdom
  if (!navigator.clipboard) {
    Object.defineProperty(navigator, "clipboard", {
      value: {
        writeText: vi.fn().mockResolvedValue(undefined),
        write: vi.fn().mockResolvedValue(undefined),
      },
      writable: true,
      configurable: true,
    });
  }
});

describe("CopyButton", () => {
  it("renders copy button", () => {
    render(<CopyButton content="# Hello" />);
    expect(screen.getByTitle("Copy content")).toBeInTheDocument();
  });

  it("opens dropdown menu on click", async () => {
    const user = userEvent.setup();
    render(<CopyButton content="# Hello" />);

    await user.click(screen.getByTitle("Copy content"));
    expect(screen.getByText("Copy as Markdown")).toBeInTheDocument();
    expect(screen.getByText("Copy as Text")).toBeInTheDocument();
    expect(screen.getByText("Copy as HTML")).toBeInTheDocument();
  });

  it("closes dropdown on second click", async () => {
    const user = userEvent.setup();
    render(<CopyButton content="# Hello" />);

    await user.click(screen.getByTitle("Copy content"));
    expect(screen.getByText("Copy as Markdown")).toBeInTheDocument();

    await user.click(screen.getByTitle("Copy content"));
    expect(screen.queryByText("Copy as Markdown")).not.toBeInTheDocument();
  });

  it("shows checkmark after copying", async () => {
    const user = userEvent.setup();
    render(<CopyButton content="# Hello" />);

    await user.click(screen.getByTitle("Copy content"));
    await user.click(screen.getByText("Copy as Markdown"));

    // After copy, the checkmark SVG is rendered (stroke-width="2" vs "1.5" for the clipboard icon)
    await waitFor(() => {
      const svg = screen.getByTitle("Copy content").querySelector("svg")!;
      expect(svg.getAttribute("stroke-width")).toBe("2");
    });
  });

  it("closes dropdown after copying", async () => {
    const user = userEvent.setup();
    render(<CopyButton content="# Hello" />);

    await user.click(screen.getByTitle("Copy content"));
    await user.click(screen.getByText("Copy as Markdown"));

    expect(screen.queryByText("Copy as Markdown")).not.toBeInTheDocument();
  });
});
