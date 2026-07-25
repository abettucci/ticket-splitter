# LinkedIn Post: Ticket Splitter - División de Gastos con Calendario de Pagos

---

## El Problema

SplitBot (mi bot de Telegram para dividir gastos) funcionaba bien, pero faltaba algo: **¿cuándo me van a pagar?**

Cuando dividís gastos con amigos, no todos pagan al instante. Algunos te dicen "te pago el viernes", otros "cuando cobre". Necesitaba:

1. Trackear no solo cuánto me deben, sino **cuándo** me van a pagar
2. Recordatorios automáticos cuando se acerca la fecha
3. Una vista de calendario con todos los pagos pendientes

---

## La Solución

Evolucioné SplitBot a **Ticket Splitter**: una versión con frontend web completo, calendario de pagos, y sistema de recordatorios.

**Nuevas Features:**
- **Calendario de Pagos**: Vista mensual de cuándo esperar cada pago
- **Recordatorios Automáticos**: Notificaciones antes de la fecha acordada
- **Dashboard Web**: React + Tailwind para gestión visual
- **Reminder Worker**: Cloudflare Worker que envía recordatorios

**Arquitectura:**
```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│   Frontend      │────▶│    Backend      │────▶│    Database     │
│   React/Vite    │     │    (API)        │     │                 │
│   Tailwind      │     │                 │     │                 │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                │
                                ▼
                        ┌─────────────────┐
                        │ Reminder Worker │
                        │ (Cloudflare)    │
                        └─────────────────┘
```

---

## Stack Tecnológico

```
Frontend:     React + Vite + TypeScript
Styling:      Tailwind CSS + shadcn/ui
Backend:      Node.js / Go
Workers:      Cloudflare Workers (recordatorios)
IaC:          Terraform
CI/CD:        GitHub Actions
```

---

## Calendario de Pagos

El calendario muestra:
- **Pagos pendientes** por fecha acordada
- **Pagos vencidos** (en rojo)
- **Pagos completados** (en verde)
- **Resumen mensual** de entradas/salidas esperadas

Cada entrada incluye:
- Quién te debe / a quién le debés
- Monto
- Descripción del gasto original
- Estado (pendiente/pagado/vencido)

---

## Sistema de Recordatorios

**Reminder Worker (Cloudflare):**
- Corre cada hora
- Busca pagos con fecha próxima (configurable: 1 día, 3 días, 1 semana)
- Envía notificación al deudor
- Marca el recordatorio como enviado (evita spam)

**Canales de notificación:**
- Push notification (si tiene la PWA instalada)
- Email (fallback)
- Telegram (integración con el bot original)

---

## Texto para LinkedIn (copiar y pegar)

```
"Te pago el viernes" - La promesa que todos hacemos y nadie trackea.

Después de construir un bot para dividir gastos, me di cuenta que faltaba lo más importante: saber CUÁNDO te van a pagar.

El problema:
• Dividir gastos es fácil, cobrarlos es difícil
• "Te pago cuando cobre" = fecha indefinida
• Sin recordatorios = pagos olvidados

La solución: Ticket Splitter
• Calendario visual de pagos pendientes
• Fechas acordadas para cada deuda
• Recordatorios automáticos antes del vencimiento
• Dashboard web para gestión completa

Stack:
• Frontend: React + Vite + Tailwind + shadcn/ui
• Reminder Worker: Cloudflare Workers
• Infra: Terraform + GitHub Actions

Features del calendario:
📅 Vista mensual de pagos esperados
🔴 Pagos vencidos destacados
✅ Historial de pagos completados
🔔 Recordatorios configurables (1 día, 3 días, 1 semana)

De "te pago el viernes" a un sistema que realmente lo trackea.

#React #TypeScript #Tailwind #CloudflareWorkers #SideProject #FinTech
```

---

## Hashtags Recomendados

#React #TypeScript #Tailwind #CloudflareWorkers #Vite #ShadcnUI #SideProject #FinTech #WebApp #Terraform #GitHubActions
