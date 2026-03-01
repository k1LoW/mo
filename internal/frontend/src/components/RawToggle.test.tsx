import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { RawToggle } from "./RawToggle";

describe("RawToggle", () => {
  it("shows 'Show raw' title when isRaw is false", () => {
    render(<RawToggle isRaw={false} onToggle={() => {}} />);
    expect(screen.getByTitle("Show raw")).toBeInTheDocument();
  });

  it("shows 'Show rendered' title when isRaw is true", () => {
    render(<RawToggle isRaw={true} onToggle={() => {}} />);
    expect(screen.getByTitle("Show rendered")).toBeInTheDocument();
  });

  it("calls onToggle when clicked", async () => {
    const user = userEvent.setup();
    const onToggle = vi.fn();
    render(<RawToggle isRaw={false} onToggle={onToggle} />);

    await user.click(screen.getByRole("button"));
    expect(onToggle).toHaveBeenCalledOnce();
  });
});
