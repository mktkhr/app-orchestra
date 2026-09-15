/**
 * How wide a bubble - a question, or a sentence-shaped answer (C1/C3) - is
 * allowed to get before it wraps. Bounded well short of full width so the
 * alternation (AC-C-101, AC-C-106) reads even when every turn is short; a
 * percentage rather than a fixed pixel count so it still leaves visible
 * margin at 375px (AC-C-106) instead of nearly filling the viewport.
 */
const BUBBLE_MAX_WIDTH = "75%";

/**
 * Shared by every bubble - a question (right), a sentence answer, and the
 * pending spinner (both left, C3/C4) - so the three read as the same shape.
 * `overflowWrap` keeps an unbroken run of characters (a long id, a URL)
 * from stretching the bubble past `BUBBLE_MAX_WIDTH` and pushing the page
 * sideways (AC-C-106); MUI's own word-wrapping handles ordinary text.
 *
 * Split out of `TurnList.tsx` so both it and `AnswerResult.tsx` (the
 * `kind: "none"` and fallback branches, which draw the same bubble shape)
 * can use it without either importing the other just for a style constant.
 */
export const BUBBLE_SX = {
  maxWidth: BUBBLE_MAX_WIDTH,
  overflowWrap: "break-word",
} as const;
