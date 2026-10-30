#!/usr/bin/env node
'use strict';

const fs = require('node:fs');
const path = require('node:path');
const { spawnSync } = require('node:child_process');

const DIST_REL = 'pkg/webgui/dist';
const INDEX_REL = `${DIST_REL}/index.html`;
const ATTRIBUTE_RE = /(?:^|\s)(?:src|href)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s"'=<>]+))/gi;
const BASE_URL = new URL('https://webgui.local/index.html');

function git(repoRoot, args) {
  const result = spawnSync('git', ['-C', repoRoot, ...args], { encoding: 'utf8' });
  if (result.error) {
    throw result.error;
  }
  return result;
}

function addError(errors, code, message) {
  errors.push({ code, message });
}

function referencedAssets(html, errors) {
  const assets = new Map();

  for (const match of html.matchAll(ATTRIBUTE_RE)) {
    const raw = (match[1] ?? match[2] ?? match[3] ?? '').replaceAll('&amp;', '&').trim();
    if (!raw || raw.startsWith('#')) {
      continue;
    }

    let parsed;
    try {
      parsed = new URL(raw, BASE_URL);
    } catch {
      addError(errors, 'E_ASSET_REFERENCE', `Invalid asset reference: ${raw}`);
      continue;
    }
    if (parsed.origin !== BASE_URL.origin) {
      continue;
    }

    const absoluteURL = raw.startsWith('//') || /^[a-z][a-z\d+.-]*:/i.test(raw);
    const assetPath = (absoluteURL ? parsed.pathname : raw).split(/[?#]/, 1)[0];
    let decoded;
    try {
      decoded = decodeURIComponent(assetPath);
    } catch {
      addError(errors, 'E_ASSET_REFERENCE', `Invalid URL encoding in asset reference: ${raw}`);
      continue;
    }

    const relative = decoded.replace(/^\/+/, '');
    if (!relative) {
      continue;
    }
    assets.set(relative, raw);
  }

  return assets;
}

function check(repoRoot) {
  const errors = [];
  const distDir = path.join(repoRoot, DIST_REL);
  const indexPath = path.join(repoRoot, INDEX_REL);

  let indexHTML;
  try {
    indexHTML = fs.readFileSync(indexPath, 'utf8');
  } catch {
    addError(errors, 'E_INDEX_MISSING', `Missing ${INDEX_REL}`);
  }

  if (indexHTML !== undefined) {
    for (const [relative, reference] of referencedAssets(indexHTML, errors)) {
      const target = path.resolve(distDir, relative);
      const insideDist = path.relative(distDir, target);
      if (insideDist === '..' || insideDist.startsWith(`..${path.sep}`) || path.isAbsolute(insideDist)) {
        addError(errors, 'E_ASSET_OUTSIDE_DIST', `${reference} resolves outside ${DIST_REL}`);
        continue;
      }

      let isFile = false;
      try {
        isFile = fs.statSync(target).isFile();
      } catch {
        addError(errors, 'E_ASSET_MISSING', `${reference} does not exist at ${DIST_REL}/${relative}`);
      }
      if (!isFile) {
        continue;
      }

      const trackedPath = `${DIST_REL}/${insideDist.split(path.sep).join('/')}`;
      const tracked = git(repoRoot, ['ls-files', '--error-unmatch', '--', trackedPath]);
      if (tracked.status !== 0) {
        addError(errors, 'E_ASSET_UNTRACKED', `${trackedPath} is referenced but not tracked by git`);
      }
    }
  }

  const untracked = git(repoRoot, ['ls-files', '--others', '-z', '--', DIST_REL]);
  if (untracked.status !== 0) {
    addError(errors, 'E_GIT', untracked.stderr.trim() || `Unable to inspect ${DIST_REL}`);
  } else {
    for (const file of untracked.stdout.split('\0').filter(Boolean)) {
      addError(errors, 'E_DIST_UNTRACKED', `${file} is untracked under ${DIST_REL}`);
    }
  }

  if (errors.length > 0) {
    for (const error of errors) {
      console.error(`[${error.code}] ${error.message}`);
    }
    console.error(`Web GUI dist check failed with ${errors.length} error(s).`);
    return 1;
  }

  const tracked = git(repoRoot, ['ls-files', '-z', '--', DIST_REL]);
  const trackedCount = tracked.stdout.split('\0').filter(Boolean).length;
  console.log(`Web GUI dist OK: ${trackedCount} tracked files.`);
  return 0;
}

const repoRoot = path.resolve(process.argv[2] ?? path.join(__dirname, '..'));
try {
  const gitRoot = git(repoRoot, ['rev-parse', '--show-toplevel']);
  if (gitRoot.status !== 0) {
    console.error(`[E_GIT] ${gitRoot.stderr.trim() || `${repoRoot} is not a git worktree`}`);
    process.exitCode = 1;
  } else {
    process.exitCode = check(repoRoot);
  }
} catch (error) {
  console.error(`[E_GIT] ${error.message}`);
  process.exitCode = 1;
}
