#!/usr/bin/env node

/**
 * scripts/verify-docs.js
 * 
 * Verifies documentation integrity across README.md and docs/wiki/:
 * 1. README.md line count (60–150 lines) and size (< 12 KB).
 * 2. README.md contains 0 decorative unicode emojis.
 * 3. README.md contains 0 banned AI buzzwords.
 * 4. All 7 wiki files exist in docs/wiki/ and have substantive content (> 30 lines).
 * 5. All 7 wiki files contain 0 decorative unicode emojis.
 * 6. All markdown and HTML links in README.md and wiki files are well-formed.
 */

const fs = require('fs');
const path = require('path');

const ROOT_DIR = path.resolve(__dirname, '..');
const wikiDir = path.join(ROOT_DIR, 'docs', 'wiki');
const BANNED_BUZZWORDS = ['delve', 'leverage', 'empower', 'supercharge', 'paradigm shift'];
const EMOJI_REGEX = /\p{Extended_Pictographic}/u;
const EMOJI_GLOBAL_REGEX = /\p{Extended_Pictographic}/gu;

const EXPECTED_WIKI_FILES = [
  'Home.md',
  'Architecture-and-Multi-Process-Daemon.md',
  'Multi-App-Fleet-Sync-and-Account-Switching.md',
  'Active-Conversation-Continuation-and-Subagent-Revival.md',
  'ACP-Agent-Mesh-Expansion.md',
  'Packaging,-VM-Validation-and-Installation.md',
  'Security,-Compliance-and-Cache-Management.md'
];

let errors = [];

function recordError(msg) {
  errors.push(msg);
  console.error(`  FAIL: ${msg}`);
}

function recordPass(msg) {
  console.log(`  PASS: ${msg}`);
}

console.log('=== Documentation Verification Suite ===\n');

// 1. Verify README.md existence, line count, and file size
console.log('[1/6] Verifying README.md metrics...');
const readmePath = path.join(ROOT_DIR, 'README.md');
if (!fs.existsSync(readmePath)) {
  recordError('README.md does not exist.');
} else {
  const readmeContent = fs.readFileSync(readmePath, 'utf8');
  const readmeLines = readmeContent.split(/\r?\n/);
  const readmeSize = fs.statSync(readmePath).size;

  if (readmeLines.length < 60 || readmeLines.length > 150) {
    recordError(`README.md line count (${readmeLines.length}) outside required range (60-150 lines).`);
  } else {
    recordPass(`README.md line count is ${readmeLines.length} (within 60-150 range).`);
  }

  const maxBytes = 12 * 1024;
  if (readmeSize >= maxBytes) {
    recordError(`README.md size (${readmeSize} bytes) exceeds 12 KB limit (${maxBytes} bytes).`);
  } else {
    recordPass(`README.md size is ${readmeSize} bytes (< 12 KB).`);
  }
}

// 2. Verify README.md has 0 decorative unicode emojis
console.log('\n[2/6] Verifying README.md zero decorative emoji rule...');
if (fs.existsSync(readmePath)) {
  const readmeContent = fs.readFileSync(readmePath, 'utf8');
  const matches = readmeContent.match(EMOJI_GLOBAL_REGEX) || [];
  if (matches.length > 0) {
    recordError(`README.md contains ${matches.length} unicode emoji(s): ${matches.join(' ')}`);
  } else {
    recordPass('README.md contains 0 decorative unicode emojis.');
  }
}

// 3. Verify README.md contains 0 banned AI buzzwords
console.log('\n[3/6] Verifying README.md banned AI buzzwords...');
if (fs.existsSync(readmePath)) {
  const lowerReadme = fs.readFileSync(readmePath, 'utf8').toLowerCase();
  let foundBuzzwords = [];
  for (const word of BANNED_BUZZWORDS) {
    if (lowerReadme.includes(word)) {
      foundBuzzwords.push(word);
    }
  }
  if (foundBuzzwords.length > 0) {
    recordError(`README.md contains banned buzzword(s): ${foundBuzzwords.join(', ')}`);
  } else {
    recordPass('README.md contains 0 banned AI buzzwords.');
  }
}

