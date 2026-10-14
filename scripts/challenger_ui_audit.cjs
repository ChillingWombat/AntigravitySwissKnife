const fs = require('fs');
const path = require('path');

function walk(dir) {
  let results = [];
  const list = fs.readdirSync(dir);
  for (const file of list) {
    const full = path.join(dir, file);
    const stat = fs.statSync(full);
    if (stat.isDirectory()) {
      if (file !== 'node_modules' && file !== 'dist') {
        results = results.concat(walk(full));
      }
    } else if (file.endsWith('.tsx') || file.endsWith('.ts') || file.endsWith('.css')) {
      results.push(full);
    }
  }
  return results;
}

const files = walk(path.join(__dirname, '..', 'frontend', 'src'));

console.log('=== ADVERSARIAL SCAN STARTING: Total files found:', files.length);

const prodFiles = files.filter(f => !f.includes('.test.') && !f.includes('tests'));
console.log('Production files to scan:', prodFiles.length);

let findings = [];

// 1. Any hardcoded hex in borders (including multi-line and Tailwind classes)
for (const f of prodFiles) {
  const content = fs.readFileSync(f, 'utf-8');

  // Match Tailwind arbitrary border colors: border-[#...]
  const twHexMatches = content.matchAll(/\bborder-\[#([0-9a-fA-F]{3,8})\]/g);
  for (const m of twHexMatches) {
    findings.push({
      type: 'TAILWIND_HEX_BORDER',
      file: f,
      line: content.substring(0, m.index).split('\n').length,
      snippet: m[0]
    });
  }

  // Match any border property with hex color (quoted in JSX or unquoted in CSS)
  const borderHexMatches = content.matchAll(/(?<![\w-])(?:border|borderColor|borderTop|borderBottom|borderLeft|borderRight)\s*:\s*(?:[`'"]([^`'"]*#[0-9a-fA-F]{3,8}[^`'"]*)[`'"]|([^;\n}]*#[0-9a-fA-F]{3,8}[^;\n}]*))/gi);
  for (const m of borderHexMatches) {
    const val = (m[1] || m[2] || '').trim();
    // Exclude CSS custom properties definitions e.g. --border-danger: #fad2cf;
    if (!m[0].startsWith('--')) {
      findings.push({
        type: 'HARDCODED_HEX_BORDER',
        file: f,
        line: content.substring(0, m.index).split('\n').length,
        snippet: m[0].trim()
      });
    }
  }

  // Match 1.5px borders in any form
  const stroke15Matches = content.matchAll(/(?:border|borderWidth|borderTop|borderBottom|borderLeft|borderRight)\s*:\s*[`'"]?1\.5px[`'"]?/gi);
  for (const m of stroke15Matches) {
    findings.push({
      type: '1.5PX_BORDER_STROKE',
      file: f,
      line: content.substring(0, m.index).split('\n').length,
      snippet: m[0]
    });
  }

  // Check 1.5px in CSS files
  if (f.endsWith('.css')) {
    const css15 = content.matchAll(/1\.5px/g);
    for (const m of css15) {
      findings.push({
        type: '1.5PX_IN_CSS',
        file: f,
        line: content.substring(0, m.index).split('\n').length,
        snippet: m[0]
      });
    }
  }

  // Check for decorative emojis in JSX/TSX
  if (f.endsWith('.tsx')) {
    const lines = content.split('\n');
    lines.forEach((line, idx) => {
      // exclude comments
      const trimmed = line.trim();
      if (trimmed.startsWith('//') || trimmed.startsWith('/*') || trimmed.startsWith('*')) return;
      const emojiMatch = line.match(/[\u{1F300}-\u{1F9FF}\u{2600}-\u{26FF}\u{2700}-\u{27BF}]/u);
      if (emojiMatch) {
        findings.push({
          type: 'DECORATIVE_EMOJI',
          file: f,
          line: idx + 1,
          snippet: trimmed
        });
      }
    });
  }
}

// Check collection containers for missing flexWrap: 'wrap'
console.log('\n=== CHECKING COLLECTION CONTAINERS FOR FLEX-WRAP ===');
for (const f of prodFiles) {
  if (!f.endsWith('.tsx')) continue;
  const content = fs.readFileSync(f, 'utf-8');

  // Look for .map( rendering badges/chips/tags inside a div/container
  // Check if the parent container has display: 'flex' (or inline-flex or className="...flex...") without flexWrap: 'wrap'
  // Let's search for chip/badge/tag collection containers
  const lines = content.split('\n');
  lines.forEach((line, idx) => {
    // If line has display: 'flex' or 'inline-flex', and has gap, but holds multiple chips/tags
    if ((line.includes("display: 'flex'") || line.includes('display: "flex"') || line.includes("className=\"flex")) &&
        (line.toLowerCase().includes('badge') || line.toLowerCase().includes('chip') || line.toLowerCase().includes('tag') || line.toLowerCase().includes('swatch') || line.toLowerCase().includes('palette'))) {
      if (!line.includes('flexWrap') && !line.includes('flex-wrap')) {
        // Look within next 3 lines for flexWrap
        const surrounding = lines.slice(Math.max(0, idx - 2), Math.min(lines.length, idx + 4)).join(' ');
        if (!surrounding.includes('flexWrap') && !surrounding.includes('flex-wrap')) {
          findings.push({
            type: 'POSSIBLE_MISSING_FLEX_WRAP',
            file: f,
            line: idx + 1,
            snippet: line.trim()
          });
        }
      }
    }
  });
}

// Check tab and action buttons for whiteSpace: 'nowrap'
console.log('\n=== CHECKING BUTTONS AND TABS FOR WHITE-SPACE: NOWRAP ===');
for (const f of prodFiles) {
  if (!f.endsWith('.tsx')) continue;
  const content = fs.readFileSync(f, 'utf-8');
  const lines = content.split('\n');

  lines.forEach((line, idx) => {
    // Look for button elements or tab buttons
    if (line.includes('<button') || line.includes('role="tab"') || line.includes("role: 'tab'")) {
      const surrounding = lines.slice(idx, Math.min(lines.length, idx + 15)).join('\n');
      // If it's a styled button without className containing btn or without whiteSpace nowrap
      if (!surrounding.includes('btn-') && !surrounding.includes('btn') && surrounding.includes('style={{')) {
        if (!surrounding.includes('whiteSpace') && !surrounding.includes('whitespace-nowrap')) {
          // Check if this button has text label
          const btnText = surrounding.match(/>([^<>{}\n]+)<\/(?:button|span)>/);
          if (btnText && btnText[1].trim().length > 3) {
            findings.push({
              type: 'BUTTON_MISSING_NOWRAP',
              file: f,
              line: idx + 1,
              snippet: btnText[1].trim()
            });
          }
        }
      }
    }
  });
}

console.log('=== FINDINGS SUMMARY ===');
console.log('Total issues found:', findings.length);
findings.forEach(issue => {
  console.log(`[${issue.type}] ${path.relative(path.join(__dirname, '..'), issue.file)}:${issue.line}`);
  console.log(`   ${issue.snippet}`);
});

