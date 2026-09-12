/**
 * Signing in against a running platform binary, for the process-level e2e
 * suites (`e2e/src/*.test.ts`). Every route but `GET /api/health` and
 * `POST /api/session` now answers 401 without a session
 * (docs/specs/auth.md, AC-A-102) - `docs/plans/auth.md` Task 6, Step 1
 * makes each existing suite sign in before it drives anything else, and
 * this is the one implementation every suite in `src/` shares, rather than
 * each carrying its own copy.
 *
 * `e2e/browser/helpers/auth.ts` is the browser-driven equivalent: it drives
 * the sign-in screen through a `Page` instead of a bare `fetch`, so the two
 * do not share code - there is no HTTP request in common to factor out.
 */

/** One person's session against a platform: the cookie every later request needs. */
export interface Session {
  readonly cookie: string;
}

/**
 * Signs in as name/password against the platform at baseUrl and returns
 * the session cookie to carry on every later request (see
 * {@link withSession}).
 *
 * Node's `fetch` (undici) keeps no cookie jar of its own - unlike a
 * browser, or Go's `net/http/cookiejar` used by
 * `services/platform/acceptance`'s own `newCookieJarClient` - so the
 * cookie `Set-Cookie` returns here has to be threaded through by hand.
 */
export async function signIn(baseUrl: string, name: string, password: string): Promise<Session> {
  const response = await fetch(`${baseUrl}/api/session`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ name, password }),
  });

  if (!response.ok) {
    throw new Error(`signing in as ${name} against ${baseUrl} failed: ${String(response.status)}`);
  }

  const setCookie = response.headers.get("set-cookie");

  if (setCookie === null) {
    throw new Error(`signing in as ${name} against ${baseUrl} set no session cookie`);
  }

  const [cookie] = setCookie.split(";");

  return { cookie: cookie ?? "" };
}

/**
 * Merges session's cookie into a fetch `RequestInit`, alongside whatever
 * headers init already carries - so a call already setting
 * `content-type: application/json` keeps it.
 *
 * Built through the `Headers` class rather than `{ ...init.headers, ... }`:
 * `RequestInit["headers"]` (`HeadersInit`) can also be an array of
 * `[name, value]` tuples or a `Headers` instance, and spreading either of
 * those onto a plain object does not merge what a caller meant - `Headers`
 * itself already knows how to fold every one of its own accepted shapes
 * together.
 */
export function withSession(session: Session, init: RequestInit = {}): RequestInit {
  const headers = new Headers(init.headers);

  headers.set("cookie", session.cookie);

  return { ...init, headers };
}
