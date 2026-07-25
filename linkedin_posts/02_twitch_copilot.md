# LinkedIn Post: Twitch Chat Copilot

---

## El Problema

Los streamers grandes de Twitch enfrentan un problema real: con miles de mensajes por minuto en el chat, es imposible leer todo. Las preguntas interesantes se pierden en un mar de spam, emotes y mensajes repetidos.

¿Cómo identificar los mensajes que realmente importan sin perderse en el ruido?

---

## La Solución

Construí **Twitch Chat Copilot**: un asistente en tiempo real que ingiere mensajes vía EventSub webhooks, los puntúa por relevancia usando un modelo de scoring, y entrega los highlights a un dashboard web vía WebSocket.

**Arquitectura:**
- **Backend**: FastAPI + PostgreSQL (async) + Redis
- **Frontend**: Next.js 14 + TypeScript + Tailwind
- **Integración**: Twitch EventSub (webhooks HTTPS)
- **Real-time**: WebSocket para streaming de mensajes

**Sistema de Scoring:**
```
score = Σ(base × weight) + Σ(penalties × spam_multiplier)

Bonificaciones:
  • Broadcaster badge    +5.0
  • Moderator badge      +3.0
  • Mención al streamer  +4.0
  • Pregunta detectada   +3.0
  • Keyword match        +2.0

Penalizaciones:
  • Links               -2.0
  • Mensaje muy corto   -1.0
  • Burst (>5 en 30s)   -0.5 × (burst-5)
  • Copypasta           score × 0.3
```

**Anti-spam con Redis:**
- Deduplicación por Message-Id
- Rate limiting por usuario
- Detección de copypasta (hash de mensajes)
- Tracking de términos trending (buckets de 30s)

---

## Stack Tecnológico

```
Backend:      Python 3.11 + FastAPI + uvicorn
Database:     PostgreSQL 16 (SQLAlchemy async + Alembic)
Cache:        Redis 7 (anti-spam + deduplicación)
Frontend:     Next.js 14 (App Router) + TypeScript + Tailwind
Auth:         Twitch OAuth2 + JWT
Infra:        Docker Compose
```

---

## Features Destacados

1. **Presets configurables**: balanced, stream_grande, qa, sorteo
2. **Cola de respuestas**: Marca mensajes para responder después
3. **Pulse en tiempo real**: Términos trending en el chat
4. **GDPR compliant**: Retención configurable (1-30 días), delete endpoint
5. **Simulador de chat**: Script Python para testing sin Twitch real

---

## Compliance con Twitch

- Retención por defecto: 7 días
- `save_chat = false` por defecto (solo highlights)
- Endpoint de borrado de datos (`DELETE /api/v1/user/data`)
- Revocación de tokens en logout
- Limpieza de EventSub subscriptions

---

## Texto para LinkedIn (copiar y pegar)

```
¿Cómo lee un streamer de Twitch 500 mensajes por minuto sin perderse las preguntas importantes?

Ese fue el problema que me propuse resolver con Twitch Chat Copilot.

El desafío:
• Streams grandes = miles de mensajes/minuto
• Preguntas interesantes se pierden en el ruido
• Spam, copypasta y emotes dominan el chat

La solución:
• Sistema de scoring en tiempo real que prioriza mensajes relevantes
• Anti-spam con Redis (deduplicación, rate limiting, detección de copypasta)
• Dashboard web con WebSocket para ver highlights instantáneamente
• Presets configurables según el tipo de stream

Stack técnico:
• Backend: FastAPI + PostgreSQL async + Redis
• Frontend: Next.js 14 + TypeScript
• Integración: Twitch EventSub webhooks
• Real-time: WebSocket streaming

El modelo de scoring considera:
✓ Badges (broadcaster, mod)
✓ Menciones y preguntas
✓ Keywords personalizados
✗ Penaliza spam, links, mensajes cortos

Resultado: De 500 mensajes/min a los 10-20 que realmente importan.

#Twitch #Streaming #FastAPI #NextJS #RealTime #WebSocket #Python #TypeScript
```

---

## Hashtags Recomendados

#Twitch #Streaming #FastAPI #NextJS #RealTime #WebSocket #Python #TypeScript #Redis #PostgreSQL #SideProject #ContentCreators #LiveStreaming
