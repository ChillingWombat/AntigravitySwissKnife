const test = require('node:test');
const assert = require('node:assert/strict');
const path = require('path');

const { isMainAppCommand, parseCliArgs } = require('./install-linux');

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
