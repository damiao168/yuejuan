import { describe, expect, it } from "vitest";
import type { CapturePage, ImageQualityRun } from "../../api/capture";
import {
  buildQualityDimensionRows,
  filterQualityPages,
  qualityPageCounts,
} from "./imageQualityModel";

const pages = [
  { id: "p1", status: "normalized", page_identity: { quality_status: "passed" } },
  { id: "p2", status: "quality_rejected", page_identity: { quality_status: "review" } },
  { id: "p3", status: "quality_rejected", page_identity: { quality_status: "failed" } },
  { id: "p4", status: "queued", page_identity: { quality_status: "unchecked" } },
] as unknown as CapturePage[];

describe("image quality workspace model", () => {
  it("counts page-level quality states without mixing capture failures", () => {
    expect(qualityPageCounts(pages)).toEqual({ all: 4, attention: 2, passed: 1, pending: 1 });
  });

  it("filters the in-page quality queue", () => {
    expect(filterQualityPages(pages, "attention").map((page) => page.id)).toEqual(["p2", "p3"]);
    expect(filterQualityPages(pages, "passed").map((page) => page.id)).toEqual(["p1"]);
  });

  it("reads all seven explainable dimensions in the product order", () => {
    const run = {
      quality_report: {
        dimensions: {
          completeness: { score: 99.5, status: "passed" },
          effective_resolution: { score: 75, status: "review" },
          sharpness: { score: 92, status: "passed" },
          geometry: { score: 88, status: "passed" },
          illumination_contrast: { score: 80, status: "passed" },
          occlusion_reflection: { score: 97, status: "passed" },
          noise_compression: { score: 90, status: "passed" },
        },
      },
    } as unknown as ImageQualityRun;

    const rows = buildQualityDimensionRows(run);
    expect(rows).toHaveLength(7);
    expect(rows.map((row) => row.key)).toEqual([
      "completeness",
      "effective_resolution",
      "sharpness",
      "geometry",
      "illumination_contrast",
      "occlusion_reflection",
      "noise_compression",
    ]);
    expect(rows[1]).toMatchObject({ label: "有效分辨率", score: 75, status: "review" });
  });
});
