import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { FontSizeToggle } from "./FontSizeToggle";

describe("FontSizeToggle", () => {
  it("shows current size in title", () => {
    render(<FontSizeToggle fontSize="medium" onChange={() => {}} />);
    expect(screen.getByTitle("Text size: Medium text")).toBeInTheDocument();
  });

  it("cycles from small to medium", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<FontSizeToggle fontSize="small" onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Text size" }));
    expect(onChange).toHaveBeenCalledWith("medium");
  });

  it("cycles from medium to large", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<FontSizeToggle fontSize="medium" onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Text size" }));
    expect(onChange).toHaveBeenCalledWith("large");
  });

  it("cycles from large to xlarge", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<FontSizeToggle fontSize="large" onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Text size" }));
    expect(onChange).toHaveBeenCalledWith("xlarge");
  });

  it("cycles from xlarge to small", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<FontSizeToggle fontSize="xlarge" onChange={onChange} />);

    await user.click(screen.getByRole("button", { name: "Text size" }));
    expect(onChange).toHaveBeenCalledWith("small");
  });
});