const localReadmePath = path.join(ROOT_DIR, 'README.local.md');
if (fs.existsSync(localReadmePath)) {
  const lowerLocal = fs.readFileSync(localReadmePath, 'utf8').toLowerCase();
  let foundLocal = [];
  for (const word of BANNED_BUZZWORDS) {
    if (lowerLocal.includes(word)) {
      foundLocal.push(word);
    }
  }
  if (foundLocal.length > 0) {
    recordError(`README.local.md contains banned buzzword(s): ${foundLocal.join(', ')}`);
  } else {
    recordPass('README.local.md contains 0 banned AI buzzwords.');
  }
}

if (fs.existsSync(wikiDir)) {
  for (const file of EXPECTED_WIKI_FILES) {
    const filePath = path.join(wikiDir, file);
    if (fs.existsSync(filePath)) {
      const lowerWiki = fs.readFileSync(filePath, 'utf8').toLowerCase();
      let foundWiki = [];
      for (const word of BANNED_BUZZWORDS) {
        if (lowerWiki.includes(word)) {
          foundWiki.push(word);
        }
      }
      if (foundWiki.length > 0) {
        recordError(`docs/wiki/${file} contains banned buzzword(s): ${foundWiki.join(', ')}`);
      }
    }
  }
  recordPass('docs/wiki/*.md contain 0 banned AI buzzwords.');
}


// 4. Verify all 7 wiki files exist and have > 30 lines
console.log('\n[4/6] Verifying GitHub Wiki files existence and substantive line count (> 30 lines)...');
if (!fs.existsSync(wikiDir)) {
  recordError(`docs/wiki directory not found at: ${wikiDir}`);
} else {
  for (const file of EXPECTED_WIKI_FILES) {
    const filePath = path.join(wikiDir, file);
    if (!fs.existsSync(filePath)) {
      recordError(`Wiki file missing: docs/wiki/${file}`);
    } else {
      const content = fs.readFileSync(filePath, 'utf8');
      const lines = content.split(/\r?\n/);
      if (lines.length <= 30) {
        recordError(`Wiki file docs/wiki/${file} has only ${lines.length} lines (must be > 30).`);
      } else {
        recordPass(`docs/wiki/${file}: ${lines.length} lines (substantive > 30 lines).`);
      }
    }
  }
}

// 5. Verify all 7 wiki files have 0 decorative unicode emojis
console.log('\n[5/6] Verifying GitHub Wiki files zero decorative emoji rule...');
if (fs.existsSync(wikiDir)) {
  for (const file of EXPECTED_WIKI_FILES) {
    const filePath = path.join(wikiDir, file);
    if (fs.existsSync(filePath)) {
      const content = fs.readFileSync(filePath, 'utf8');
      const matches = content.match(EMOJI_GLOBAL_REGEX) || [];
      if (matches.length > 0) {
        recordError(`docs/wiki/${file} contains ${matches.length} unicode emoji(s): ${matches.join(' ')}`);
      } else {
        recordPass(`docs/wiki/${file}: 0 decorative emojis.`);
      }
    }
  }
}

// 6. Verify all markdown links within README.md and wiki files are well-formed
console.log('\n[6/6] Verifying link integrity in README.md and docs/wiki/*.md...');

function slugifyHeader(headerText) {
  return headerText
    .toLowerCase()
    .trim()
    .replace(/[^\w\s-]/g, '')
    .replace(/\s+/g, '-');
}

function extractHeaders(content) {
  const headers = new Set();
  const headerRegex = /^#{1,6}\s+(.+)$/gm;
  let match;
  while ((match = headerRegex.exec(content)) !== null) {
    headers.add(slugifyHeader(match[1]));
  }
  return headers;
}

