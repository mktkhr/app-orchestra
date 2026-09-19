/**
 * The parts of `variantSuffix` (flags.ts) that come from the process
 * environment rather than from a `run.ts` flag: the picker, the Jev
 * criteria and gate, the service router, the fill arms, the Anthropic
 * thinking switch, the token budgets, the pick wording and the embedding
 * model. Split out of flags.ts only to keep that file under eslint's
 * 300-line cap; every rule about what carries a suffix and what does not
 * is unchanged, and an unset variable still adds nothing.
 */
export function envSuffix(): string {
  const picker = process.env["ORCHESTRA_PICKER"];
  const jev = picker === "hybrid" ? "-hybrid" : picker === "jev" ? "-jev" : "";
  const criteria = process.env["ORCHESTRA_JEV_CRITERIA"] === "v2" ? "-v2" : "";
  const gate = process.env["ORCHESTRA_GATE"] === "jev" ? "-gate" : "";
  const router = process.env["ORCHESTRA_SERVICE_ROUTER"] === "jev" ? "-router" : "";
  const routerCriteria = process.env["ORCHESTRA_SERVICE_ROUTER_CRITERIA"] === "ops" ? "-ops" : "";
  const fillEnum = process.env["ORCHESTRA_FILL_ENUM"] === "jev" ? "-fillenum" : "";
  const skipEmpty = process.env["ORCHESTRA_FILL_SKIP_EMPTY"] === "1" ? "-skipempty" : "";
  const refusalMode = process.env["ORCHESTRA_FILL_ENUM_REFUSAL"];
  const refusal =
    refusalMode === "separate" ? "-refusalsep" : refusalMode === "1" ? "-refusal" : "";
  const unsetWording =
    process.env["ORCHESTRA_FILL_ENUM_UNSET_WORDING"] === "wide" ? "-unsetwide" : "";
  const model = process.env["ORCHESTRA_LLM_MODEL"];
  const modelSuffix =
    model === undefined || model === "qwen3.5-9b-q8"
      ? ""
      : `-model-${model.toLowerCase().replaceAll(/[^a-z0-9.-]/gu, "-")}`;
  const anthropicThinking = process.env["ORCHESTRA_ANTHROPIC_THINKING"] === "on" ? "-think" : "";
  const maxTokensValue = process.env["ORCHESTRA_PLANNER_MAX_TOKENS"];
  const maxTokens = maxTokensValue === undefined ? "" : `-mt${maxTokensValue}`;
  const pickWordingName = process.env["ORCHESTRA_PICK_WORDING"];
  const pickWording =
    pickWordingName === undefined || pickWordingName === "v1" ? "" : `-pick${pickWordingName}`;
  const embedModel = process.env["ORCHESTRA_NARROWING_EMBED_MODEL"];
  const embed =
    embedModel === undefined || embedModel === "e5-large-q8"
      ? ""
      : `-embed${embedModel.toLowerCase().replaceAll(/[^a-z0-9.-]/gu, "-")}`;

  return `${jev}${criteria}${gate}${router}${routerCriteria}${fillEnum}${skipEmpty}${refusal}${unsetWording}${modelSuffix}${anthropicThinking}${maxTokens}${pickWording}${embed}`;
}
