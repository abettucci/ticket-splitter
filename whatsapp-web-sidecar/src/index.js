import crypto from 'node:crypto';
import express from 'express';
import fs from 'node:fs';
import path from 'node:path';
import QRCode from 'qrcode';
import pkg from 'whatsapp-web.js';

const { Client, LocalAuth } = pkg;

const PORT = Number(process.env.PORT || 3000);
const SHARED_SECRET = process.env.SHARED_SECRET;
const GO_BACKEND_URL = process.env.GO_BACKEND_URL;
const SESSION_PATH = process.env.SESSION_PATH || '/data/wa-session';
const INBOUND_PATH = process.env.INBOUND_PATH || '/wa-web/inbound';
const ALLOW_GROUPS = String(process.env.ALLOW_GROUPS || 'false').toLowerCase() === 'true';
const QR_MAX_RETRIES = Math.max(1, Number.parseInt(process.env.QR_MAX_RETRIES || '1', 10) || 1);

if (!SHARED_SECRET) {
  console.error('FATAL: SHARED_SECRET is required');
  process.exit(1);
}
if (!GO_BACKEND_URL) {
  console.error('FATAL: GO_BACKEND_URL is required');
  process.exit(1);
}

// Railway restarts can leave Chromium's singleton files in the persistent
// LocalAuth profile. They are process locks, not WhatsApp credentials; keeping
// them makes Chromium believe a browser is still running on another host.
clearStaleChromiumLocks();

const client = new Client({
  authStrategy: new LocalAuth({ dataPath: SESSION_PATH }),
  // Avoid endless QR regeneration while an account is in WhatsApp's cooldown.
  // After this limit the process stays up, but a manual Railway redeploy is
  // required before another QR can be requested.
  qrMaxRetries: QR_MAX_RETRIES,
  puppeteer: {
    headless: true,
    args: [
      '--no-sandbox',
      '--disable-setuid-sandbox',
      '--disable-dev-shm-usage',
      '--disable-accelerated-2d-canvas',
      '--no-first-run',
      '--no-zygote',
      '--disable-gpu',
    ],
    executablePath: process.env.PUPPETEER_EXECUTABLE_PATH || undefined,
  },
});

let isReady = false;
let pendingQR = null;
let linkingPaused = false;
// WhatsApp may identify private chats by phone (`@c.us`) or by a Linked ID
// (`@lid`). Keep the real JID received from WhatsApp so replies target the
// same conversation instead of assuming every private chat is `@c.us`.
const inboundChatJIDs = new Map();

client.on('qr', (qr) => {
  linkingPaused = false;
  pendingQR = qr;
  console.log('WhatsApp QR available at the protected /qr endpoint.');
});

client.on('authenticated', () => {
  console.log('WhatsApp authenticated; session persisted to', SESSION_PATH);
});

client.on('auth_failure', (msg) => {
  console.error('Auth failure:', msg);
});

client.on('ready', () => {
  isReady = true;
  pendingQR = null;
  console.log('WhatsApp client ready');
});

client.on('disconnected', (reason) => {
  isReady = false;
  pendingQR = null;
  linkingPaused = reason === 'Max qrcode retries reached';
  console.error('WhatsApp client disconnected:', reason);
});

client.on('message', async (msg) => {
  try {
    if (msg.fromMe) return;
    if (msg.from === 'status@broadcast') return;

    const isGroup = msg.from.endsWith('@g.us');
    if (isGroup && !ALLOW_GROUPS) return;

    const rawId = msg.from.replace(/@(c\.us|g\.us|lid)$/, '');
    const chatId = Number(rawId);
    if (!Number.isSafeInteger(chatId)) {
      console.warn('Cannot parse chat id from', msg.from);
      return;
    }
    inboundChatJIDs.set(chatKey(chatId, isGroup ? 'group' : 'private'), msg.from);

    let displayName = rawId;
    try {
      const contact = await msg.getContact();
      displayName = contact.pushname || contact.name || contact.number || rawId;
    } catch (e) {
      console.warn('Could not fetch contact for', msg.from, e.message);
    }

    const payload = {
      chat_id: chatId,
      raw_jid: msg.from,
      chat_type: isGroup ? 'group' : 'private',
      from_name: displayName,
      text: msg.body || '',
      message_id: msg.id?._serialized || '',
      timestamp: msg.timestamp || Math.floor(Date.now() / 1000),
    };

    await postToBackend(payload);
  } catch (e) {
    console.error('Error handling inbound message:', e);
  }
});

