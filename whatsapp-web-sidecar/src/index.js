import crypto from 'node:crypto';
import express from 'express';
import fs from 'node:fs';
import path from 'node:path';
import QRCode from 'qrcode';
import pkg from 'whatsapp-web.js';

const { Client, List, LocalAuth } = pkg;

const PORT = Number(process.env.PORT || 3000);
const SHARED_SECRET = process.env.SHARED_SECRET;
const GO_BACKEND_URL = process.env.GO_BACKEND_URL;
const SESSION_PATH = process.env.SESSION_PATH || '/data/wa-session';
const INBOUND_PATH = process.env.INBOUND_PATH || '/wa-web/inbound';
// Group expense splitting is enabled by default. Set ALLOW_GROUPS=false only to
// temporarily pause group traffic without unlinking the WhatsApp account.
const ALLOW_GROUPS = String(process.env.ALLOW_GROUPS || 'true').toLowerCase() === 'true';
const QR_MAX_RETRIES = Math.max(1, Number.parseInt(process.env.QR_MAX_RETRIES || '1', 10) || 1);
const INBOUND_CHAT_JIDS_PATH = path.join(SESSION_PATH, 'inbound-chat-jids.json');

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
// (`@lid`). Persist the real JID on the Railway volume so direct debt
// reminders still work after a sidecar restart.
const inboundChatJIDs = loadInboundChatJIDs();

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

    const rawId = jidToRawID(msg.from);
    const chatId = Number(rawId);
    if (!Number.isSafeInteger(chatId)) {
      console.warn('Cannot parse chat id from', msg.from);
      return;
    }

    const senderJid = isGroup ? msg.author : msg.from;
    const senderRawID = jidToRawID(senderJid);
    const senderId = Number(senderRawID);
    if (!Number.isSafeInteger(senderId)) {
      console.warn('Cannot parse sender id from', senderJid);
      return;
    }

    rememberInboundChat(chatId, isGroup ? 'group' : 'private', msg.from);
    if (!isGroup) {
      rememberInboundChat(senderId, 'private', senderJid);
    }

    let displayName = senderRawID;
    try {
      const contact = await msg.getContact();
      displayName = contact.pushname || contact.name || contact.number || senderRawID;
    } catch (e) {
      console.warn('Could not fetch contact for', msg.from, e.message);
    }

    let chatName = '';
    if (isGroup) {
      try {
        const chat = await msg.getChat();
        chatName = chat.name || '';
      } catch (e) {
        console.warn('Could not fetch group metadata for', msg.from, e.message);
      }
    }

    const interactiveID = getInteractiveID(msg);
    const isMentioned = isGroup && messageMentionsClient(msg);

    const payload = {
      chat_id: chatId,
      raw_jid: msg.from,
      chat_type: isGroup ? 'group' : 'private',
      chat_name: chatName,
      sender_id: senderId,
      sender_jid: senderJid,
      from_name: displayName,
      text: msg.body || '',
      interactive_id: interactiveID,
      is_mentioned: isMentioned,
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

  const { chat_id, chat_type, text, interactive } = req.body || {};
  if (!chat_id || !text) {
    return res.status(400).json({ ok: false, error: 'missing chat_id or text' });
  }

  const jid = chat_type === 'group'
    ? `${chat_id}@g.us`
    : inboundChatJIDs.get(chatKey(chat_id, 'private')) ?? `${chat_id}@c.us`;

  try {
    const sanitized = stripHtmlForWhatsApp(String(text));
    if (isListRequest(interactive)) {
      try {
        const list = new List(
          sanitized,
          String(interactive.button_text || 'Ver opciones'),
          [{ title: String(interactive.title || 'Splitter'), rows: interactive.rows }],
          String(interactive.title || 'Splitter'),
          String(interactive.footer || ''),
        );
        await client.sendMessage(jid, list);
        return res.json({ ok: true, interactive: 'list' });
      } catch (interactiveError) {
        // Interactive messages are not equally supported by every WhatsApp Web
        // build. The numbered text sent by Go remains a fully functional
        // fallback, so do not fail a user action merely because the list did.
        console.warn('Interactive list failed; sending text fallback:', interactiveError.message);
      }
    }
    await client.sendMessage(jid, sanitized);
    res.json({ ok: true, interactive: 'text' });
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

function isListRequest(interactive) {
  return interactive
    && interactive.type === 'list'
    && Array.isArray(interactive.rows)
    && interactive.rows.length > 0
    && interactive.rows.length <= 10
    && interactive.rows.every((row) => row && typeof row.id === 'string' && typeof row.title === 'string');
}

function getInteractiveID(msg) {
  // whatsapp-web.js has exposed these fields under different names across
  // WhatsApp Web versions. Keep the extraction defensive so a normal text
  // message can never be interpreted as a bot action.
  const candidates = [
    msg.selectedRowId,
    msg.selectedButtonId,
    msg.rawData?.selectedRowId,
    msg.rawData?.selectedButtonId,
    msg.rawData?.listResponseMessage?.singleSelectReply?.selectedRowId,
    msg.rawData?.buttonsResponseMessage?.selectedButtonId,
  ];
  return candidates.find((value) => typeof value === 'string' && value.length > 0) || '';
}

function messageMentionsClient(msg) {
  const ownJid = client.info?.wid?._serialized;
  if (!ownJid || !Array.isArray(msg.mentionedIds)) return false;
  return msg.mentionedIds.some((mentionedJid) => sameWhatsAppIdentity(mentionedJid, ownJid));
}

function sameWhatsAppIdentity(left, right) {
  const leftRaw = jidToRawID(left);
  const rightRaw = jidToRawID(right);
  return left === right || (leftRaw !== '' && leftRaw === rightRaw);
}

function chatKey(chatId, chatType) {
  return `${chatType}:${chatId}`;
}

function jidToRawID(jid) {
  return String(jid || '').replace(/@(c\.us|g\.us|lid)$/, '');
}

function rememberInboundChat(chatId, chatType, jid) {
  inboundChatJIDs.set(chatKey(chatId, chatType), jid);
  if (chatType === 'private') {
    persistInboundChatJIDs();
  }
}

function loadInboundChatJIDs() {
  try {
    const raw = fs.readFileSync(INBOUND_CHAT_JIDS_PATH, 'utf8');
    const entries = JSON.parse(raw);
    if (!Array.isArray(entries)) {
      throw new Error('expected an array');
    }
    return new Map(entries.filter(([key, jid]) => typeof key === 'string' && typeof jid === 'string'));
  } catch (error) {
    if (error.code !== 'ENOENT') {
      console.warn('Could not restore persisted chat addresses:', error.message);
    }
    return new Map();
  }
}

function persistInboundChatJIDs() {
  const temporaryPath = `${INBOUND_CHAT_JIDS_PATH}.tmp`;
  try {
    fs.mkdirSync(SESSION_PATH, { recursive: true });
    fs.writeFileSync(temporaryPath, JSON.stringify([...inboundChatJIDs]), { mode: 0o600 });
    fs.renameSync(temporaryPath, INBOUND_CHAT_JIDS_PATH);
  } catch (error) {
    console.warn('Could not persist chat addresses:', error.message);
    try { fs.rmSync(temporaryPath, { force: true }); } catch (_) {}
  }
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
