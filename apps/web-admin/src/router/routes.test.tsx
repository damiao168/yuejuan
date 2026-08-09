import { describe, expect, it } from "vitest";
import {
  createRouteRegistry,
  notFoundRoute,
  routeFromPath,
  shouldEnableMockRoutes
} from "./routes";

const mockPaths = ["/permissions", "/quality", "/review", "/settings"];

describe("route registration boundary", () => {
  it("registers only production-ready routes in the production registry", () => {
    const registry = createRouteRegistry(false);

    expect(registry.length).toBeGreaterThan(0);
    expect(registry.every((route) => route.productionReady && !route.mock)).toBe(true);
    expect(registry.some((route) => mockPaths.includes(route.path))).toBe(false);
  });

  it("keeps mock routes available only for explicit development and demo registries", () => {
    const registry = createRouteRegistry(true);

    expect(registry.filter((route) => route.mock).map((route) => route.path).sort()).toEqual(mockPaths);
  });

  it("does not resolve a mock deep link against the production registry", () => {
    const registry = createRouteRegistry(false);

    expect(routeFromPath("/review", registry)).toBe(notFoundRoute);
    expect(routeFromPath("/dashboard", registry).path).toBe("/dashboard");
  });

  it("cannot enable mock routes by setting the feature flag in production", () => {
    expect(shouldEnableMockRoutes({
      DEV: false,
      MODE: "production",
      VITE_ENABLE_MOCK_ROUTES: "true"
    })).toBe(false);
    expect(shouldEnableMockRoutes({ DEV: false, MODE: "demo" })).toBe(true);
  });
});
