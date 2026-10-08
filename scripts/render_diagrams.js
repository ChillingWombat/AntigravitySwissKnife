const { app, BrowserWindow } = require('electron');
const fs = require('fs');
const path = require('path');

const targets = {
  architecture: {
    svg: path.resolve('assets/architecture.svg'),
    png: path.resolve('assets/architecture.png'),
    w: 1200,
    h: 680
  },
  modules_overview: {
    svg: path.resolve('assets/modules_overview.svg'),
    png: path.resolve('assets/modules_overview.png'),
    w: 1200,
    h: 580
  },
  lifecycle_flow: {
    svg: path.resolve('assets/lifecycle_flow.svg'),
    png: path.resolve('assets/lifecycle_flow.png'),
    w: 1200,
    h: 480
  }
};

const key = process.env.RENDER_TARGET || 'architecture';
const item = targets[key];

if (!item) {
  console.error('Unknown target:', key);
  process.exit(1);
}

app.whenReady().then(async () => {
  const win = new BrowserWindow({
    width: item.w,
    height: item.h,
    show: false,
    backgroundColor: '#090d16',
    webPreferences: { offscreen: true }
  });

  const svgContent = fs.readFileSync(item.svg, 'utf8');
  await win.loadURL('about:blank');
  await win.webContents.executeJavaScript(`
    document.body.style.margin = '0';
    document.body.style.padding = '0';
    document.body.style.background = '#090d16';
    document.body.style.overflow = 'hidden';
    document.body.innerHTML = ${JSON.stringify(svgContent)};
  `);
  await new Promise(r => setTimeout(r, 600));
  const image = await win.webContents.capturePage();
  fs.writeFileSync(item.png, image.toPNG());
  console.log('Saved PNG for', key, '->', item.png, 'Bytes:', fs.statSync(item.png).size);
  app.quit();
});
