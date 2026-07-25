# LinkedIn Post: SplitBot - Bot de División de Gastos

---

## El Problema

Cada vez que salgo con amigos surge el mismo drama: "¿Quién pagó qué?", "¿Cuánto te debo?", "¿Ya me pagaste?". Las apps existentes como Splitwise tienen planes premium, requieren que todos se bajen la app, y son overkill para dividir una pizza.

Quería algo simple, gratis, y que funcione donde ya estamos: **Telegram**.

---

## La Solución

Construí **SplitBot**: un bot de Telegram 100% gratuito para dividir gastos grupales, con backend serverless en AWS que corre prácticamente gratis.

**Arquitectura:**
- **Backend**: Go 1.21+ (Lambda) - respuestas en milisegundos
- **Database**: DynamoDB (25GB gratis siempre)
- **API**: API Gateway + Webhook de Telegram
- **IaC**: Terraform
- **CI/CD**: GitHub Actions (deploy automático)

**Comandos del Bot:**
```
/nuevo_gasto [desc] [monto]  → Crear gasto
/ver_gastos                  → Ver gastos del grupo
/dividir [id]                → Dividir entre participantes
/mis_deudas                  → Ver lo que debés
/pagar [id]                  → Marcar como pagado
/balance                     → Balance grupal
```

**Seguridad:**
- Encriptación de datos sensibles
- Rate limiting para prevenir abuse
- Validación estricta de inputs
- Sin almacenamiento de mensajes personales

---

## Stack Tecnológico

```
Backend:      Go 1.21+
Database:     DynamoDB
Compute:      AWS Lambda
API:          API Gateway
IaC:          Terraform
CI/CD:        GitHub Actions
Frontend:     React (landing page en Netlify)
```

---

## Costos Operativos

| Recurso | Free Tier |
|---------|-----------|
| Lambda | 1M req/mes gratis |
| API Gateway | 1M req/mes gratis |
| DynamoDB | 25GB gratis siempre |
| **Total** | **$0/mes** |

---

## CI/CD Pipeline

El workflow de GitHub Actions:
1. Compila el backend en Go
2. Ejecuta Terraform para crear/actualizar infraestructura
3. Configura el webhook de Telegram automáticamente
4. (Opcional) Despliega landing page a Netlify

Un `git push` = deploy completo.

---

## Texto para LinkedIn (copiar y pegar)

```
"¿Cuánto te debo?" - La pregunta que arruina cualquier salida con amigos.

Cansado de apps de gastos compartidos con planes premium y features que nadie usa, construí mi propia solución: SplitBot.

El problema:
• Apps existentes requieren que todos se la bajen
• Planes premium para features básicas
• Overkill para dividir una cena

La solución:
• Bot de Telegram - donde ya estamos todos
• 100% gratuito - sin planes premium
• Backend en Go - respuestas en milisegundos
• Serverless en AWS - costo operativo $0/mes

Stack:
• Go 1.21 + AWS Lambda
• DynamoDB (25GB free tier permanente)
• Terraform + GitHub Actions (CI/CD automático)

Comandos simples:
/nuevo_gasto Pizza 5000
/dividir 1
/mis_deudas

Un git push = deploy completo a producción.

A veces la mejor solución es la más simple.

#Golang #AWS #Serverless #Telegram #Lambda #DynamoDB #Terraform #SideProject
```

---

## Hashtags Recomendados

#Golang #AWS #Serverless #Telegram #Lambda #DynamoDB #Terraform #GitHubActions #SideProject #FinTech #Bot #OpenSource
