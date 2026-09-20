# V2 Cobros con Mercado Pago

## Objetivo

Permitir que una transferencia pendiente de una división web se pague con Mercado Pago y se marque como pagada solamente cuando Mercado Pago confirme que el cobro fue aprobado.

La V1 seguirá siendo una herramienta sin registro y local al navegador. Sus divisiones, gastos y pagos manuales viven en `localStorage`; no habrá credenciales de pago ni comunicación con el backend.

## Alcance de la V1 web

- Una división contiene participantes y múltiples gastos.
- Un gasto admite uno o más pagadores y se reparte de forma equitativa o por importes exactos entre los participantes elegidos.
- El resumen simplifica las deudas en transferencias. Cada transferencia puede marcarse manualmente como pagada y el balance se recalcula.
- Las divisiones se guardan en el navegador y se pueden exportar como JSON. El resultado se puede copiar o compartir como texto.

## Experiencia prevista para V2

1. En una transferencia pendiente, el deudor elige **Pagar con Mercado Pago**.
2. La web solicita al backend la creación de una orden de pago vinculada de forma única a esa transferencia.
3. El backend crea el checkout de Mercado Pago con una referencia externa que identifica la transferencia, pero no contiene nombres ni información sensible.
4. El deudor completa el pago en Mercado Pago y vuelve a la web. El retorno solo informa el estado visible al usuario; no modifica el balance.
5. Mercado Pago notifica al backend por webhook. El backend valida la firma, consulta el recurso de pago ante Mercado Pago y, únicamente si el estado es `approved`, marca la transferencia como pagada de manera idempotente.
6. La web consulta o recibe el estado actualizado y recalcula el resumen. Los intentos rechazados, pendientes o cancelados permanecen pendientes.

## Decisiones de arquitectura necesarias

### Persistencia y acceso

V2 necesita persistencia de servidor para las divisiones, transferencias, órdenes de pago y eventos recibidos. `localStorage` no es una fuente confiable para reconciliar un webhook y no puede compartirse entre dispositivos.

La solución debe crear identificadores de división y de transferencia no predecibles. Si se conserva el enfoque sin registro, el acceso puede resolverse con enlaces de capacidad firmados y de alcance limitado; si se quiere que las personas administren sus cobros, se deberá diseñar autenticación y autorización explícitas. Esta decisión se toma antes de implementar el checkout.

### Modelo de cobro

Antes de construir V2 se debe decidir quién recibe el dinero:

- **Cobro de un único recaudador:** el checkout se crea bajo la cuenta Mercado Pago de esa persona. Sirve cuando un integrante cobra todas las deudas del grupo.
- **Cobro directo al acreedor de cada transferencia:** cada acreedor debe vincular o autorizar su cuenta Mercado Pago mediante un mecanismo compatible con el producto de Mercado Pago elegido. No se debe cobrar en la cuenta de SplitBot para luego redistribuir fondos sin una evaluación legal, comercial y técnica específica.

La elección define el flujo de alta de usuarios y cuentas, el producto de Mercado Pago y las responsabilidades de conciliación. La V2 no debe asumir que una preferencia creada con una única cuenta puede liquidar automáticamente pagos a distintos integrantes.

### Backend de pagos

- Exponer un endpoint autenticado para crear una orden solo a partir de una transferencia pendiente y su monto exacto almacenado en el servidor. Nunca aceptar monto o acreedor arbitrarios desde el cliente.
- Guardar la relación entre división, transferencia, orden interna, preferencia/orden de Mercado Pago y estado de conciliación.
- Configurar URLs de retorno para éxito, pendiente y fallo; la UI debe comunicar cada estado sin cambiar el pago local por sí misma.
- Configurar un endpoint HTTPS de webhook. Verificar la firma `x-signature`, responder rápido y procesar el evento de forma idempotente.
- Obtener el detalle del pago desde la API de Mercado Pago antes de cambiar el estado. Registrar de forma auditable el identificador, estado y fecha, sin almacenar datos de tarjeta ni credenciales en la base de datos.
- Guardar las credenciales privadas solamente en un gestor de secretos y aplicar rate limiting, validación de origen y trazabilidad al nuevo endpoint.

## Contrato mínimo de datos

Una transferencia pendiente deberá tener, como mínimo:

```ts
type Settlement = {
  id: string;
  splitId: string;
  debtorParticipantId: string;
  creditorParticipantId: string;
  amount: number;
  currency: "ARS";
  status: "pending" | "processing" | "paid" | "failed" | "cancelled";
  paymentProvider?: "mercado_pago";
  providerPaymentId?: string;
  paidAt?: string;
};
```

El backend será la fuente de verdad de `status` para los pagos Mercado Pago. La interfaz puede optimizar la experiencia mostrando `processing`, pero no puede reemplazar la confirmación del webhook.

## Plan de entrega V2

1. Definir el modelo de cobro y validar los requisitos comerciales y regulatorios con Mercado Pago antes de elegir el producto de checkout.
2. Extraer la lógica de divisiones a tipos y cálculos reutilizables, y migrar las divisiones web a una API con persistencia.
3. Implementar el modelo de transferencias y el endpoint de creación de órdenes con pruebas de validación de importe, estado y autorización.
4. Integrar el checkout elegido desde la web, con estados de retorno claros y sin exponer credenciales privadas.
5. Implementar webhook, validación de firma, consulta de pago, idempotencia y auditoría.
6. Probar pagos aprobados, pendientes, rechazados, eventos duplicados, retorno manipulado y reinicio del navegador antes de habilitar el botón en producción.

## Referencias técnicas

- [Mercado Pago Checkout Bricks](https://www.mercadopago.com.ar/developers/es/docs/checkout-bricks/overview): opciones de checkout disponibles y sus medios de pago.
- [Preferencias de Checkout](https://www.mercadopago.com.ar/developers/es/reference/online-payments/checkout-pro-preferences/overview): creación de preferencias, URLs de retorno y consulta de pagos.
- [Notificaciones de Mercado Pago](https://www.mercadopago.com.ar/developers/es/docs/links-and-debts/additional-content/your-integrations/notifications): webhooks recomendados, tópico de pagos y validación de origen.
