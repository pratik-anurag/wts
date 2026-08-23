import "@testing-library/jest-dom/vitest";
import { afterEach, vi } from "vitest";
import { cleanup } from "@testing-library/react";

afterEach(() => {
  cleanup();
});

// Node 25 exposes an incomplete global Web Storage object unless a backing
// file is configured. Keep tests deterministic with an in-memory Storage
// implementation instead of inheriting that process-level object.
const createTestStorage = (): Storage => {
  const entries = new Map<string, string>();
  return {
    get length() {
      return entries.size;
    },
    clear: () => entries.clear(),
    getItem: (key) => entries.get(String(key)) ?? null,
    key: (index) => [...entries.keys()][index] ?? null,
    removeItem: (key) => entries.delete(String(key)),
    setItem: (key, value) => entries.set(String(key), String(value)),
  };
};

Object.defineProperty(globalThis, "localStorage", {
  configurable: true,
  value: createTestStorage(),
});
Object.defineProperty(globalThis, "sessionStorage", {
  configurable: true,
  value: createTestStorage(),
});

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

Object.defineProperty(Element.prototype, "scrollIntoView", {
  writable: true,
  value: vi.fn(),
});

Object.defineProperty(Element.prototype, "scrollTo", {
  configurable: true,
  writable: true,
  value: vi.fn(),
});

// jsdom exposes constructable style sheets without the browser replacement API.
Object.defineProperty(CSSStyleSheet.prototype, "replaceSync", {
  configurable: true,
  value: vi.fn(),
});

Object.defineProperty(HTMLElement.prototype, "hasPointerCapture", {
  writable: true,
  value: vi.fn(() => false),
});

Object.defineProperty(HTMLElement.prototype, "setPointerCapture", {
  writable: true,
  value: vi.fn(),
});

Object.defineProperty(HTMLElement.prototype, "releasePointerCapture", {
  writable: true,
  value: vi.fn(),
});

class TestResizeObserver {
  observe() {}
  unobserve() {}
  disconnect() {}
}

class TestIntersectionObserver {
  root = null;
  rootMargin = "";
  thresholds = [];
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

Object.defineProperty(globalThis, "ResizeObserver", {
  writable: true,
  value: TestResizeObserver,
});

Object.defineProperty(globalThis, "IntersectionObserver", {
  writable: true,
  value: TestIntersectionObserver,
});
