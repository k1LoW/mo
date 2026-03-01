import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TocToggle } from "./TocToggle";

describe("TocToggle", () => {
  it("shows 'Show table of contents' title when closed", () => {
    render(<TocToggle isTocOpen={false} onToggle={() => {}} />);
    expect(screen.getByTitle("Show table of contents")).toBeInTheDocument();
  });

  it("shows 'Hide table of contents' title when open", () => {
    render(<TocToggle isTocOpen={true} onToggle={() => {}} />);
    expect(screen.getByTitle("Hide table of contents")).toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<TocToggle isTocOpen={false} onToggle={onToggle} />);

    await user.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledOnce();
  });
});
