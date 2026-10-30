import { existsSync, readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const runtimeRoot = join(repoRoot, "injected-runtime");
const failures = [];

const onboarding = readFileSync(join(runtimeRoot, "src/modules/onboarding.ts"), "utf8");
const stylesDir = join(runtimeRoot, "src/styles");
const styleSources = readdirSync(stylesDir)
  .filter((name) => name.endsWith(".ts"))
  .map((name) => readFileSync(join(stylesDir, name), "utf8"))
  .join("\n");
const migratedSources = `${onboarding}\n${styleSources}`;

if (/\bsetInterval\s*\(/.test(migratedSources)) {
  failures.push("migrated onboarding sources must use TimerRegistry, not setInterval");
}
if (/\bnew\s+MutationObserver\b/.test(migratedSources)) {
  failures.push("migrated onboarding sources must use DomScheduler, not MutationObserver");
}
if (existsSync(join(repoRoot, "pkg/plugins/onboarding_accept.go"))) {
  failures.push("legacy onboarding injection source still exists");
}

const assetDir = join(repoRoot, "pkg/injected/assets");
const assetNames = existsSync(assetDir)
  ? readdirSync(assetDir).filter((name) => !name.startsWith(".")).sort()
  : [];
if (assetNames.length !== 1 || assetNames[0] !== "runtime.js") {
  failures.push(`expected exactly pkg/injected/assets/runtime.js, found ${assetNames.join(", ") || "nothing"}`);
}

const assetPath = join(assetDir, "runtime.js");
if (!existsSync(assetPath) || statSync(assetPath).size < 1000) {
  failures.push("runtime.js is missing or implausibly small");
} else {
  const asset = readFileSync(assetPath, "utf8");
  const count = (marker) => asset.split(marker).length - 1;
  if (count("__SWISS_RUNTIME__") < 1) failures.push("runtime.js is missing the reinjection handle");
  for (const marker of ["MutationObserver", "setInterval"]) {
    if (count(marker) !== 1) failures.push(`${marker} must occur exactly once in runtime.js, found ${count(marker)}`);
  }
  if (/\bBearer\b|\bAuthorization\b|\bsessionStorage\b|\beval\s*\(|\bnew\s+Function\b/.test(asset)) {
    failures.push("runtime.js contains a forbidden credential or dynamic-code construct");
  }
}

if (failures.length > 0) {
  for (const failure of failures) console.error(`injected-runtime: ${failure}`);
  process.exit(1);
}

console.log(`injected-runtime: verified ${statSync(assetPath).size} byte single-owner bundle`);
