import test from "node:test";
import assert from "node:assert/strict";
import { existsSync, readFileSync, readdirSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const runtimeRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const repoRoot = dirname(runtimeRoot);

test("migrated onboarding sources use runtime scheduler and timer owners", () => {
  const onboarding = readFileSync(join(runtimeRoot, "src/modules/onboarding.ts"), "utf8");
  const styleDir = join(runtimeRoot, "src/styles");
  const styleSources = readdirSync(styleDir)
    .filter((name) => name.endsWith(".ts"))
    .map((name) => readFileSync(join(styleDir, name), "utf8"))
    .join("\n");
  const sources = `${onboarding}\n${styleSources}`;

  assert.doesNotMatch(sources, /\bsetInterval\s*\(/);
  assert.doesNotMatch(sources, /\bnew\s+MutationObserver\b/);
  assert.match(onboarding, /context\.scheduler\.on\(/);
  assert.match(onboarding, /context\.timers\.every\(/);
  assert.equal(existsSync(join(repoRoot, "pkg/plugins/onboarding_accept.go")), false);
});

test("broker source does not read or embed renderer credentials", () => {
  const broker = readFileSync(join(runtimeRoot, "src/core/broker.ts"), "utf8");
  assert.doesNotMatch(broker, /\bBearer\b/i);
  assert.doesNotMatch(broker, /\bAuthorization\b/i);
  assert.doesNotMatch(broker, /\bsessionStorage\b/);
  assert.doesNotMatch(broker, /\blocalStorage\b/);
});
