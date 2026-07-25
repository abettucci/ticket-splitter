# 📅 Sistema de Calendario de Pagos - Implementación

## ✅ Implementado (Fase 1)

### 1. **Nuevo Comando: `/calendario_pagos`**

Muestra un calendario visual de todos tus pagos futuros:

```
📅 Tu Calendario de Pagos

🔔 Pagos Programados

🔴 1. Netflix
   💰 $1500.00 | 👤 María
   📅 28/12/2024 (hoy)

🟡 2. Recital
   💰 $5000.00 | 👤 Juan
   📅 30/12/2024 (en 2 días)

🟢 3. Alquiler
   💰 $50000.00 | 👤 Pedro
   📅 05/01/2025 (en 8 días)

💳 Deudas Pendientes
(sin fecha programada)

• Cena
  💰 $2500.00

💡 Leyenda:
🔴 Vencido o para hoy
🟡 Próximos 3 días
🟢 Más de 3 días
```

**Características:**
- ✅ Muestra próximos pagos programados (recordatorios)
- ✅ Muestra deudas pendientes del grupo
- ✅ Código de colores por urgencia
- ✅ Ordenado por fecha
- ✅ Muestra hasta 10 pagos próximos

### 2. **Estructura de Datos Mejorada**

Se agregaron campos a la tabla `Reminder`:

```go
type Reminder struct {
    // ... campos existentes ...
    
    // NUEVOS CAMPOS
    ChatID             int64     // Para futuros pagos grupales
    PayeeUserID        int64     // Para tracking del beneficiario
    ReminderType       string    // "payment" (debo pagar) o "collection" (debo cobrar)
    
    // Recurrencia avanzada (estilo Google Calendar)
    RecurrenceRule     string    // Regla de recurrencia
    RecurrenceInterval int       // Ej: cada 2 semanas
    RecurrenceDayOfWeek int      // 0=domingo, 1=lunes, etc
    RecurrenceDayOfMonth int     // Día del mes (1-31)
    RecurrenceWeekOfMonth int    // 1=primer, 2=segundo, -1=último
}
```

## 🚧 Pendiente (Fase 2) - Para implementar

### 1. **Mejorar `/recordar_pago` con recurrencias avanzadas**

**Sintaxis propuesta:**

```bash
# Básico (ya funciona)
/recordar_pago Netflix 1500 María 1 monthly

# CON FECHAS (nuevo)
/recordar_pago Netflix 1500 María 01/01/2025 31/12/2025 monthly

# RECURRENCIAS AVANZADAS (nuevo)
/recordar_pago Gimnasio 5000 Pedro 01/01/2025 31/12/2025 every_monday
/recordar_pago Alquiler 50000 Juan 01/01/2025 31/12/2025 first_monday
/recordar_pago Tarjeta 10000 María 01/01/2025 31/12/2025 every_15_days
/recordar_pago Cuota 3000 Pedro 01/01/2025 31/12/2025 day_15
```

**Frecuencias a implementar:**
- `every_monday`, `every_tuesday`, etc. - Cada lunes/martes/etc
- `first_monday`, `second_monday`, etc. - Primer/segundo lunes del mes
- `last_monday`, `last_friday`, etc. - Último lunes/viernes del mes
- `every_X_days` - Cada X días (ej: `every_15_days`)
- `day_X` - Día X de cada mes (ej: `day_15` = día 15 de cada mes)
- `biweekly` - Cada 2 semanas
- `quarterly` - Trimestral
- `semiannual` - Semestral
- `annual` - Anual

### 2. **Función para calcular próxima fecha según regla**

```go
// Función a implementar en db/client.go
func CalculateNextReminderDate(reminder *Reminder) time.Time {
    now := time.Now()
    
    switch {
    case reminder.RecurrenceRule == "every_monday":
        // Encontrar próximo lunes
        return findNextWeekday(now, time.Monday)
        
    case reminder.RecurrenceRule == "first_monday":
        // Primer lunes del próximo mes
        return findNthWeekdayOfMonth(now, time.Monday, 1)
        
    case reminder.RecurrenceRule == "last_friday":
        // Último viernes del mes
        return findNthWeekdayOfMonth(now, time.Friday, -1)
        
    case strings.HasPrefix(reminder.RecurrenceRule, "every_"):
        // every_15_days, every_30_days, etc
        days := parseIntervalDays(reminder.RecurrenceRule)
        return now.AddDate(0, 0, days)
        
    case strings.HasPrefix(reminder.RecurrenceRule, "day_"):
        // day_15 = día 15 de cada mes
        day := parseDayOfMonth(reminder.RecurrenceRule)
        return findNextDayOfMonth(now, day)
    }
    
    // Fallback a frecuencias simples
    switch reminder.Frequency {
    case "daily":
        return now.AddDate(0, 0, 1)
    case "weekly":
        return now.AddDate(0, 0, 7)
    case "monthly":
        return now.AddDate(0, 1, 0)
    default:
        return now
    }
}
```

### 3. **Parser mejorado para `/recordar_pago`**