async function postToBackend(payload) {
  const body = JSON.stringify(payload);
  const sig = crypto.createHmac('sha256', SHARED_SECRET).update(body).digest('hex');
  const url = GO_BACKEND_URL.replace(/\/$/, '') + INBOUND_PATH;

  try {
    const res = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Webhook-Signature': sig,
      },
      body,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => '');
      console.error('Backend returned', res.status, text);
    }
  } catch (e) {
    console.error('Failed to POST to backend:', e.message);
  }
}

const app = express();
app.use(express.json({ limit: '256kb' }));

app.get('/health', (_req, res) => {
  res.json({ ok: true, ready: isReady, linkingPaused });
});

// The QR grants a new device access to the linked WhatsApp account. Keep it
// private: browsers prompt for HTTP Basic credentials, avoiding a secret in a
// URL, history, or Railway logs. User: splitbot; password: SHARED_SECRET.
app.get('/qr', requireQRAuth, async (_req, res) => {
  if (!pendingQR) {
    if (linkingPaused) {
      return res.status(429).json({
        ok: false,
        error: 'QR retry limit reached; wait for WhatsApp cooldown, then redeploy to request a new QR',
      });
    }
    return res.status(isReady ? 410 : 404).json({
      ok: false,
      error: isReady ? 'no QR pending; WhatsApp is already linked' : 'QR not available yet',
    });
  }

  try {
    const svg = await QRCode.toString(pendingQR, {
      type: 'svg',
      errorCorrectionLevel: 'M',
      margin: 2,
      color: { dark: '#111827', light: '#ffffff' },
    });
    res.set({
      'Cache-Control': 'no-store, no-cache, must-revalidate, private',
      Pragma: 'no-cache',
      'Referrer-Policy': 'no-referrer',
    });
    res.type('image/svg+xml').send(svg);
  } catch (error) {
    console.error('Failed to render WhatsApp QR:', error);
    res.status(500).json({ ok: false, error: 'could not render QR' });
  }
});

app.post('/send', async (req, res) => {
  const auth = req.headers.authorization || '';
  if (auth !== `Bearer ${SHARED_SECRET}`) {
    return res.status(401).json({ ok: false, error: 'unauthorized' });
  }
  if (!isReady) {
    return res.status(503).json({ ok: false, error: 'client not ready' });
  }

  const { chat_id, chat_type, text } = req.body || {};
  if (!chat_id || !text) {
    return res.status(400).json({ ok: false, error: 'missing chat_id or text' });
  }

  const jid = chat_type === 'group'
    ? `${chat_id}@g.us`
    : inboundChatJIDs.get(chatKey(chat_id, 'private')) ?? `${chat_id}@c.us`;

  try {
    const sanitized = stripHtmlForWhatsApp(String(text));
    await client.sendMessage(jid, sanitized);
    res.json({ ok: true });
  } catch (e) {
    console.error('Send failed:', e);
    res.status(500).json({ ok: false, error: e.message });
  }
});

function stripHtmlForWhatsApp(text) {
  return text
    .replace(/<b>(.*?)<\/b>/g, '*$1*')
    .replace(/<i>(.*?)<\/i>/g, '_$1_')
    .replace(/<code>(.*?)<\/code>/g, '`$1`')
    .replace(/<[^>]+>/g, '')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>');
}

function chatKey(chatId, chatType) {
  return `${chatType}:${chatId}`;
}

function requireQRAuth(req, res, next) {
  const expected = `Basic ${Buffer.from(`splitbot:${SHARED_SECRET}`).toString('base64')}`;
  const provided = req.headers.authorization || '';
  const isAuthorized = provided.length === expected.length
    && crypto.timingSafeEqual(Buffer.from(provided), Buffer.from(expected));

  if (!isAuthorized) {
    res.set('WWW-Authenticate', 'Basic realm="SplitBot WhatsApp QR", charset="UTF-8"');
    return res.status(401).send('Authentication required');
  }

  next();
}

function clearStaleChromiumLocks() {
  const profileDir = path.join(SESSION_PATH, 'session');
  for (const lockName of ['SingletonLock', 'SingletonCookie', 'SingletonSocket']) {
    try {
      fs.rmSync(path.join(profileDir, lockName), { force: true });
    } catch (error) {
      console.warn(`Could not clear stale Chromium ${lockName}:`, error.message);
    }
  }
}

app.listen(PORT, () => console.log(`Sidecar listening on :${PORT}`));

console.log('Initializing WhatsApp client...');
client.initialize().catch((error) => {
  console.error('WhatsApp client failed to initialize:', error);
  process.exit(1);
});

process.on('SIGTERM', async () => {
  console.log('SIGTERM received, shutting down gracefully');
  try { await client.destroy(); } catch (_) {}
  process.exit(0);
});
