import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import LinedTextarea from "./LinedTextarea";

describe("LinedTextarea", () => {
  it("renders a gutter line number for each line of text", () => {
    const { container } = render(
      <LinedTextarea value={'{\n  "hello": "world"\n}'} aria-label="payload" onChange={() => undefined} />,
    );

    const gutter = container.querySelector(".lined-textarea__gutter");
    expect(gutter).toBeTruthy();
    expect(gutter?.textContent).toBe("123");
    expect(screen.getByLabelText("payload")).toHaveValue('{\n  "hello": "world"\n}');
  });

  it("renders at least one line number for empty values", () => {
    const { container } = render(
      <LinedTextarea value="" aria-label="payload" onChange={() => undefined} />,
    );

    const lines = container.querySelectorAll(".lined-textarea__line");
    expect(lines).toHaveLength(1);
    expect(lines[0]).toHaveTextContent("1");
  });

  it("highlights the error line in the gutter", () => {
    const { container } = render(
      <LinedTextarea
        value={'{\n  "a": 1\n}x'}
        errorLine={3}
        aria-label="payload"
        aria-invalid
        className="input-invalid"
        onChange={() => undefined}
      />,
    );

    const errorLine = container.querySelector(".lined-textarea__line--error");
    expect(errorLine).toHaveTextContent("3");
    expect(container.querySelector(".lined-textarea--invalid")).toBeTruthy();
    expect(container.querySelector(".lined-textarea.input-invalid")).toBeTruthy();
    expect(screen.getByLabelText("payload").className).not.toMatch(/input-invalid/);
  });
});
