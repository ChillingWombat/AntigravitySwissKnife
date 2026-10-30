import { mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { build } from "esbuild";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const outfile = path.resolve(root, "../pkg/injected/assets/runtime.js");

await mkdir(path.dirname(outfile), { recursive: true });
await build({
  absWorkingDir: root,
  entryPoints: ["src/bootstrap.ts"],
  outfile,
  bundle: true,
  minify: true,
  format: "iife",
  platform: "browser",
  target: "es2020",
  charset: "ascii",
  legalComments: "none",
  sourcemap: false,
  treeShaking: true,
});

console.log(`injected runtime built: ${outfile}`);
