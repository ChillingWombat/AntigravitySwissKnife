const { app, BrowserWindow } = require('electron');
const fs = require('fs');
const path = require('path');
const React = require(path.resolve(__dirname, '../frontend/node_modules/react'));
const ReactDOMServer = require(path.resolve(__dirname, '../frontend/node_modules/react-dom/server'));
const {
  Users,
  Cpu,
  Folder,
  SlidersHorizontal,
  ShieldCheck,
  Gauge,
  AppWindow
} = require(path.resolve(__dirname, '../frontend/node_modules/lucide-react'));

const Github = (props) => React.createElement('svg', {
  width: props.size || 24,
  height: props.size || 24,
  viewBox: '0 0 24 24',
  fill: 'none',
  stroke: props.color || 'currentColor',
  strokeWidth: props.strokeWidth || 2,
  strokeLinecap: 'round',
  strokeLinejoin: 'round'
}, [
  React.createElement('path', {
    key: 'p1',
    d: 'M9 19c-5 1.5-5-2.5-7-3m14 6v-3.87a3.37 3.37 0 0 0-.94-2.61c3.14-.35 6.44-1.54 6.44-7A5.44 5.44 0 0 0 20 4.77 5.07 5.07 0 0 0 19.91 1S18.73.65 16 2.48a13.38 13.38 0 0 0-7 0C6.27.65 5.09 1 5.09 1A5.07 5.07 0 0 0 5 4.77a5.44 5.44 0 0 0-1.5 3.78c0 5.42 3.3 6.61 6.44 7A3.37 3.37 0 0 0 9 18.13V22'
  })
]);

app.on('window-all-closed', (e) => e.preventDefault());

const cards = [
  {
    name: 'Account Switcher',
    icon: Users,
    iconColor: '#0284c7',
    bgColor: '#e0f2fe'
  },
  {
    name: 'Custom Provider',
    icon: Cpu,
    iconColor: '#16a34a',
    bgColor: '#dcfce7'
  },
  {
    name: 'GitHub Workspace',
    icon: Github,
    iconColor: '#0f172a',
    bgColor: '#f1f5f9'
  },
  {
    name: 'Preview Browser',
    icon: AppWindow,
    iconColor: '#dc2626',
    bgColor: '#fee2e2'
  },
  {
    name: 'File Explorer',
    icon: Folder,
    iconColor: '#2563eb',
    bgColor: '#eff6ff'
  },
  {
    name: 'UI Enhancement',
    icon: SlidersHorizontal,
    iconColor: '#059669',
    bgColor: '#d1fae5'
  },
  {
    name: 'Conversation Vault',
    icon: ShieldCheck,
    iconColor: '#d97706',
    bgColor: '#fef3c7'
  },
  {
    name: 'Token Monitor',
    icon: Gauge,
    iconColor: '#e11d48',
    bgColor: '#ffe4e6'
  }
];

function generateHtml() {
  const cardsHtml = cards.map(c => {
    const iconSvg = ReactDOMServer.renderToStaticMarkup(
      React.createElement(c.icon, { size: 28, strokeWidth: 2, color: c.iconColor })
    );
    return `
      <div class="feature-card">
        <div class="icon-circle" style="background-color: ${c.bgColor};">
          ${iconSvg}
        </div>
        <div class="card-title">${c.name}</div>
      </div>
    `;
  }).join('');

  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    * {
      box-sizing: border-box;
      margin: 0;
      padding: 0;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif;
      -webkit-font-smoothing: antialiased;
    }
    body {
      background-color: #f8fafc;
      width: 920px;
      height: 382px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justifyContent: center;
      padding: 22px 20px;
      overflow: hidden;
    }
    .grid {
      display: grid;
      grid-template-columns: repeat(4, 206px);
      grid-template-rows: repeat(2, 140px);
      gap: 16px;
      justify-content: center;
    }
    .feature-card {
      background: #ffffff;
      border: 1px solid #e2e8f0;
      border-radius: 16px;
      display: flex;
      flex-direction: column;
      align-items: center;
      justifyContent: center;
      box-shadow: 0 4px 16px -2px rgba(15, 23, 42, 0.05), 0 2px 4px -1px rgba(15, 23, 42, 0.02);
      transition: all 0.2s ease;
      padding: 16px 12px;
      width: 206px;
      height: 140px;
    }
    .icon-circle {
      width: 58px;
      height: 58px;
      border-radius: 50%;
      display: flex;
      align-items: center;
      justify-content: center;
      margin-bottom: 14px;
    }
    .card-title {
      font-size: 16px;
      font-weight: 600;
      color: #0f172a;
      white-space: nowrap;
      text-align: center;
      line-height: 1.2;
      letter-spacing: -0.1px;
    }
    .footer-pill {
      margin-top: 16px;
      display: inline-flex;
      align-items: center;
      gap: 5px;
      padding: 5px 14px;
      border-radius: 9999px;
      background: #ffffff;
      border: 1px solid #cbd5e1;
      font-size: 12px;
      font-weight: 500;
      color: #475569;
      box-shadow: 0 1px 3px rgba(0, 0, 0, 0.03);
    }
    .plus {
      font-weight: 700;
      color: #0284c7;
    }
  </style>
</head>
<body>
  <div class="grid">
    ${cardsHtml}
  </div>
  <div class="footer-pill">
    <span class="plus">+</span>
    <span>and many more!</span>
  </div>
</body>
</html>`;
}

app.whenReady().then(async () => {
  const win = new BrowserWindow({
    width: 920,
    height: 382,
    show: false,
    frame: false,
    backgroundColor: '#f8fafc',
    webPreferences: {
      offscreen: true,
      zoomFactor: 1.0
    }
  });

  const html = generateHtml();
  await win.loadURL(`data:text/html;charset=utf-8,${encodeURIComponent(html)}`);
  await new Promise(r => setTimeout(r, 600));

  const image = await win.webContents.capturePage();
  const outPath = path.resolve('images/key_features_preview.png');
  fs.writeFileSync(outPath, image.toPNG());
  console.log(`Rendered preview to ${outPath} (${fs.statSync(outPath).size} bytes)`);
  win.destroy();
  app.exit(0);
});
