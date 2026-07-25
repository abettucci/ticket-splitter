# LinkedIn Post: Procesador de Tickets con Google Drive + AI

---

## El Problema

Tengo la costumbre de sacarle foto a todos mis tickets de compra, pero después quedan olvidados en el carrete del celular. Quería una forma de:

1. Subir la foto a algún lado
2. Que automáticamente extraiga fecha, comercio e importe
3. Que lo guarde en una planilla para análisis

Sin intervención manual. Solo subir la foto y listo.

---

## La Solución

Construí un sistema que monitorea una carpeta de Google Drive y cuando detecta una nueva imagen, la procesa con **GPT-4 Vision** para extraer los datos del ticket automáticamente.

**Flujo:**
```
📱 Subo foto a carpeta de Drive
        ↓
🔍 Google Drive detecta el cambio (Push Notification)
        ↓
🌐 Webhook a AWS API Gateway
        ↓
⚡ Lambda descarga imagen + llama a GPT-4 Vision
        ↓
📊 Extrae: fecha, comercio, importe
        ↓
📝 Guarda en Google Sheets + DynamoDB (deduplicación)
```

**Componentes:**
- **Google Drive API**: Watch notifications (push)
- **AWS Lambda**: Procesamiento serverless
- **OpenAI GPT-4 Vision**: OCR + extracción de datos
- **Google Sheets**: Destino final de los datos
- **DynamoDB**: Control de duplicados
- **EventBridge**: Renovación automática del watcher (cada 6 días)

---

## Stack Tecnológico

```
Cloud:        AWS (Lambda, API Gateway, DynamoDB, Secrets Manager)
Storage:      Google Drive
AI/OCR:       OpenAI GPT-4 Vision
Output:       Google Sheets
IaC:          Terraform
Scheduling:   EventBridge (renovación de watchers)
```

---

## Arquitectura

```
┌─────────────────┐     ┌─────────────────┐     ┌─────────────────┐
│  Google Drive   │────▶│  API Gateway    │────▶│     Lambda      │
│  (Watch folder) │     │  (Webhook)      │     │  (Processor)    │
└─────────────────┘     └─────────────────┘     └─────────────────┘
                                                        │
                        ┌───────────────────────────────┼───────────────────────────────┐
                        ▼                               ▼                               ▼
                ┌─────────────────┐             ┌─────────────────┐             ┌─────────────────┐
                │  GPT-4 Vision   │             │    DynamoDB     │             │  Google Sheets  │
                │  (OCR + Parse)  │             │  (Deduplicación)│             │  (Output)       │
                └─────────────────┘             └─────────────────┘             └─────────────────┘
```

---

## Detalles Técnicos

**¿Por qué EventBridge cada 6 días?**
- Los watchers de Google Drive expiran en 7 días
- Renovar cada 6 días garantiza continuidad
- Si falla una ejecución, queda 1 día de margen

**Seguridad:**
- Validación del usuario que sube el archivo
- Alertas SNS si alguien no autorizado sube archivos
- Credenciales en AWS Secrets Manager

**Costos estimados:**
- AWS: ~$1.31/mes
- OpenAI (GPT-4 Vision): ~$5/mes (100 imágenes)
- **Total: ~$6.31/mes**

---

## Texto para LinkedIn (copiar y pegar)

```
¿Qué pasa con las fotos de tickets que sacamos y nunca volvemos a mirar?

Quería convertir esas fotos olvidadas en datos útiles, sin trabajo manual.

El problema:
• Fotos de tickets acumulándose en el celular
• Datos valiosos (fecha, comercio, monto) sin digitalizar
• Proceso manual = nunca se hace

La solución:
• Carpeta de Google Drive monitoreada 24/7
• Cuando subo una foto → webhook automático
• GPT-4 Vision extrae fecha, comercio e importe
• Datos van directo a Google Sheets

Arquitectura:
• Google Drive Push Notifications → AWS API Gateway
• Lambda procesa con OpenAI GPT-4 Vision
• DynamoDB para evitar duplicados
• EventBridge renueva el watcher cada 6 días

Costo: ~$6/mes (incluyendo OpenAI)

Flujo del usuario:
📱 Saco foto del ticket
📤 La subo a Drive
✅ Listo. Datos en mi planilla.

A veces la mejor automatización es la que no requiere pensar.

#Automation #GoogleDrive #OpenAI #GPT4Vision #AWS #Lambda #Serverless #SideProject
```

---

## Hashtags Recomendados

#Automation #GoogleDrive #OpenAI #GPT4Vision #AWS #Lambda #Serverless #OCR #Terraform #SideProject #PersonalFinance