function verifyDocumentLinks(filePath) {
  const content = fs.readFileSync(filePath, 'utf8');
  const fileDir = path.dirname(filePath);
  const relDocPath = path.relative(ROOT_DIR, filePath);
  const documentHeaders = extractHeaders(content);

  // Markdown links: [text](target) or ![alt](target)
  const mdLinkRegex = /!?\[([^\]]*)\]\(([^)]+)\)/g;
  let match;
  let docLinkCount = 0;

  while ((match = mdLinkRegex.exec(content)) !== null) {
    const [fullMatch, text, target] = match;
    docLinkCount++;
    const cleanTarget = target.trim();

    if (!cleanTarget) {
      recordError(`Empty link target in ${relDocPath}: "${fullMatch}"`);
      continue;
    }

    if (/\s/.test(cleanTarget)) {
      recordError(`Unencoded whitespace in link target in ${relDocPath}: "${cleanTarget}"`);
      continue;
    }

    if (cleanTarget.startsWith('#')) {
      // Internal anchor
      const anchor = cleanTarget.slice(1);
      if (!documentHeaders.has(anchor)) {
        recordError(`Anchor #${anchor} in ${relDocPath} does not match any header in the document.`);
      }
    } else if (cleanTarget.startsWith('http://') || cleanTarget.startsWith('https://')) {
      try {
        new URL(cleanTarget);
      } catch (e) {
        recordError(`Malformed URL in ${relDocPath}: "${cleanTarget}"`);
      }
    } else {
      // Relative file path
      const [targetFile, targetAnchor] = cleanTarget.split('#');
      const resolvedPath = path.resolve(fileDir, targetFile);

      if (!fs.existsSync(resolvedPath)) {
        recordError(`Broken relative link in ${relDocPath}: "${cleanTarget}" (resolved: ${resolvedPath})`);
      }
    }
  }

  // HTML links: <a href="..."> and <img src="...">
  const htmlLinkRegex = /<(?:a|img)[^>]+(?:href|src)=["']([^"']+)["']/g;
  while ((match = htmlLinkRegex.exec(content)) !== null) {
    const [fullMatch, target] = match;
    docLinkCount++;
    const cleanTarget = target.trim();

    if (!cleanTarget) {
      recordError(`Empty HTML link target in ${relDocPath}: "${fullMatch}"`);
      continue;
    }

    if (cleanTarget.startsWith('#')) {
      const anchor = cleanTarget.slice(1);
      if (!documentHeaders.has(anchor)) {
        recordError(`HTML anchor #${anchor} in ${relDocPath} does not match any header in the document.`);
      }
    } else if (cleanTarget.startsWith('http://') || cleanTarget.startsWith('https://')) {
      try {
        new URL(cleanTarget);
      } catch (e) {
        recordError(`Malformed HTML URL in ${relDocPath}: "${cleanTarget}"`);
      }
    } else {
      const [targetFile] = cleanTarget.split('#');
      const resolvedPath = path.resolve(fileDir, targetFile);
      if (!fs.existsSync(resolvedPath)) {
        recordError(`Broken relative HTML link in ${relDocPath}: "${cleanTarget}" (resolved: ${resolvedPath})`);
      }
    }
  }

  recordPass(`${relDocPath}: Verified ${docLinkCount} link(s) successfully.`);
}

// Verify links in README.md
if (fs.existsSync(readmePath)) {
  verifyDocumentLinks(readmePath);
}

// Verify links in README.local.md
if (fs.existsSync(localReadmePath)) {
  verifyDocumentLinks(localReadmePath);
}

// Verify links in all wiki files
if (fs.existsSync(wikiDir)) {
  for (const file of EXPECTED_WIKI_FILES) {
    const filePath = path.join(wikiDir, file);
    if (fs.existsSync(filePath)) {
      verifyDocumentLinks(filePath);
    }
  }
}

// Final Summary
console.log('\n=== Verification Summary ===');
if (errors.length > 0) {
  console.error(`\nFAILED: Found ${errors.length} issue(s) across documentation.`);
  process.exit(1);
} else {
  console.log('\nAll documentation verification checks passed successfully.');
  process.exit(0);
}
