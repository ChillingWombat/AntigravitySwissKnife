const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const {
  isMainAppCommand,
  parseCliArgs,
  cleanShellRCHooks,
  acquireShellRCLock,
  releaseShellRCLock
} = require('./install-linux');

const appDir = '/home/test/.local/share/antigravity-swiss-knife/app';

function writeLock(lock, owner) {
  fs.mkdirSync(lock, { mode: 0o700 });
  fs.writeFileSync(path.join(lock, 'owner.json'), JSON.stringify(owner), { mode: 0o600 });
}

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

test('shell RC cleanup leaves every RC unchanged when any scan fails', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-plan-fail-'));
  try {
    const zshrc = path.join(home, '.zshrc');
    const bashrc = path.join(home, '.bashrc');
    const zshOriginal = '# BEGIN Swiss Knife CLI Integration\nexport CLOUD_CODE_URL="http://127.0.0.1:8765"\n';
    const bashOriginal = 'alias keep=1\n# BEGIN Swiss Knife CLI Integration\nexport CLOUD_CODE_URL="http://127.0.0.1:8765"\n# END Swiss Knife CLI Integration\n';
    fs.writeFileSync(zshrc, zshOriginal, { mode: 0o600 });
    fs.writeFileSync(bashrc, bashOriginal, { mode: 0o600 });

    assert.throws(
      () => cleanShellRCHooks(home, { lockTimeoutMs: 250 }),
      /shell RC cleanup failed: \.zshrc: managed block is missing/
    );
    assert.equal(fs.readFileSync(zshrc, 'utf8'), zshOriginal);
    assert.equal(fs.readFileSync(bashrc, 'utf8'), bashOriginal);
    assert.deepEqual(fs.readdirSync(home).sort(), ['.bashrc', '.zshrc']);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('installer locks never delete a fresh live contender', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-live-'));
  try {
    const rc = path.join(home, '.zshrc');
    const lock = `${rc}.swiss-install.lock`;
    const original = "# BEGIN Swiss Knife CLI Integration\nexport CLOUD_CODE_URL=\"http://127.0.0.1:8765\"\n# END Swiss Knife CLI Integration\n";
    const owner = JSON.stringify({ pid: process.pid, started_at: new Date().toISOString(), token: 'contender' });
    fs.writeFileSync(rc, original, { mode: 0o600 });
    writeLock(lock, JSON.parse(owner));

    assert.throws(
      () => acquireShellRCLock(lock, { lockTimeoutMs: 30 }),
      /timed out waiting for shell RC lock .*owner pid/
    );
    assert.equal(fs.readFileSync(rc, 'utf8'), original);
    assert.equal(fs.readFileSync(path.join(lock, 'owner.json'), 'utf8'), owner);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('installer reclaims a dead-owner lock directory', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-stale-'));
  try {
    const lock = path.join(home, '.zshrc.swiss-install.lock');
    writeLock(lock, { pid: 2147483647, started_at: new Date(0).toISOString(), token: 'dead-owner' });

    const handle = acquireShellRCLock(lock, { lockTimeoutMs: 250 });
    const metadata = JSON.parse(fs.readFileSync(path.join(lock, 'owner.json'), 'utf8'));
    assert.equal(metadata.pid, process.pid);
    assert.equal(Number.isNaN(Date.parse(metadata.started_at)), false);
    assert.match(metadata.token, /^[a-f0-9]{32}$/);
    releaseShellRCLock(handle);
    assert.equal(fs.existsSync(lock), false);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('installer fails closed for an aged live-owner lock', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-aged-live-'));
  try {
    const lock = path.join(home, '.zshrc.swiss-install.lock');
    const owner = JSON.stringify({ pid: process.pid, started_at: new Date(0).toISOString(), token: 'aged-live' });
    writeLock(lock, JSON.parse(owner));
    const old = new Date(Date.now() - (5 * 60 * 1000) - 1000);
    fs.utimesSync(lock, old, old);

    assert.throws(
      () => acquireShellRCLock(lock, { lockTimeoutMs: 30 }),
      /timed out waiting for shell RC lock .*owner pid/
    );
    assert.equal(fs.readFileSync(path.join(lock, 'owner.json'), 'utf8'), owner);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});

test('installer release removes lock and trash directories after normal completion', () => {
  const home = fs.mkdtempSync(path.join(os.tmpdir(), 'swiss-installer-lock-release-'));
  try {
    const lock = path.join(home, '.zshrc.swiss-install.lock');
    const handle = acquireShellRCLock(lock, { lockTimeoutMs: 250 });
    assert.equal(fs.statSync(lock).isDirectory(), true);

    releaseShellRCLock(handle);

    assert.equal(fs.existsSync(lock), false);
    assert.deepEqual(fs.readdirSync(home), []);
  } finally {
    fs.rmSync(home, { recursive: true, force: true });
  }
});
