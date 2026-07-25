# LinkedIn Post: Sistema ETL de Gastos Personales

---

## El Problema

Cada mes me encontraba con el mismo problema: mis gastos estaban dispersos en múltiples fuentes - notificaciones del banco por email, tickets de supermercado en PDF, reportes de MercadoPago - y no tenía una forma unificada de analizarlos.

Quería responder preguntas simples como "¿Cuánto gasté este mes en el super?" o "¿Cuántas cuotas tengo activas?" sin tener que revisar manualmente decenas de emails.

---

## La Solución

Construí un **pipeline ETL serverless y event-driven** que procesa automáticamente todos mis gastos en tiempo real:

**Arquitectura Multi-Cloud:**
- **AWS Lambda** (10 funciones) para procesamiento serverless
- **AWS Step Functions** para orquestación de flujos ETL
- **Google BigQuery** como Data Warehouse centralizado
- **Pub/Sub + Gmail API** para captura de eventos en tiempo real

**Flujos de Datos:**
1. **Gastos Bancarios**: Gmail → Pub/Sub → Lambda → BigQuery
2. **Tickets de Supermercado**: PDF → OCR (pdfplumber) → Parsing → BigQuery  
3. **MercadoPago**: Webhook → Validación bcrypt → ETL → BigQuery

**El plus**: Un **chatbot de Telegram con IA** que genera SQL automáticamente con GPT-4 para responder preguntas en lenguaje natural sobre mis finanzas.

---

## Stack Tecnológico

```
Cloud:        AWS (Lambda, Step Functions, S3, DynamoDB, API Gateway)
              GCP (BigQuery, Pub/Sub, Gmail API)
IaC:          Terraform
CI/CD:        GitHub Actions (matrix build para 10 imágenes Docker)
Lenguaje:     Python 3.11
AI:           OpenAI GPT-4o-mini
Bot:          Telegram Bot API
```

---

## Resultados

- **Costo operativo**: ~$1.50/mes (aprovechando free tiers)
- **Procesamiento**: Tiempo real (event-driven)
- **Automatización**: 100% - desde la llegada del email hasta BigQuery
- **Queries**: Lenguaje natural vía Telegram → SQL → Respuesta

---

## Aprendizajes Clave

1. **Event-driven > Batch**: Pub/Sub + webhooks permiten procesamiento inmediato
2. **Step Functions**: Excelente para orquestar ETLs con manejo de errores (compensation flows)
3. **Multi-cloud funciona**: AWS para compute, GCP para data warehouse - cada uno en lo que es mejor
4. **IaC es imprescindible**: Terraform + GitHub Actions = deploys reproducibles
5. **AI como interfaz**: GPT-4 genera SQL sorprendentemente bien con el schema correcto

---

## Texto para LinkedIn (copiar y pegar)

```
¿Alguna vez quisiste saber exactamente cuánto gastaste este mes sin revisar 50 emails diferentes?

Ese problema me llevó a construir mi propio sistema ETL serverless para finanzas personales.

El problema:
• Gastos dispersos en emails del banco, PDFs del super, reportes de MercadoPago
• Sin forma unificada de analizarlos
• Preguntas simples requerían trabajo manual

La solución:
• Pipeline event-driven que procesa gastos en tiempo real
• 10 Lambda functions orquestadas con Step Functions
• BigQuery como single source of truth
• Chatbot de Telegram con GPT-4 para consultas en lenguaje natural

Stack: AWS Lambda, Step Functions, S3, API Gateway | GCP BigQuery, Pub/Sub | Terraform | Python | OpenAI

Costo operativo: ~$1.50/mes

El resultado: "¿Cuánto gasté en Carrefour este mes?" → Respuesta instantánea en Telegram.

A veces los mejores proyectos nacen de resolver tus propios problemas.

#DataEngineering #AWS #GCP #Serverless #ETL #Python #SideProject
```

---

## Hashtags Recomendados

#DataEngineering #AWS #GCP #Serverless #ETL #Python #Terraform #BigQuery #Lambda #StepFunctions #SideProject #FinTech #PersonalFinance #OpenAI #ChatGPT
