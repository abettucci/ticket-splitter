package bot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// ============================================
// REMINDER HANDLERS
// ============================================

// handleCreateReminder crea un recordatorio de pago
func (h *Handler) handleCreateReminder(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 4 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /recordar_pago [descripción] [monto] [deudor] [acreedor] [cuotas] [frecuencia] [fecha]

<b>Parámetros:</b>
• <b>descripción</b> - Puede tener espacios (ej: Cuota ACDC)
• <b>monto</b> - Cantidad a pagar (el primer número)
• <b>deudor</b> - Quién debe pagar (usar "yo" si sos vos)
• <b>acreedor</b> - A quién se le paga (usar "yo" si te pagan a vos)
• <b>cuotas</b> - Número de cuotas (opcional, default: 1)
• <b>frecuencia</b> - monthly, weekly, daily, once (opcional)
• <b>fecha</b> - Cuándo empieza (opcional)

<b>Frecuencias:</b>
• monthly - Mensual | weekly - Semanal
• daily - Diario | once - Una sola vez

<b>Fechas especiales:</b>
• primero_del_mes - Día 1 de cada mes
• mitad_de_mes - Día 15
• fin_de_mes - Último día del mes
• primera_semana - Entre el 1 y 7
• ultima_semana - Últimos 7 días del mes
• DD/MM/YYYY - Fecha específica
• mañana, hoy, lunes, martes...

<b>Ejemplos:</b>
• /recordar_pago Netflix 1500 yo María 1 monthly primero_del_mes
  <i>(Yo le pago a María el 1ro de cada mes)</i>

• /recordar_pago Cuota ACDC 56542 Enzo yo 3 monthly 01/03/2026
  <i>(Enzo me paga a mí en 3 cuotas)</i>

• /recordar_pago Préstamo auto 10000 Juan yo 6 monthly
  <i>(Juan me debe en 6 cuotas mensuales)</i>

• /recordar_pago Alquiler depto 150000 yo Inmobiliaria 12 monthly primero_del_mes
  <i>(Yo pago alquiler el 1ro de cada mes)</i>`)
	}

	// Parsear argumentos: buscar el monto (primer número) para separar descripción del resto
	// Formato: [descripción con espacios] [monto] [deudor] [acreedor] [cuotas] [frecuencia] [fecha]

	var description string
	var amountStr string
	var remainingArgs []string
	amountIndex := -1

	// Buscar el primer argumento que sea un número (el monto)
	for i, arg := range args {
		// Limpiar comas y puntos para verificar si es número
		testAmount := strings.ReplaceAll(arg, ",", ".")
		if _, err := strconv.ParseFloat(testAmount, 64); err == nil {
			amountIndex = i
			amountStr = testAmount
			break
		}
	}

	if amountIndex == -1 || amountIndex == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No se encontró el monto. Asegurate de incluir un número.\n\nEjemplo: /recordar_pago Cuota ACDC 56542 Enzo yo 3 monthly")
	}

	// Todo antes del monto es la descripción
	description = strings.Join(args[:amountIndex], " ")

	// Todo después del monto son los demás argumentos
	remainingArgs = args[amountIndex+1:]

	if len(remainingArgs) < 2 {
		return h.tg.SendMessage(ctx, chatID, "❌ Faltan argumentos. Necesitás al menos: descripción, monto, deudor y acreedor.\n\nEjemplo: /recordar_pago Cuota ACDC 56542 Enzo yo 3 monthly")
	}

	// Extraer argumentos restantes
	debtorName := remainingArgs[0]   // Quién debe
	creditorName := remainingArgs[1] // A quién le deben

	var installmentsStr string = "1"
	var frequency string = "monthly"
	var startDateStr string = ""

	if len(remainingArgs) >= 3 {
		installmentsStr = remainingArgs[2]
	}
	if len(remainingArgs) >= 4 {
		frequency = strings.ToLower(remainingArgs[3])
	}
	if len(remainingArgs) >= 5 {
		startDateStr = remainingArgs[4]
	}

	// Determinar tipo de recordatorio y nombres
	var reminderType string
	var payeeName string // La otra persona involucrada

	debtorLower := strings.ToLower(debtorName)
	creditorLower := strings.ToLower(creditorName)

	if debtorLower == "yo" || debtorLower == "mi" || debtorLower == "me" {
		// Yo debo pagar a alguien
		reminderType = "payment"
		payeeName = creditorName
	} else if creditorLower == "yo" || creditorLower == "mi" || creditorLower == "me" {
		// Alguien me debe pagar a mí
		reminderType = "collection"
		payeeName = debtorName
	} else {
		// Si ninguno es "yo", asumimos que el usuario es el deudor
		reminderType = "payment"
		payeeName = creditorName
	}

	// Validar frecuencia
	validFrequencies := map[string]bool{
		"monthly": true,
		"weekly":  true,
		"daily":   true,
		"once":    true,
	}
	if !validFrequencies[frequency] {
		return h.tg.SendMessage(ctx, chatID, "❌ Frecuencia inválida. Usa: monthly, weekly, daily, o once")
	}

	// Parsear monto
	amountStr = strings.ReplaceAll(amountStr, ",", ".")
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ El monto debe ser un número válido.")
	}

	// Parsear cuotas
	installments, err := strconv.Atoi(installmentsStr)
	if err != nil || installments < 1 {
		return h.tg.SendMessage(ctx, chatID, "❌ El número de cuotas debe ser un número mayor a 0.")
	}

	// Parsear fecha de inicio
	var startDate time.Time
	if startDateStr != "" {
		startDate, err = parseFlexibleDate(startDateStr)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Fecha inválida: %v\n\nUsa formatos como: 15/01/2026, mañana, hoy, proximo_lunes", err))
		}
	} else {
		// Por defecto, mañana a las 9 AM
		startDate = time.Now().AddDate(0, 0, 1)
	}

	// Asegurarse de que la hora sea las 9 AM
	startDate = time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 9, 0, 0, 0, startDate.Location())

	// Crear recordatorio
	reminder, err := h.db.CreateReminder(ctx, userID, description, amount, payeeName, "", installments, frequency, startDate, reminderType)
	if err != nil {
		h.logger.Printf("Error creating reminder: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al crear el recordatorio.")
	}

	// Mensaje de confirmación
	frequencyText := map[string]string{
		"monthly": "mensual",
		"weekly":  "semanal",
		"daily":   "diario",
		"once":    "única",
	}

	var installmentText string
	if installments == 1 && frequency == "once" {
		installmentText = "Pago único"
	} else if installments == 1 {
		installmentText = fmt.Sprintf("Pago %s recurrente", frequencyText[frequency])
	} else {
		installmentText = fmt.Sprintf("%d cuotas %ss", installments, frequencyText[frequency])
	}

	shortID := reminder.ID[:8]

	// Mensaje diferente según el tipo de recordatorio
	var personLabel string
	var notificationText string
	var typeEmoji string

	if reminderType == "collection" {
		personLabel = "Deudor"
		notificationText = fmt.Sprintf("Te recordaré cobrarle a %s", telegram.EscapeHTML(payeeName))
		typeEmoji = "📥"
	} else {
		personLabel = "Pagar a"
		notificationText = "Te recordaré el pago en cada fecha."
		typeEmoji = "📤"
	}

	message := fmt.Sprintf(`✅ <b>Recordatorio creado</b>

%s <b>%s</b>
💰 Monto: %s
👤 %s: %s
📅 %s
⏰ Primer recordatorio: %s
🆔 ID: <code>%s</code>

<i>%s</i>

Usa /mis_recordatorios para ver todos tus recordatorios.`,
		typeEmoji,
		telegram.EscapeHTML(description),
		telegram.FormatMoney(amount),
		personLabel,
		telegram.EscapeHTML(payeeName),
		installmentText,
		startDate.Format("02/01/2006 15:04"),
		shortID,
		notificationText)

	return h.tg.SendMessage(ctx, chatID, message)
}

// handleMyReminders muestra los recordatorios activos del usuario
func (h *Handler) handleMyReminders(ctx context.Context, chatID, userID int64) error {
	reminders, err := h.db.GetUserReminders(ctx, userID)
	if err != nil {
		h.logger.Printf("Error getting reminders: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los recordatorios.")
	}

	if len(reminders) == 0 {
		return h.tg.SendMessage(ctx, chatID, "📋 No tienes recordatorios activos.\n\nUsa /recordar_pago para crear uno.")
	}

	// Detectar duplicados por descripción
	descCount := make(map[string]int)
	for _, r := range reminders {
		descCount[r.Description]++
	}

	var sb strings.Builder
	sb.WriteString("🔔 <b>Tus recordatorios activos</b>\n\n")

	for i, reminder := range reminders {
		shortID := reminder.ID[:8]
		progress := fmt.Sprintf("%d/%d", reminder.CurrentInstallment, reminder.TotalInstallments)

		// Emoji y etiqueta según tipo
		var typeEmoji, personLabel string
		if reminder.ReminderType == "collection" {
			typeEmoji = "📥"
			personLabel = "Deudor"
		} else {
			typeEmoji = "📤"
			personLabel = "Pagar a"
		}

		// Marcar si hay duplicados
		duplicateWarning := ""
		if descCount[reminder.Description] > 1 {
			duplicateWarning = " ⚠️"
		}

		sb.WriteString(fmt.Sprintf("%d. %s <b>%s</b>%s\n", i+1, typeEmoji, telegram.EscapeHTML(reminder.Description), duplicateWarning))
		sb.WriteString(fmt.Sprintf("   💰 %s | 👤 %s: %s\n", telegram.FormatMoney(reminder.Amount), personLabel, telegram.EscapeHTML(reminder.PayeeName)))
		sb.WriteString(fmt.Sprintf("   📊 Progreso: %s | 📅 Próximo: %s\n", progress, reminder.NextReminderDate.Format("02/01/2006")))
		sb.WriteString(fmt.Sprintf("   🆔 <code>%s</code>\n\n", shortID))
	}

	// Mostrar advertencia si hay duplicados
	hasDuplicates := false
	for _, count := range descCount {
		if count > 1 {
			hasDuplicates = true
			break
		}
	}
	if hasDuplicates {
		sb.WriteString("⚠️ <i>Tienes recordatorios duplicados. Usa /cancelar_recordatorio [id] para eliminar los que no necesites.</i>\n\n")
	}

	sb.WriteString("<i>Usa /cancelar_recordatorio [id] para cancelar uno.</i>")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleCancelReminder cancela un recordatorio
func (h *Handler) handleCancelReminder(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, "❌ Uso: /cancelar_recordatorio [id]\n\nEjemplo: /cancelar_recordatorio a1b2c3d4")
	}

	shortID := args[0]

	// Buscar el recordatorio
	reminders, err := h.db.GetUserReminders(ctx, userID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al buscar el recordatorio.")
	}

	var foundReminder *db.Reminder
	for _, reminder := range reminders {
		if len(reminder.ID) >= len(shortID) && reminder.ID[:len(shortID)] == shortID {
			foundReminder = &reminder
			break
		}
	}

	if foundReminder == nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Recordatorio no encontrado.")
	}

	// Desactivar recordatorio
	err = h.db.DeactivateReminder(ctx, userID, foundReminder.ID)
	if err != nil {
		h.logger.Printf("Error deactivating reminder: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al cancelar el recordatorio.")
	}

	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ Recordatorio cancelado: <b>%s</b>", telegram.EscapeHTML(foundReminder.Description)))
}

// handleRemindDebtors envía recordatorios a los deudores del grupo
func (h *Handler) handleRemindDebtors(ctx context.Context, chatID, userID int64) error {
	// Obtener miembros del grupo
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los miembros del grupo.")
	}

	var sb strings.Builder
	sb.WriteString("📢 <b>Recordatorio de deudas enviado</b>\n\n")

	debtorsCount := 0
	for _, member := range members {
		splits, err := h.db.GetUserPendingSplits(ctx, member.UserID)
		if err != nil || len(splits) == 0 {
			continue
		}

		var totalDebt float64
		for _, split := range splits {
			totalDebt += split.Amount
		}

		if totalDebt > 0 {
			debtorsCount++

			// Enviar mensaje privado al deudor
			debtMessage := fmt.Sprintf(`🔔 <b>Recordatorio de Deudas</b>

Hola %s, tienes deudas pendientes en el grupo.

💰 <b>Total adeudado: %s</b>

Usa /mis_deudas para ver el detalle.
Usa /pagar [id] para marcar como pagado.`,
				telegram.EscapeHTML(member.DisplayName),
				telegram.FormatMoney(totalDebt))

			// Intentar enviar mensaje privado
			err = h.tg.SendMessage(ctx, member.UserID, debtMessage)
			if err != nil {
				sb.WriteString(fmt.Sprintf("⚠️ %s - No se pudo enviar (debe iniciar chat con el bot)\n", member.DisplayName))
			} else {
				sb.WriteString(fmt.Sprintf("✅ %s - %s\n", member.DisplayName, telegram.FormatMoney(totalDebt)))
			}
		}
	}

	if debtorsCount == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No hay deudas pendientes en el grupo!")
	}

	sb.WriteString(fmt.Sprintf("\n<i>Total de deudores notificados: %d</i>", debtorsCount))

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handlePaymentCalendar muestra el calendario de pagos futuros
func (h *Handler) handlePaymentCalendar(ctx context.Context, chatID, userID int64) error {
	// Obtener recordatorios del usuario
	reminders, err := h.db.GetUserReminders(ctx, userID)
	if err != nil {
		h.logger.Printf("Error getting reminders: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener el calendario.")
	}

	// Obtener deudas pendientes del grupo
	splits, err := h.db.GetUserPendingSplits(ctx, userID)
	if err != nil {
		h.logger.Printf("Error getting splits: %v", err)
	}

	if len(reminders) == 0 && len(splits) == 0 {
		return h.tg.SendMessage(ctx, chatID, "📅 No tienes pagos programados ni deudas pendientes.")
	}

	var sb strings.Builder
	sb.WriteString("📅 <b>Tu Calendario de Pagos</b>\n\n")

	// Sección de pagos próximos (recordatorios)
	if len(reminders) > 0 {
		sb.WriteString("🔔 <b>Pagos Programados</b>\n\n")

		// Generar próximos 5 pagos por recordatorio
		type upcomingPayment struct {
			date         time.Time
			description  string
			amount       float64
			payee        string
			installment  int
			reminderType string
		}

		var payments []upcomingPayment
		now := time.Now()

		// Usar un mapa para evitar duplicados por descripción+payee
		seenReminders := make(map[string]bool)

		for _, reminder := range reminders {
			// Crear clave única para detectar duplicados
			reminderKey := fmt.Sprintf("%s|%s|%.2f", reminder.Description, reminder.PayeeName, reminder.Amount)
			if seenReminders[reminderKey] {
				// Duplicado, saltar
				continue
			}
			seenReminders[reminderKey] = true

			// Calcular cuotas restantes
			remainingInstallments := reminder.TotalInstallments - reminder.CurrentInstallment + 1
			if remainingInstallments <= 0 {
				// Recordatorio completado, no mostrar
				continue
			}

			// Generar próximos pagos (máximo las cuotas restantes, hasta 5)
			maxPaymentsForReminder := remainingInstallments
			if maxPaymentsForReminder > 5 {
				maxPaymentsForReminder = 5
			}

			nextDate := reminder.NextReminderDate
			for i := 0; i < maxPaymentsForReminder && nextDate.Before(now.AddDate(0, 6, 0)); i++ {
				if reminder.EndDate.IsZero() || nextDate.Before(reminder.EndDate) {
					payments = append(payments, upcomingPayment{
						date:         nextDate,
						description:  reminder.Description,
						amount:       reminder.Amount,
						payee:        reminder.PayeeName,
						installment:  reminder.CurrentInstallment + i,
						reminderType: reminder.ReminderType,
					})

					// Calcular siguiente fecha según frecuencia
					switch reminder.Frequency {
					case "daily":
						nextDate = nextDate.AddDate(0, 0, 1)
					case "weekly":
						nextDate = nextDate.AddDate(0, 0, 7)
					case "monthly":
						nextDate = nextDate.AddDate(0, 1, 0)
					default:
						break
					}
				}
			}
		}

		// Ordenar por fecha
		for i := 0; i < len(payments); i++ {
			for j := i + 1; j < len(payments); j++ {
				if payments[i].date.After(payments[j].date) {
					payments[i], payments[j] = payments[j], payments[i]
				}
			}
		}

		// Mostrar próximos pagos (máximo 10)
		maxPayments := 10
		if len(payments) > maxPayments {
			payments = payments[:maxPayments]
		}

		for i, payment := range payments {
			daysUntil := int(payment.date.Sub(now).Hours() / 24)
			dateStr := payment.date.Format("02/01/2006")

			var urgency string
			if daysUntil <= 0 {
				urgency = "🔴"
			} else if daysUntil <= 3 {
				urgency = "🟡"
			} else {
				urgency = "🟢"
			}

			// Emoji y etiqueta según tipo
			var typeEmoji, personLabel string
			if payment.reminderType == "collection" {
				typeEmoji = "📥"
				personLabel = "Deudor"
			} else {
				typeEmoji = "📤"
				personLabel = "Pagar a"
			}

			sb.WriteString(fmt.Sprintf("%s %d. %s <b>%s</b>\n", urgency, i+1, typeEmoji, telegram.EscapeHTML(payment.description)))
			sb.WriteString(fmt.Sprintf("   💰 %s | 👤 %s: %s\n", telegram.FormatMoney(payment.amount), personLabel, telegram.EscapeHTML(payment.payee)))
			sb.WriteString(fmt.Sprintf("   📅 %s", dateStr))

			if daysUntil > 0 {
				sb.WriteString(fmt.Sprintf(" (en %d días)", daysUntil))
			} else if daysUntil == 0 {
				sb.WriteString(" (hoy)")
			} else {
				sb.WriteString(" (vencido)")
			}
			sb.WriteString("\n\n")
		}
	}

	// Sección de deudas pendientes
	if len(splits) > 0 {
		sb.WriteString("💳 <b>Deudas Pendientes</b>\n")
		sb.WriteString("<i>(sin fecha programada)</i>\n\n")

		// Agrupar por gasto
		expenseDebts := make(map[string]float64)
		expenseInfo := make(map[string]*db.ExpenseSplit)

		for _, split := range splits {
			expenseDebts[split.ExpenseID] += split.Amount
			if _, exists := expenseInfo[split.ExpenseID]; !exists {
				expenseInfo[split.ExpenseID] = &split
			}
		}

		count := 0
		for expenseID, totalAmount := range expenseDebts {
			if count >= 5 {
				sb.WriteString(fmt.Sprintf("\n<i>... y %d deudas más</i>\n", len(expenseDebts)-5))
				break
			}

			split := expenseInfo[expenseID]
			expense, err := h.db.GetExpenseByShortID(ctx, chatID, split.ExpenseID)
			if err == nil {
				sb.WriteString(fmt.Sprintf("• <b>%s</b>\n", telegram.EscapeHTML(expense.Description)))
				sb.WriteString(fmt.Sprintf("  💰 %s\n\n", telegram.FormatMoney(totalAmount)))
			}
			count++
		}
	}

	sb.WriteString("\n💡 <b>Leyenda:</b>\n")
	sb.WriteString("🔴 Vencido o para hoy\n")
	sb.WriteString("🟡 Próximos 3 días\n")
	sb.WriteString("🟢 Más de 3 días\n\n")
	sb.WriteString("<i>Usa /mis_recordatorios para gestionar tus recordatorios\nUsa /mis_deudas para ver detalles de deudas</i>")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// ============================================
// HELPER FUNCTIONS FOR DATE PARSING
// ============================================

// parseFlexibleDate parsea fechas en múltiples formatos
func parseFlexibleDate(dateStr string) (time.Time, error) {
	dateStr = strings.ToLower(strings.TrimSpace(dateStr))
	now := time.Now()

	// Casos especiales - días del mes
	switch dateStr {
	case "primero_del_mes", "primer_dia", "dia_1", "1_del_mes":
		return findNextDayOfMonth(now, 1), nil
	case "mitad_de_mes", "mitad_del_mes", "dia_15", "15_del_mes":
		return findNextDayOfMonth(now, 15), nil
	case "fin_de_mes", "fin_del_mes", "ultimo_dia", "ultimo_del_mes":
		return findLastDayOfMonth(now), nil
	case "primera_semana", "primera_semana_del_mes":
		return findNextDayOfMonth(now, 5), nil // Día 5 como referencia de primera semana
	case "ultima_semana", "ultima_semana_del_mes":
		return findLastWeekOfMonth(now), nil
	}

	// Casos especiales - días de la semana
	switch dateStr {
	case "hoy", "today":
		return now, nil
	case "mañana", "manana", "tomorrow":
		return now.AddDate(0, 0, 1), nil
	case "lunes", "proximo_lunes", "próximo_lunes":
		return findNextWeekday(now, time.Monday), nil
	case "martes", "proximo_martes", "próximo_martes":
		return findNextWeekday(now, time.Tuesday), nil
	case "miercoles", "miércoles", "proximo_miercoles", "próximo_miércoles":
		return findNextWeekday(now, time.Wednesday), nil
	case "jueves", "proximo_jueves", "próximo_jueves":
		return findNextWeekday(now, time.Thursday), nil
	case "viernes", "proximo_viernes", "próximo_viernes":
		return findNextWeekday(now, time.Friday), nil
	case "sabado", "sábado", "proximo_sabado", "próximo_sábado":
		return findNextWeekday(now, time.Saturday), nil
	case "domingo", "proximo_domingo", "próximo_domingo":
		return findNextWeekday(now, time.Sunday), nil
	}

	// Intentar parsear formato DD/MM/YYYY
	if matched, _ := regexp.MatchString(`^\d{1,2}/\d{1,2}/\d{4}$`, dateStr); matched {
		t, err := time.Parse("02/01/2006", dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("formato de fecha inválido, usa DD/MM/YYYY")
		}
		return t, nil
	}

	// Intentar parsear formato YYYY-MM-DD
	if matched, _ := regexp.MatchString(`^\d{4}-\d{1,2}-\d{1,2}$`, dateStr); matched {
		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			return time.Time{}, fmt.Errorf("formato de fecha inválido, usa YYYY-MM-DD")
		}
		return t, nil
	}

	return time.Time{}, fmt.Errorf("formato de fecha no reconocido: %s", dateStr)
}

// findNextWeekday encuentra el próximo día de la semana especificado
func findNextWeekday(from time.Time, targetWeekday time.Weekday) time.Time {
	// Si es el mismo día de la semana pero es hoy, buscar el siguiente
	daysUntil := int(targetWeekday - from.Weekday())
	if daysUntil <= 0 {
		daysUntil += 7
	}
	return from.AddDate(0, 0, daysUntil)
}

// findNextDayOfMonth encuentra el próximo día específico del mes
func findNextDayOfMonth(from time.Time, targetDay int) time.Time {
	year, month, day := from.Date()

	// Si ya pasó ese día este mes, ir al siguiente mes
	if day >= targetDay {
		month++
		if month > 12 {
			month = 1
			year++
		}
	}

	// Asegurarse de que el día existe en ese mes
	lastDay := lastDayOfMonth(year, month)
	if targetDay > lastDay {
		targetDay = lastDay
	}

	return time.Date(year, month, targetDay, 9, 0, 0, 0, from.Location())
}

// findLastDayOfMonth encuentra el último día del mes actual o siguiente
func findLastDayOfMonth(from time.Time) time.Time {
	year, month, day := from.Date()

	lastDay := lastDayOfMonth(year, month)

	// Si ya estamos en el último día o pasamos, ir al siguiente mes
	if day >= lastDay {
		month++
		if month > 12 {
			month = 1
			year++
		}
		lastDay = lastDayOfMonth(year, month)
	}

	return time.Date(year, month, lastDay, 9, 0, 0, 0, from.Location())
}

// findLastWeekOfMonth encuentra un día en la última semana del mes
func findLastWeekOfMonth(from time.Time) time.Time {
	year, month, day := from.Date()

	lastDay := lastDayOfMonth(year, month)
	targetDay := lastDay - 3 // Mitad de la última semana

	// Si ya pasamos esa fecha, ir al siguiente mes
	if day >= targetDay {
		month++
		if month > 12 {
			month = 1
			year++
		}
		lastDay = lastDayOfMonth(year, month)
		targetDay = lastDay - 3
	}

	return time.Date(year, month, targetDay, 9, 0, 0, 0, from.Location())
}

// lastDayOfMonth retorna el último día de un mes específico
func lastDayOfMonth(year int, month time.Month) int {
	// Ir al primer día del mes siguiente y restar un día
	nextMonth := month + 1
	nextYear := year
	if nextMonth > 12 {
		nextMonth = 1
		nextYear++
	}
	return time.Date(nextYear, nextMonth, 1, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -1).Day()
}