```go
func (h *Handler) handleCreateReminderAdvanced(ctx context.Context, chatID, userID int64, args []string) error {
    // Formato: /recordar_pago [desc] [monto] [persona] [fecha_inicio] [fecha_fin] [recurrencia]
    
    if len(args) < 6 {
        return h.showReminderHelp(ctx, chatID)
    }
    
    description := args[0]
    amountStr := args[1]
    payeeName := args[2]
    startDateStr := args[3]  // "01/01/2025" o "tomorrow" o "next_monday"
    endDateStr := args[4]    // "31/12/2025" o "never"
    recurrence := args[5]    // "every_monday", "first_monday", "every_15_days", etc
    
    // Parse dates
    startDate := parseFlexibleDate(startDateStr)
    endDate := parseFlexibleDate(endDateStr)
    
    // Create reminder with advanced recurrence
    reminder := &db.Reminder{
        Description: description,
        Amount: amount,
        PayeeName: payeeName,
        StartDate: startDate,
        EndDate: endDate,
        RecurrenceRule: recurrence,
        // ... otros campos
    }
    
    // Calcular próxima fecha según regla
    reminder.NextReminderDate = CalculateNextReminderDate(reminder)
    
    // Guardar
    err := h.db.CreateReminderAdvanced(ctx, reminder)
    // ...
}
```

### 4. **Parsers de fechas flexibles**

```go
// Soportar formatos como:
// - "01/01/2025"
// - "tomorrow"
// - "next_monday"
// - "in_7_days"
// - "never"
func parseFlexibleDate(dateStr string) time.Time {
    switch strings.ToLower(dateStr) {
    case "tomorrow":
        return time.Now().AddDate(0, 0, 1)
    case "next_monday":
        return findNextWeekday(time.Now(), time.Monday)
    case "never":
        return time.Time{} // Zero value
    default:
        if strings.HasPrefix(dateStr, "in_") {
            // "in_7_days", "in_2_weeks", "in_3_months"
            return parseRelativeDate(dateStr)
        }
        // Parse DD/MM/YYYY
        return parseAbsoluteDate(dateStr)
    }
}
```

### 5. **Comando `/recordatorios_cobrar`**

Para recordatorios de pagos que **te deben a vos**:

```bash
/recordatorios_cobrar Préstamo 10000 Juan 01/01/2025 31/12/2025 monthly
```

Esto crea recordatorios de tipo `"collection"` en lugar de `"payment"`.

### 6. **Notificaciones inteligentes**

En el `reminder-worker`, mejorar las notificaciones:

```go
// Notificar con anticipación
if reminder.NextReminderDate.Sub(now) <= 24*time.Hour {
    message = "🔴 ¡Pago para mañana!"
} else if reminder.NextReminderDate.Sub(now) <= 3*24*time.Hour {
    message = "🟡 Pago en 3 días"
}

// Incluir próximos 3 pagos en la notificación
message += "\n\nPróximos pagos:\n"
for i := 0; i < 3; i++ {
    nextDate := CalculateNextReminderDate(reminder)
    message += fmt.Sprintf("• %s: $%.2f\n", nextDate.Format("02/01"), reminder.Amount)
}
```

## 📋 Tareas de Implementación

### Prioridad Alta (hacer primero)
- [ ] Implementar `CalculateNextReminderDate()` con todas las reglas
- [ ] Crear `parseFlexibleDate()` para fechas flexibles
- [ ] Actualizar `handleCreateReminder` para soportar fechas inicio/fin
- [ ] Implementar reglas básicas: `every_monday`, `first_monday`, `day_15`

### Prioridad Media
- [ ] Implementar `every_X_days` dinámico
- [ ] Agregar comando `/recordatorios_cobrar`
- [ ] Mejorar notificaciones con anticipación
- [ ] Agregar validación de fechas (fin > inicio)

### Prioridad Baja
- [ ] Soporte para múltiples zonas horarias
- [ ] Exportar calendario a Google Calendar
- [ ] Recordatorios de grupo (todos pagan)
- [ ] Estadísticas de pagos (cuánto pagás por mes)

## 🎯 Ejemplos de Uso Final

```bash
# Gimnasio cada lunes
/recordar_pago Gimnasio 5000 Pedro 01/01/2025 31/12/2025 every_monday

# Alquiler primer día del mes
/recordar_pago Alquiler 50000 Juan 01/01/2025 never day_1

# Tarjeta cada 15 días
/recordar_pago Tarjeta 10000 María tomorrow 31/12/2025 every_15_days

# Cuota primer viernes de cada mes
/recordar_pago Cuota 3000 Pedro next_friday 31/12/2025 first_friday

# Ver calendario
/calendario_pagos
```

## 📝 Notas de Implementación

1. **Backward compatibility**: Los recordatorios existentes con `frequency` simple seguirán funcionando
2. **Migración**: No se requiere migración de datos, los campos nuevos son opcionales
3. **Testing**: Crear casos de prueba para cada tipo de recurrencia
4. **Documentación**: Actualizar `/help` con ejemplos de cada recurrencia

---

**Estado actual**: Fase 1 completada ✅  
**Próximo paso**: Implementar Fase 2 con recurrencias avanzadas

