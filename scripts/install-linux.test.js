const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const {
  isMainAppCommand,
  parseCliArgs,
  cleanShellRCHooks,
  acquireShellRCLock
} = require('./install-linux');

const appDir = '/home/test/.local/share/antigravity-swiss-knife/app';

test('isMainAppCommand accepts the installed wrapper and binary', () => {
  assert.equal(isMainAppCommand([path.join(appDir, 'antigravity-swiss-knife')], appDir), true);
  assert.equal(isMainAppCommand([path.join(appDir, 'antigravity-swiss-knife.bin')], appDir), true);
});

test('isMainAppCommand rejects helpers and unrelated executables', () => {
  assert.equal(
    isMainAppCommand([path.join(appDir, 'antigravity-swiss-knife.bin'), '--type=renderer'], appDir),
    false
  );
  assert.equal(
    isMainAppCommand([path.join(appDir, 'resources', 'bin', 'swiss'), 'daemon'], appDir),
    false
  );
  assert.equal(isMainAppCommand(['/opt/Antigravity/antigravity'], appDir), false);
});

test('installer defaults to restart and launch outside CI', () => {
  const previousCI = process.env.CI;
  const previousNoLaunch = process.env.ANTIGRAVITY_INSTALL_NO_LAUNCH;
  delete process.env.CI;
  delete process.env.ANTIGRAVITY_INSTALL_NO_LAUNCH;
  const options = parseCliArgs();
  assert.equal(options.restart, true);
  assert.equal(options.launch, true);
  if (previousCI !== undefined) process.env.CI = previousCI;
  if (previousNoLaunch !== undefined) process.env.ANTIGRAVITY_INSTALL_NO_LAUNCH = previousNoLaunch;
});

test('shell RC cleanup atomically removes only managed blocks', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-rc-'));
  try {
    const rc = path.join(home, '.zshrc');
    const original = "alias ls='ls --color=auto'\n# BEGIN Swiss Knife CLI Integration\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n# END Swiss Knife CLI Integration\nexport CLOUD_CODE_URL=\"https://example.test\"\n";
    fs.writeFileSync(rc, original, { mode: 0o600 });

    cleanShellRCHooks(home);

    assert.equal(fs.readFileSync(rc, 'utf8'), "alias ls='ls --color=auto'\nexport CLOUD_CODE_URL=\"https://example.test\"\n");
    assert.deepEqual(fs.readdirSync(home), ['.zshrc']);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('shell RC cleanup rejects a fresh live-owner lock', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-live-'));
  try {
    const rc = path.join(home, '.zshrc');
    const lock = `${rc}.swiss-install.lock`;
    const original = "# BEGIN Swiss Knife CLI Integration\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n# END Swiss Knife CLI Integration\n";
    fs.writeFileSync(rc, original, { mode: 0o600 });
    fs.writeFileSync(lock, JSON.stringify({ pid: process.pid, createdAt: new Date().toISOString() }), { mode: 0o600 });

    assert.throws(
      () => cleanShellRCHooks(home, { lockTimeoutMs: 30 }),
      /shell RC cleanup failed: \.zshrc: timed out waiting/
    );
    assert.equal(fs.readFileSync(rc, 'utf8'), original);
    assert.equal(fs.existsSync(lock), true);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('installer locks contain owner metadata and reclaim dead or aged owners', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-stale-'));
  try {
    const lock = path.join(home, '.zshrc.swiss-install.lock');
    fs.writeFileSync(
      lock,
      JSON.stringify({ pid: 2147483647, createdAt: new Date(0).toISOString() }),
      { mode: 0o600 }
    );

    let fd = acquireShellRCLock(lock, { lockTimeoutMs: 250 });
    let metadata = JSON.parse(fs.readFileSync(lock, 'utf8'));
    assert.equal(metadata.pid, process.pid);
    assert.equal(Number.isNaN(Date.parse(metadata.createdAt)), false);
    fs.closeSync(fd);
    fs.unlinkSync(lock);

    fs.writeFileSync(lock, '{}', { mode: 0o600 });
    const old = new Date(Date.now() - (5 * 60 * 1000) - 1000);
    fs.utimesSync(lock, old, old);

    fd = acquireShellRCLock(lock, { lockTimeoutMs: 250 });
    metadata = JSON.parse(fs.readFileSync(lock, 'utf8'));
    assert.equal(metadata.pid, process.pid);
    fs.closeSync(fd);
    fs.unlinkSync(lock);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});
