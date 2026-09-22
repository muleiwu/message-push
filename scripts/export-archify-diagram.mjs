// Headless element screenshot for archify diagrams.
// Opens the delivered HTML in embed+light mode, locates the diagram element,
// and captures it as a PNG at the requested device scale.
//
// Usage:
//   node scripts/export-archify-diagram.mjs <input.html> <output.png> [scale]
// Example:
//   node scripts/export-archify-diagram.mjs docs/archify/en/message-push-system.en.html \
//        docs/archify/en/message-push-system.en.png 4
//
// Requires: Node.js >= 22 (global WebSocket) and Google Chrome (macOS path below).
import { spawn } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import { pathToFileURL } from 'node:url';

const [, , htmlPath, outPath, scaleArg] = process.argv;
if (!htmlPath || !outPath) {
  console.error('Usage: node scripts/export-archify-diagram.mjs <input.html> <output.png> [scale]');
  process.exit(1);
}
const scale = Number(scaleArg || 4);
const CHROME = '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome';
const PORT = 9333;

const chrome = spawn(CHROME, [
  '--headless=new', '--disable-gpu', '--hide-scrollbars',
  `--remote-debugging-port=${PORT}`,
  '--user-data-dir=/tmp/archify-export-profile',
  'about:blank',
], { stdio: ['ignore', 'ignore', 'ignore'] });

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

async function newTarget(url) {
  for (let i = 0; i < 30; i++) {
    try {
      const res = await fetch(`http://127.0.0.1:${PORT}/json/new?${new URLSearchParams({ url })}`, { method: 'PUT' });
      if (res.ok) return await res.json();
    } catch (_) { /* chrome not up yet */ }
    await sleep(200);
  }
  throw new Error('chrome devtools endpoint never came up');
}

const target = await newTarget('about:blank');
const ws = new WebSocket(target.webSocketDebuggerUrl);
let msgId = 0;
const pending = new Map();
const events = [];

ws.onmessage = (ev) => {
  const msg = JSON.parse(ev.data);
  if (msg.id && pending.has(msg.id)) {
    pending.get(msg.id)(msg);
    pending.delete(msg.id);
  } else if (msg.method) {
    events.push(msg);
  }
};
await new Promise((r, j) => { ws.onopen = r; ws.onerror = j; });

function send(method, params = {}) {
  const id = ++msgId;
  ws.send(JSON.stringify({ id, method, params }));
  return new Promise((res) => pending.set(id, res));
}

function waitForLoad() {
  return new Promise(async (resolve) => {
    const check = setInterval(() => {
      if (events.some((e) => e.method === 'Page.loadEventFired')) {
        clearInterval(check);
        resolve();
      }
    }, 100);
    setTimeout(() => { clearInterval(check); resolve(); }, 15000);
  });
}

await send('Page.enable');
await send('Emulation.setDeviceMetricsOverride', { width: 1880, height: 1000, deviceScaleFactor: 1, mobile: false });
const loadP = waitForLoad();
await send('Page.navigate', { url: `${pathToFileURL(htmlPath).href}?embed=1&theme=light` });
await loadP;
await sleep(1500); // let viewer settle (theme, fonts)

const rectRes = await send('Runtime.evaluate', {
  returnByValue: true,
  expression: `(() => {
    const diagram = document.querySelector('.diagram-container');
    const svg = diagram && (diagram.querySelector(':scope > svg') || diagram.querySelector(':scope > .diagram-stage > svg'));
    const stage = diagram && (diagram.querySelector(':scope > .diagram-stage') || svg);
    const el = stage || svg;
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { x: r.x, y: r.y, width: r.width, height: r.height };
  })()`,
});
const rect = rectRes.result?.result?.value;
if (!rect) throw new Error('diagram element not found');
console.log('element rect:', JSON.stringify(rect));

const shot = await send('Page.captureScreenshot', {
  format: 'png',
  clip: { x: rect.x, y: rect.y, width: rect.width, height: rect.height, scale },
  captureBeyondViewport: false,
});
if (shot.error || !shot.result?.data) throw new Error(JSON.stringify(shot.error || shot).slice(0, 200));
writeFileSync(outPath, Buffer.from(shot.result.data, 'base64'));
console.log('wrote', outPath, `at ${Math.round(rect.width * scale)}x${Math.round(rect.height * scale)}`);

ws.close();
chrome.kill();
process.exit(0);
