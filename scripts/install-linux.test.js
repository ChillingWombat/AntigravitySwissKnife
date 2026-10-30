const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('fs');
const os = require('os');
const path = require('path');

const { isMainAppCommand, parseCliArgs, cleanShellRCHooks } = require('./install-linux');

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
