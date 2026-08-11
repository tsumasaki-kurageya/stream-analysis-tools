import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { App } from "./App";

describe("App", () => {
  it("identifies the scaffold without exposing unfinished controls", () => {
    render(<App />);

    expect(
      screen.getByRole("heading", { name: "Foundation ready" }),
    ).toBeDefined();
    expect(screen.queryByRole("button")).toBeNull();
  });
});
