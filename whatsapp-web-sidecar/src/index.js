import crypto from 'node:crypto';
import express from 'express';
import qrcode from 'qrcode-terminal';
import pkg from 'whatsapp-web.js';

const { Client, LocalAuth } = pkg;

const PORT = Number(process.env.PORT || 3000);
const SHARED_SECRET = process.env.SHARED_SECRET;
const GO_BACKEND_URL = process.env.GO_BACKEND_URL;
const SESSION_PATH = process.env.SESSION_PATH || '/data/wa-session';
const INBOUND_PATH = process.env.INBOUND_PATH || '/wa-web/inbound';
const ALLOW_GROUPS = String(process.env.ALLOW_GROUPS || 'false').toLowerCase() === 'true';

if (!SHARED_SECRET) {
  console.error('FATAL: SHARED_SECRET is required');
  process.exit(1);
}
if (!GO_BACKEND_URL) {
  console.error('FATAL: GO_BACKEND_URL is required');
  process.exit(1);
}

const client = new Client({
  authStrategy: new LocalAuth({ dataPath: SESSION_PATH }),
  puppeteer: {
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

client.on('qr', (qr) => {
  console.log('==== SCAN THIS QR WITH WHATSAPP (Linked devices > Link a device) ====');
  qrcode.generate(qr, { small: true });
  console.log('QR also raw (for backup):', qr);
});

client.on('authenticated', () => {
  console.log('WhatsApp authenticated; session persisted to', SESSION_PATH);
});

client.on('auth_failure', (msg) => {
  console.error('Auth failure:', msg);
});

client.on('ready', () => {
  isReady = true;
  console.log('WhatsApp client ready');
});

client.on('disconnected', (reason) => {
  isReady = false;
  console.error('WhatsApp client disconnected:', reason);
});

client.on('message', async (msg) => {
  try {
    if (msg.fromMe) return;
    if (msg.from === 'status@broadcast') return;

    const isGroup = msg.from.endsWith('@g.us');
    if (isGroup && !ALLOW_GROUPS) return;

    const rawId = msg.from.replace(/@(c|g)\.us$/, '');
    const chatId = Number(rawId);
    if (!Number.isFinite(chatId)) {
      console.warn('Cannot parse chat id from', msg.from);
      return;
    }

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
  res.json({ ok: true, ready: isReady });
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

  const suffix = chat_type === 'group' ? '@g.us' : '@c.us';
  const jid = `${chat_id}${suffix}`;

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

app.listen(PORT, () => console.log(`Sidecar listening on :${PORT}`));

console.log('Initializing WhatsApp client...');
client.initialize();

process.on('SIGTERM', async () => {
  console.log('SIGTERM received, shutting down gracefully');
  try { await client.destroy(); } catch (_) {}
  process.exit(0);
});
