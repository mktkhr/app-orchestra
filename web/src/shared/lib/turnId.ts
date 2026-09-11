/**
 * A unique id for one conversation turn.
 *
 * Not `crypto.randomUUID()`, which exists only in a secure context: over
 * plain HTTP on anything but localhost - a machine reached by its LAN or
 * Tailscale address, which is how this is actually looked at - the property
 * is undefined and calling it throws. A turn id is a React key and nothing
 * more; it never leaves the browser, so it does not need to be a UUID, and
 * it must not be the reason the page fails to render.
 *
 * `crypto.randomUUID` is still preferred when it is there, so ids look the
 * same in the contexts that have it.
 */
let counter = 0;

export function nextTurnId(): string {
  const { crypto } = globalThis;

  if (typeof crypto.randomUUID === "function") {
    return crypto.randomUUID();
  }

  counter += 1;

  return `turn-${String(Date.now())}-${String(counter)}`;
}
