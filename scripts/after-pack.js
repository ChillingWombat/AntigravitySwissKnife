const fs = require('fs');
const path = require('path');
const { execFileSync } = require('child_process');
const os = require('os');

const C_WRAPPER_SOURCE = `#define _GNU_SOURCE
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>

int main(int argc, char *argv[]) {
    char exe_path[PATH_MAX];
    if (realpath("/proc/self/exe", exe_path) == NULL) {
        perror("realpath");
        return 1;
    }
    char bin_path[PATH_MAX + 16];
    snprintf(bin_path, sizeof(bin_path), "%s.bin", exe_path);

    int has_no_sandbox = 0;
    for (int i = 1; i < argc; i++) {
        if (strcmp(argv[i], "--no-sandbox") == 0) {
            has_no_sandbox = 1;
            break;
        }
    }

    char **new_argv = calloc((size_t)argc + 4, sizeof(char *));
    if (!new_argv) return 1;

    int idx = 0;
    new_argv[idx++] = bin_path;
    if (!has_no_sandbox) {
        new_argv[idx++] = "--no-sandbox";
        new_argv[idx++] = "--disable-setuid-sandbox";
    }
    for (int i = 1; i < argc; i++) {
        new_argv[idx++] = argv[i];
    }
    new_argv[idx] = NULL;

    execv(bin_path, new_argv);
    perror("execv");
    return 1;
}
`;

const BASH_WRAPPER_SOURCE = `#!/usr/bin/env bash
SOURCE="\${BASH_SOURCE[0]}"
while [ -h "$SOURCE" ]; do
  DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
  SOURCE="$(readlink "$SOURCE")"
  [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
exec "\${DIR}/antigravity-swiss-knife.bin" --no-sandbox --disable-setuid-sandbox "$@"
`;

exports.default = async function afterPack(context) {
  const { appOutDir, electronPlatformName } = context;

  if (electronPlatformName === 'linux') {
    const chromeSandbox = path.join(appOutDir, 'chrome-sandbox');
    if (fs.existsSync(chromeSandbox)) {
      fs.unlinkSync(chromeSandbox);
      console.log('[afterPack] Removed chrome-sandbox:', chromeSandbox);
    }

    const binPath = path.join(appOutDir, 'antigravity-swiss-knife');
    const realBinPath = path.join(appOutDir, 'antigravity-swiss-knife.bin');

    if (fs.existsSync(binPath) && !fs.existsSync(realBinPath)) {
      const stat = fs.statSync(binPath);
      if (stat.size > 1024 * 1024) {
        fs.renameSync(binPath, realBinPath);
        fs.chmodSync(realBinPath, 0o755);

        let compiledNativeWrapper = false;
        try {
          const tmpSrc = path.join(os.tmpdir(), `ask-wrapper-${process.pid}.c`);
          fs.writeFileSync(tmpSrc, C_WRAPPER_SOURCE, 'utf8');
          execFileSync('gcc', ['-O2', '-s', tmpSrc, '-o', binPath], { stdio: 'ignore' });
          try { fs.unlinkSync(tmpSrc); } catch {}
          fs.chmodSync(binPath, 0o755);
          compiledNativeWrapper = true;
          console.log('[afterPack] Installed native ELF --no-sandbox wrapper:', binPath);
        } catch {
          fs.writeFileSync(binPath, BASH_WRAPPER_SOURCE, { mode: 0o755 });
          fs.chmodSync(binPath, 0o755);
          console.log('[afterPack] Installed bash --no-sandbox wrapper:', binPath);
        }
      }
    }

    const swissBin = path.join(appOutDir, 'resources', 'bin', 'swiss');
    if (fs.existsSync(swissBin)) {
      fs.chmodSync(swissBin, 0o755);
    }
  } else if (electronPlatformName === 'darwin') {
    const rootDir = path.resolve(__dirname, '..');
    const isArm64 = context.arch === 3 || String(appOutDir).includes('arm64');
    const archBinName = isArm64 ? 'swiss-darwin-arm64' : 'swiss-darwin-amd64';
    const srcBin = path.join(rootDir, 'release', 'macos', archBinName);
    const appBin = path.join(
      appOutDir,
      'Antigravity Swiss Knife.app',
      'Contents',
      'Resources',
      'bin',
      'swiss'
    );
    if (fs.existsSync(srcBin) && fs.existsSync(path.dirname(appBin))) {
      fs.copyFileSync(srcBin, appBin);
      fs.chmodSync(appBin, 0o755);
      console.log(`[afterPack] Bundled ${archBinName} into ${appBin}`);
    } else if (fs.existsSync(appBin)) {
      fs.chmodSync(appBin, 0o755);
    }
  } else if (electronPlatformName === 'win32') {
    const rootDir = path.resolve(__dirname, '..');
    const srcWinBin = path.join(rootDir, 'bin', 'swiss.exe');
    const destWinBinDir = path.join(appOutDir, 'resources', 'bin');
    const destWinBin = path.join(destWinBinDir, 'swiss.exe');
    if (fs.existsSync(srcWinBin)) {
      if (!fs.existsSync(destWinBinDir)) {
        fs.mkdirSync(destWinBinDir, { recursive: true });
      }
      fs.copyFileSync(srcWinBin, destWinBin);
      console.log(`[afterPack] Verified swiss.exe in ${destWinBin}`);
    }
  }
};
