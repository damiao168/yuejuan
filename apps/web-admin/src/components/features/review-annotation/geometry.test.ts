import { describe, expect, it } from "vitest";
import { canonicalPoint, canonicalRectangle, clientPointToCanonical, geometryStyle } from "./geometry";

describe("canonical annotation geometry", () => {
  it("is invariant when the same image point is displayed at another scale", () => {
    const small = clientPointToCanonical({ x: 70, y: 45 }, { left: 20, top: 20, width: 200, height: 100 });
    const large = clientPointToCanonical({ x: 120, y: 70 }, { left: 20, top: 20, width: 400, height: 200 });
    expect(small).toEqual(large);
    expect(small).toEqual({ x: 0.25, y: 0.25 });
  });

  it("round-trips a reverse drag into normalized CSS geometry", () => {
    const geometry = canonicalRectangle({ x: 0.75, y: 0.8 }, { x: 0.25, y: 0.3 });
    expect(geometry).toEqual({
      coordinate_space: "canonical_image_normalized",
      x: 0.25,
      y: 0.3,
      width: 0.5,
      height: 0.5
    });
    expect(geometryStyle(geometry)).toEqual({ left: "25%", top: "30%", width: "50%", height: "50%" });
  });

  it("stores point annotations without display-pixel dimensions", () => {
    expect(canonicalPoint({ x: 1.3, y: -0.2 })).toEqual({
      coordinate_space: "canonical_image_normalized",
      x: 1,
      y: 0,
      width: 0,
      height: 0
    });
  });
});
