const { app, BrowserWindow } = require('electron');
const fs = require('fs');
const path = require('path');

app.on('window-all-closed', (e) => {
  e.preventDefault();
});

const targets = [
  {
    name: 'architecture',
    svg: path.resolve('assets/architecture.svg'),
    png: path.resolve('assets/architecture.png'),
    w: 1200,
    h: 680,
    bg: '#f8fafc'
  },
  {
    name: 'key_features',
    svg: path.resolve('assets/key_features.svg'),
    png: path.resolve('assets/key_features.png'),
    w: 1200,
    h: 560,
    bg: '#f8fafc'
  },
  {
    name: 'modules_overview',
    svg: path.resolve('assets/modules_overview.svg'),
    png: path.resolve('assets/modules_overview.png'),
    w: 1200,
    h: 580,
    bg: '#f8fafc'
  },
  {
    name: 'lifecycle_flow',
    svg: path.resolve('assets/lifecycle_flow.svg'),
    png: path.resolve('assets/lifecycle_flow.png'),
    w: 1200,
    h: 480,
    bg: '#f8fafc'
  }
];

app.whenReady().then(async () => {
  for (const item of targets) {
    console.log(`Rendering ${item.name}...`);
    const win = new BrowserWindow({
      width: item.w,
      height: item.h,
      show: false,
      backgroundColor: item.bg,
      webPreferences: { offscreen: true }
    });

    const svgContent = fs.readFileSync(item.svg, 'utf8');
    const html = `<!DOCTYPE html><html><body style="margin:0;padding:0;background:${item.bg};overflow:hidden;">${svgContent}</body></html>`;
    await win.loadURL(`data:text/html;charset=utf-8,${encodeURIComponent(html)}`);
    await new Promise(r => setTimeout(r, 600));

    const image = await win.webContents.capturePage();
    fs.writeFileSync(item.png, image.toPNG());
    console.log(`Saved PNG for ${item.name} -> ${item.png} (${fs.statSync(item.png).size} bytes)`);
    win.destroy();
  }

  app.exit(0);
});
