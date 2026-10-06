import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi, afterEach } from "vitest";
import ModelSelector from "./ModelSelector";

afterEach(() => vi.unstubAllGlobals());

describe("ModelSelector", () => {
  it("lists models and fires onChange", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({
        ok: true,
        json: async () => [
          { id: "a", provider: "x", external: false },
          { id: "c", provider: "x", external: true },
        ],
      }),
    );
    const onChange = vi.fn();
    render(<ModelSelector value="a" onChange={onChange} />);
    await screen.findByRole("option", { name: "c" });
    fireEvent.change(screen.getByRole("combobox"), { target: { value: "c" } });
    expect(onChange).toHaveBeenCalledWith("c");
  });

  it("disables select and shows error on failure", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue({ ok: false, status: 500 }),
    );
    render(<ModelSelector value="" onChange={vi.fn()} />);
    await screen.findByRole("alert");
    expect((screen.getByRole("combobox") as HTMLSelectElement).disabled).toBe(
      true,
    );
  });
});
