/**
 * The admin account's password, seeded by `e2e/playwright.config.ts`'s own
 * `webServer` env (`ORCHESTRA_ADMIN_PASSWORD`) for the platform every spec
 * in this directory drives. Named here, once, so the config and
 * `helpers/auth.ts`'s `signInAsAdmin` cannot drift apart the way two
 * separately-typed literals could.
 */
export const ADMIN_PASSWORD = "e2e-admin-password";
