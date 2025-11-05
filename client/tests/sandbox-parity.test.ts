import { describe, expect, it } from "vitest";
import rendererGolden from "@assets/vanilla/testdata/form_output.golden.html?raw";
import { vanillaFormHtml } from "../dev/templates";

describe("sandbox parity", () => {
  it("keeps vanilla sandbox markup aligned with renderer golden snapshot", () => {
    expect(vanillaFormHtml.trim()).toBe(rendererGolden.trim());
  });
});
