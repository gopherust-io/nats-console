import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import BrandLogo from "./BrandLogo";

describe("BrandLogo", () => {
  it("renders the dark wordmark only", () => {
    const { container } = render(<BrandLogo />);
    const imgs = container.querySelectorAll("img.brand-logo__img");
    expect(imgs).toHaveLength(1);
    expect(imgs[0]?.getAttribute("src")).toContain("brand-logo.png");
    expect(imgs[0]?.getAttribute("src")).not.toContain("brand-logo-light");
    expect(container.querySelector(".brand-logo__img--light")).toBeNull();
  });
});
