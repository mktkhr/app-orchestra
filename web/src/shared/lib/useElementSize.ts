import { useCallback, useEffect, useState, type RefCallback } from "react";

interface ElementSize {
  readonly width: number;
  readonly height: number;
}

const ZERO_SIZE: ElementSize = { width: 0, height: 0 };

/**
 * The rendered pixel size of one element, kept live with `ResizeObserver`.
 *
 * A caller that wants to draw at its actual box - a chart inside a workspace
 * panel, whose box grows when the panel spans more grid columns
 * (`docs/specs/layout.md` section 5, AC-L-106) - reads this instead of
 * guessing from the viewport, which is what a media query can only ever do.
 *
 * The returned ref is a callback, not a plain `useRef`, on purpose: this
 * hook's own first caller (`PanelResult`) does not render the measured
 * element until a result has loaded, so the node an effect with an empty
 * dependency array captured at mount would still be `null` and the
 * `ResizeObserver` would never be created - confirmed by hand, not assumed,
 * against a running platform: two panels of different `width`, both drawing
 * `ResultChart` at its fixed 320×240 default regardless. A callback ref
 * turns "the node arrived" into a state change the effect below can depend
 * on, so it (re)attaches whenever the node this is given actually changes,
 * mount order included.
 *
 * Starts at `{ width: 0, height: 0 }`: nothing has been measured yet, and a
 * test environment with no layout engine (`happy-dom`) never fires a
 * resize, so a caller must treat zero as "not measured" rather than "empty."
 */
export function useElementSize<T extends HTMLElement>(): [RefCallback<T>, ElementSize] {
  const [node, setNode] = useState<T | null>(null);
  const [size, setSize] = useState<ElementSize>(ZERO_SIZE);
  const ref = useCallback((element: T | null) => {
    setNode(element);
  }, []);

  useEffect(() => {
    let observer: ResizeObserver | undefined;

    if (node !== null) {
      observer = new ResizeObserver((entries) => {
        const entry = entries[0];

        if (entry === undefined) {
          return;
        }

        setSize({ width: entry.contentRect.width, height: entry.contentRect.height });
      });
      observer.observe(node);
    }

    return () => {
      observer?.disconnect();
    };
  }, [node]);

  return [ref, size];
}
