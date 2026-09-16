/**
 * A failed `POST /api/plan`, carrying the platform's own `ErrorResponse`
 * message so the conversation can show what the server said (a service
 * down, a timeout) instead of only a generic retry line. Since a service's
 * 4xx answer became a `none` result, a failure here is a real platform or
 * service fault, and its message is the one thing worth reading.
 */
export class PlanRequestError extends Error {
  readonly serverMessage: string;

  constructor(serverMessage: string) {
    super(`POST /api/plan failed: ${serverMessage}`);
    this.name = "PlanRequestError";
    this.serverMessage = serverMessage;
  }
}
