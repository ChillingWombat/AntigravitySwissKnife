'use strict';

const assert = require('node:assert/strict');
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { execFileSync, spawnSync } = require('node:child_process');
const test = require('node:test');

const SCRIPT = path.join(__dirname, 'check-webgui-dist.js');
const DIST_REL = 'pkg/webgui/dist';

function git(root, ...args) {
  execFileSync('git', ['-C', root, ...args], { stdio: 'pipe' });
}

function createRepo(t) {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), 'webgui-dist-check-'));
  t.after(() => fs.rmSync(root, { recursive: true, force: true }));
  git(root, 'init', '-q');
  git(root, 'config', 'user.email', 'webgui-dist@example.com');
  git(root, 'config', 'user.name', 'Web GUI Dist Test');
  return root;
}

function writeDistFile(root, relative, contents) {
  const target = path.join(root, DIST_REL, relative);
  fs.mkdirSync(path.dirname(target), { recursive: true });
  fs.writeFileSync(target, contents);
}

function runChecker(root) {
  return spawnSync(process.execPath, [SCRIPT, root], { encoding: 'utf8' });
}

test('accepts a clean tracked asset set', (t) => {
  const root = createRepo(t);
  writeDistFile(root, 'index.html', [
    '<!doctype html>',
    '<script type="module" src="./assets/index-test.js"></script>',
    '<link rel="stylesheet" href="./assets/index-test.css">',
    '<link rel="icon" href="./favicon.ico">',
  ].join('\n'));
  writeDistFile(root, 'assets/index-test.js', 'console.log("ok");\n');
  writeDistFile(root, 'assets/index-test.css', 'body { margin: 0; }\n');
  writeDistFile(root, 'favicon.ico', 'icon');
  git(root, 'add', '-A', '--', DIST_REL);

  const result = runChecker(root);

  assert.equal(result.status, 0, result.stderr);
  assert.match(result.stdout, /Web GUI dist OK/);
});

test('reports a missing referenced asset', (t) => {
  const root = createRepo(t);
  writeDistFile(root, 'index.html', '<script src="./assets/missing.js"></script>\n');
  git(root, 'add', '--', `${DIST_REL}/index.html`);

  const result = runChecker(root);

  assert.equal(result.status, 1);
  assert.match(result.stderr, /\[E_ASSET_MISSING\]/);
  assert.match(result.stderr, /assets\/missing\.js/);
});

test('reports referenced and unreferenced untracked assets', (t) => {
  const root = createRepo(t);
  writeDistFile(root, 'index.html', '<script src="./assets/extra.js"></script>\n');
  git(root, 'add', '--', `${DIST_REL}/index.html`);
  writeDistFile(root, 'assets/extra.js', 'console.log("untracked");\n');
  writeDistFile(root, 'assets/orphan.js', 'console.log("orphan");\n');

  const result = runChecker(root);

  assert.equal(result.status, 1);
  assert.match(result.stderr, /\[E_ASSET_UNTRACKED\]/);
  assert.match(result.stderr, /\[E_DIST_UNTRACKED\].*assets\/orphan\.js/);
});
