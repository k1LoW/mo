import { describe, it, expect, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ThemeToggle } from "./ThemeToggle";

beforeEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
});

describe("ThemeToggle", () => {
  it("renders toggle button", () => {
    render(<ThemeToggle />);
    expect(screen.getByTitle("Toggle theme")).toBeInTheDocument();
  });

  it("sets data-theme attribute on document", () => {
    localStorage.setItem("mo-theme", "dark");
    render(<ThemeToggle />);
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
  });

  it("toggles from light to dark on click", async () => {
    const user = userEvent.setup();
    localStorage.setItem("mo-theme", "light");
    render(<ThemeToggle />);

    await user.click(screen.getByTitle("Toggle theme"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("dark");
    expect(localStorage.getItem("mo-theme")).toBe("dark");
  });

  it("toggles from dark to light on click", async () => {
    const user = userEvent.setup();
    localStorage.setItem("mo-theme", "dark");
    render(<ThemeToggle />);

    await user.click(screen.getByTitle("Toggle theme"));
    expect(document.documentElement.getAttribute("data-theme")).toBe("light");
    expect(localStorage.getItem("mo-theme")).toBe("light");
  });
});
