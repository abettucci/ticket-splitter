package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// handleDivideCustom maneja división con montos personalizados
func (h *Handler) handleDivideCustom(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 3 {
		return h.tg.SendMessage(ctx, chatID, `❌ *Formato incorrecto*

Uso: /dividir_custom [id] @user1 monto1 @user2 monto2...

Ejemplo:
/dividir_custom a1b2c3 @juan 5000 @maria 3000 @pedro 2000`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	if expense.IsDivided {
		return h.tg.SendMessage(ctx, chatID, "⚠️ Este gasto ya fue dividido")
	}

	// Parsear splits custom
	customSplits := make(map[int64]float64)
	members, _ := h.db.GetGroupMembers(ctx, chatID)
	memberMap := make(map[string]int64)

	for _, m := range members {
		if m.Username != "" {
			memberMap[strings.ToLower(m.Username)] = m.UserID
		}
	}

	var totalCustom float64
	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}

		username := strings.TrimPrefix(args[i], "@")
		username = strings.ToLower(username)
		amount, err := strconv.ParseFloat(args[i+1], 64)
		if err != nil {
			continue
		}

		if uid, ok := memberMap[username]; ok {
			customSplits[uid] = amount
			totalCustom += amount
		}
	}

	if len(customSplits) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No se encontraron splits válidos")
	}

	if totalCustom != expense.TotalAmount {
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Los montos no suman el total\n\nTotal: %s\nSuma: %s", telegram.FormatMoney(expense.TotalAmount), telegram.FormatMoney(totalCustom)))
	}

	payers := []db.Payer{{
		UserID:   expense.CreatedBy,
		UserName: expense.CreatorName,
		Amount:   expense.TotalAmount,
	}}

	err = h.db.CreateSplitsWithCustomAmounts(ctx, expense, customSplits, payers)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al dividir el gasto")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💰 <b>Gasto dividido (custom): %s</b>\n\n", expense.Description))
	sb.WriteString(fmt.Sprintf("📊 Total: %s\n\n<b>División:</b>\n", telegram.FormatMoney(expense.TotalAmount)))

	for uid, amount := range customSplits {
		for _, m := range members {
			if m.UserID == uid {
				sb.WriteString(fmt.Sprintf("• %s: %s\n", m.DisplayName, telegram.FormatMoney(amount)))
				break
			}
		}
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleDivideSelect maneja división selectiva (solo algunos miembros)
func (h *Handler) handleDivideSelect(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 2 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /dividir_entre [id] @user1 @user2 @user3

Ejemplo:
/dividir_entre a1b2c3 @juan @maria @pedro`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	if expense.IsDivided {
		return h.tg.SendMessage(ctx, chatID, "⚠️ Este gasto ya fue dividido")
	}

	members, _ := h.db.GetGroupMembers(ctx, chatID)
	memberMap := make(map[string]*db.Member)

	for i := range members {
		if members[i].Username != "" {
			memberMap[strings.ToLower(members[i].Username)] = &members[i]
		}
	}

	selectedMembers := []db.Member{}
	for i := 1; i < len(args); i++ {
		username := strings.TrimPrefix(args[i], "@")
		username = strings.ToLower(username)

		if member, ok := memberMap[username]; ok {
			selectedMembers = append(selectedMembers, *member)
		}
	}

	if len(selectedMembers) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No se encontraron miembros válidos")
	}

	err = h.db.CreateSplits(ctx, expense, selectedMembers)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al dividir el gasto")
	}

	amountPerPerson := expense.TotalAmount / float64(len(selectedMembers))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💰 <b>Gasto dividido (selectivo): %s</b>\n\n", expense.Description))
	sb.WriteString(fmt.Sprintf("📊 Total: %s\n", telegram.FormatMoney(expense.TotalAmount)))
	sb.WriteString(fmt.Sprintf("👥 Participantes: %d\n", len(selectedMembers)))
	sb.WriteString(fmt.Sprintf("💵 Por persona: <b>%s</b>\n\n", telegram.FormatMoney(amountPerPerson)))

	for _, m := range selectedMembers {
		sb.WriteString(fmt.Sprintf("• %s\n", m.DisplayName))
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleDivideItems maneja división por items
func (h *Handler) handleDivideItems(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, `❌ *Formato incorrecto*

Uso: /dividir_items [id]

Luego envía items en el formato:
Item: descripción, monto, @user1 @user2

Ejemplo:
Pizza: 5000, @juan @maria
Cerveza: 3000, @pedro @juan`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	if expense.IsDivided {
		return h.tg.SendMessage(ctx, chatID, "⚠️ Este gasto ya fue dividido")
	}

	return h.tg.SendMessage(ctx, chatID, `📝 <b>División por items</b>

Envía cada item en una línea con el formato:
[descripción]: [monto], @user1 @user2

Ejemplo:
Pizza: 5000, @juan @maria
Cerveza: 3000, @pedro

Cuando termines, envía: /confirmar_items`)
}

// handleEdit maneja edición de gastos
func (h *Handler) handleEdit(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 3 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /editar [id] [campo] [nuevo_valor]

<b>Campos editables:</b>
• descripcion
• monto
• categoria

<b>Ejemplo:</b>
/editar a1b2c3 descripcion Cena grupal
/editar a1b2c3 monto 18000
/editar a1b2c3 categoria comida

💡 <i>Si el gasto ya fue dividido, al cambiar el monto se recalculan las deudas automáticamente.</i>
💡 <i>Usa /redividir [id] para cambiar los participantes de un gasto ya dividido.</i>`)
	}

	shortID := args[0]
	field := strings.ToLower(args[1])
	value := strings.Join(args[2:], " ")

	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	var extraMessage string
	oldAmount := expense.TotalAmount

	switch field {
	case "descripcion", "description":
		expense.Description = value
	case "monto", "amount":
		amount, err := strconv.ParseFloat(strings.ReplaceAll(value, ",", "."), 64)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ Monto inválido")
		}
		expense.TotalAmount = amount

		// Si el gasto ya fue dividido, recalcular los splits
		if expense.IsDivided && oldAmount != amount {
			err = h.db.RecalculateSplitsForExpense(ctx, expense, amount)
			if err != nil {
				h.logger.Printf("Error recalculating splits: %v", err)
				return h.tg.SendMessage(ctx, chatID, "❌ Error al recalcular las deudas")
			}

			// Obtener los splits actualizados para mostrar
			splits, _ := h.db.GetExpenseSplits(ctx, expense.ID)
			if len(splits) > 0 {
				newAmountPerPerson := amount / float64(len(splits))
				extraMessage = fmt.Sprintf("\n\n🔄 <b>Deudas recalculadas</b>\n👥 %d participantes\n💵 Nueva deuda por persona: %s", len(splits), telegram.FormatMoney(newAmountPerPerson))
			}
		}
	case "categoria", "category":
		expense.Category = value
	default:
		return h.tg.SendMessage(ctx, chatID, "❌ Campo no válido. Campos permitidos: descripcion, monto, categoria")
	}

	err = h.db.UpdateExpense(ctx, expense)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al actualizar el gasto")
	}

	statusIcon := "⏳"
	if expense.IsDivided {
		statusIcon = "✅"
	}

	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ <b>Gasto actualizado</b>\n\n📝 %s\n💰 %s\n%s %s%s",
		telegram.EscapeHTML(expense.Description),
		telegram.FormatMoney(expense.TotalAmount),
		statusIcon,
		map[bool]string{true: "Dividido", false: "Pendiente de dividir"}[expense.IsDivided],
		extraMessage))
}

// handleRedivide permite redividir un gasto ya dividido con nuevos participantes
func (h *Handler) handleRedivide(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /redividir [id] [@user1 @user2...]

<b>Ejemplos:</b>
• /redividir a1b2c3
  <i>(Redivide entre todos los miembros del grupo)</i>

• /redividir a1b2c3 @ana @flor @pauli
  <i>(Redivide solo entre los usuarios seleccionados)</i>

💡 Esto elimina la división anterior y crea una nueva.`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	// Resetear la división existente (eliminar splits y marcar como no dividido)
	if expense.IsDivided {
		h.logger.Printf("Resetting division for expense ID: %s, PK: %s, SK: %s", expense.ID, expense.PK, expense.SK)
		err = h.db.ResetExpenseDivision(ctx, expense)
		if err != nil {
			h.logger.Printf("Error resetting expense division for %s: %v", expense.ID, err)
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Error al resetear la división anterior: %v", err))
		}
		expense.IsDivided = false
	}

	// Obtener miembros del grupo
	allMembers, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil || len(allMembers) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No hay miembros registrados en el grupo.")
	}

	var selectedMembers []db.Member

	// Si hay usuarios especificados, usar solo esos
	if len(args) > 1 {
		memberMap := make(map[string]*db.Member)
		for i := range allMembers {
			if allMembers[i].Username != "" {
				memberMap[strings.ToLower(allMembers[i].Username)] = &allMembers[i]
			}
		}

		for i := 1; i < len(args); i++ {
			username := strings.TrimPrefix(args[i], "@")
			username = strings.ToLower(username)

			if member, ok := memberMap[username]; ok {
				selectedMembers = append(selectedMembers, *member)
			}
		}

		if len(selectedMembers) == 0 {
			return h.tg.SendMessage(ctx, chatID, "❌ No se encontraron miembros válidos. Verifica los usernames.")
		}
	} else {
		// Sin usuarios especificados, usar todos los miembros
		selectedMembers = allMembers
	}

	// Crear nuevas divisiones
	err = h.db.CreateSplits(ctx, expense, selectedMembers)
	if err != nil {
		h.logger.Printf("Error creating splits: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al crear la nueva división")
	}

	amountPerPerson := expense.TotalAmount / float64(len(selectedMembers))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔄 <b>Gasto redividido: %s</b>\n\n", telegram.EscapeHTML(expense.Description)))
	sb.WriteString(fmt.Sprintf("📊 Total: %s\n", telegram.FormatMoney(expense.TotalAmount)))
	sb.WriteString(fmt.Sprintf("👥 Participantes: %d\n", len(selectedMembers)))
	sb.WriteString(fmt.Sprintf("💵 Por persona: <b>%s</b>\n\n", telegram.FormatMoney(amountPerPerson)))
	sb.WriteString("<b>Nueva división:</b>\n")

	for _, m := range selectedMembers {
		status := "⏳ Pendiente"
		if m.UserID == expense.CreatedBy {
			status = "✅ Pagó (creador)"
		}
		sb.WriteString(fmt.Sprintf("• %s: %s\n", telegram.EscapeHTML(m.DisplayName), status))
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleRedivideCustom permite redividir un gasto ya dividido con montos personalizados
func (h *Handler) handleRedivideCustom(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 3 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /redividir_custom [id] @user1 monto1 @user2 monto2...

<b>Ejemplo:</b>
/redividir_custom a1b2c3 @agusbettu 100000 @kuper 20000

💡 Esto elimina la división anterior y crea una nueva con los montos especificados.
⚠️ La suma de los montos debe ser igual al total del gasto.`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	// Parsear splits custom
	customSplits := make(map[int64]float64)
	members, _ := h.db.GetGroupMembers(ctx, chatID)
	memberMap := make(map[string]*db.Member)

	for i := range members {
		if members[i].Username != "" {
			memberMap[strings.ToLower(members[i].Username)] = &members[i]
		}
	}

	var totalCustom float64
	var splitDetails []string

	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			break
		}

		username := strings.TrimPrefix(args[i], "@")
		username = strings.ToLower(username)
		amount, err := strconv.ParseFloat(strings.ReplaceAll(args[i+1], ",", "."), 64)
		if err != nil {
			continue
		}

		if member, ok := memberMap[username]; ok {
			customSplits[member.UserID] = amount
			totalCustom += amount
			splitDetails = append(splitDetails, fmt.Sprintf("• %s: %s", member.DisplayName, telegram.FormatMoney(amount)))
		}
	}

	if len(customSplits) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No se encontraron splits válidos. Verifica los usernames y montos.")
	}

	if totalCustom != expense.TotalAmount {
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Los montos no suman el total del gasto\n\n💰 Total del gasto: %s\n📊 Suma de montos: %s\n📉 Diferencia: %s",
			telegram.FormatMoney(expense.TotalAmount), telegram.FormatMoney(totalCustom), telegram.FormatMoney(expense.TotalAmount-totalCustom)))
	}

	// Resetear la división existente si ya estaba dividido
	if expense.IsDivided {
		h.logger.Printf("Resetting division for expense ID: %s (redividir_custom)", expense.ID)
		err = h.db.ResetExpenseDivision(ctx, expense)
		if err != nil {
			h.logger.Printf("Error resetting expense division: %v", err)
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Error al resetear la división anterior: %v", err))
		}
	}

	// Crear los payers (quien pagó el gasto)
	payers := []db.Payer{{
		UserID:   expense.CreatedBy,
		UserName: expense.CreatorName,
		Amount:   expense.TotalAmount,
	}}

	// Crear nuevas divisiones con montos custom
	err = h.db.CreateSplitsWithCustomAmounts(ctx, expense, customSplits, payers)
	if err != nil {
		h.logger.Printf("Error creating custom splits: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al crear la nueva división")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔄 <b>Gasto redividido (custom): %s</b>\n\n", telegram.EscapeHTML(expense.Description)))
	sb.WriteString(fmt.Sprintf("📊 Total: %s\n", telegram.FormatMoney(expense.TotalAmount)))
	sb.WriteString(fmt.Sprintf("👤 Pagado por: %s\n", expense.CreatorName))
	sb.WriteString(fmt.Sprintf("👥 Participantes: %d\n\n", len(customSplits)))
	sb.WriteString("<b>Nueva división:</b>\n")

	for _, detail := range splitDetails {
		sb.WriteString(detail + "\n")
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleChangePayer permite cambiar quién pagó un gasto
func (h *Handler) handleChangePayer(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 2 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /cambiar_pagador [id] @nuevo_pagador

<b>Ejemplo:</b>
/cambiar_pagador a1b2c3 @kuper

💡 Esto cambia quién pagó el gasto. Si el gasto ya fue dividido, se recalculan las deudas automáticamente.`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	// Buscar el nuevo pagador
	newPayerUsername := strings.TrimPrefix(args[1], "@")
	newPayerUsername = strings.ToLower(newPayerUsername)

	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los miembros del grupo")
	}

	var newPayer *db.Member
	for i := range members {
		if members[i].Username != "" && strings.ToLower(members[i].Username) == newPayerUsername {
			newPayer = &members[i]
			break
		}
	}

	if newPayer == nil {
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Usuario @%s no encontrado en el grupo", newPayerUsername))
	}

	oldPayerName := expense.CreatorName
	oldPayerID := expense.CreatedBy

	// Actualizar el pagador en el expense
	expense.CreatedBy = newPayer.UserID
	expense.CreatorName = newPayer.DisplayName

	err = h.db.UpdateExpense(ctx, expense)
	if err != nil {
		h.logger.Printf("Error updating expense payer: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al actualizar el pagador")
	}

	// Si el gasto ya fue dividido, actualizar los splits
	var extraMessage string
	if expense.IsDivided {
		err = h.db.UpdateSplitsPayer(ctx, expense.ID, oldPayerID, newPayer.UserID)
		if err != nil {
			h.logger.Printf("Error updating splits payer: %v", err)
			extraMessage = "\n\n⚠️ <i>Nota: Hubo un problema al actualizar las deudas. Considera usar /redividir para recalcular.</i>"
		} else {
			extraMessage = "\n\n✅ <i>Las deudas han sido recalculadas automáticamente.</i>"
		}
	}

	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf(`✅ <b>Pagador actualizado</b>

📝 Gasto: %s
💰 Monto: %s

👤 Pagador anterior: %s
👤 Nuevo pagador: <b>%s</b>%s`,
		telegram.EscapeHTML(expense.Description),
		telegram.FormatMoney(expense.TotalAmount),
		telegram.EscapeHTML(oldPayerName),
		telegram.EscapeHTML(newPayer.DisplayName),
		extraMessage))
}

// handleDelete maneja eliminación de gastos (muestra confirmación con botones)
func (h *Handler) handleDelete(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /eliminar [id_gasto]

💡 El gasto irá a la papelera por 30 días.
Usa /papelera para ver gastos eliminados.
Usa /restaurar [id] para recuperar un gasto.`)
	}

	shortID := args[0]
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado")
	}

	// Determinar estado del gasto
	statusText := "Pendiente de dividir"
	statusIcon := "⏳"
	if expense.IsDivided {
		statusText = "Dividido"
		statusIcon = "✅"
	}

	// Mensaje de confirmación con detalles completos
	confirmMsg := fmt.Sprintf(`⚠️ <b>¿Eliminar este gasto?</b>

📝 <b>%s</b>
💰 Monto: %s
👤 Creado por: %s
📅 Fecha: %s
%s %s
🆔 ID: <code>%s</code>

<i>El gasto irá a la papelera por 30 días.</i>`,
		telegram.EscapeHTML(expense.Description),
		telegram.FormatMoney(expense.TotalAmount),
		telegram.EscapeHTML(expense.CreatorName),
		expense.CreatedAt.Format("02/01/2006"),
		statusIcon,
		statusText,
		shortID)

	// Crear teclado inline con botones de confirmación
	keyboard := telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{
					Text:         "✅ Sí, eliminar",
					CallbackData: fmt.Sprintf("delete_confirm:%s", shortID),
				},
				{
					Text:         "❌ Cancelar",
					CallbackData: fmt.Sprintf("delete_cancel:%s", shortID),
				},
			},
		},
	}

	// Enviar mensaje con botones
	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        confirmMsg,
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
}

// handleTrash muestra los gastos eliminados (papelera)
func (h *Handler) handleTrash(ctx context.Context, chatID int64) error {
	expenses, err := h.db.GetDeletedExpenses(ctx, chatID, 20)
	if err != nil {
		h.logger.Printf("Error getting deleted expenses: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener la papelera.")
	}

	if len(expenses) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🗑️ <b>Papelera vacía</b>\n\nNo hay gastos eliminados.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🗑️ <b>Papelera</b> (%d gastos)\n\n", len(expenses)))

	for i, expense := range expenses {
		shortID := expense.ID[:8]
		deletedInfo := ""
		if !expense.DeletedAt.IsZero() {
			daysAgo := int(time.Since(expense.DeletedAt).Hours() / 24)
			daysLeft := 30 - daysAgo
			if daysLeft < 0 {
				daysLeft = 0
			}
			deletedInfo = fmt.Sprintf(" (expira en %d días)", daysLeft)
		}

		sb.WriteString(fmt.Sprintf("%d. <b>%s</b>\n", i+1, telegram.EscapeHTML(expense.Description)))
		sb.WriteString(fmt.Sprintf("   💰 %s | 🆔 <code>%s</code>%s\n\n", telegram.FormatMoney(expense.TotalAmount), shortID, deletedInfo))
	}

	sb.WriteString("<i>Usa /restaurar [id] para recuperar un gasto.</i>\n")
	sb.WriteString("<i>Los gastos se eliminan permanentemente después de 30 días.</i>")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleRestore restaura un gasto eliminado
func (h *Handler) handleRestore(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /restaurar [id_gasto]

💡 Usa /papelera para ver los gastos eliminados y sus IDs.`)
	}

	shortID := args[0]

	// Buscar el gasto incluyendo eliminados
	expense, err := h.db.GetExpenseByShortIDIncludeDeleted(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado en la papelera")
	}

	if !expense.IsDeleted {
		return h.tg.SendMessage(ctx, chatID, "⚠️ Este gasto no está eliminado")
	}

	err = h.db.RestoreExpense(ctx, expense.PK, expense.SK)
	if err != nil {
		h.logger.Printf("Error restoring expense: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al restaurar el gasto")
	}

	statusText := "Pendiente de dividir"
	if expense.IsDivided {
		statusText = "Dividido"
	}

	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf(`✅ <b>Gasto restaurado</b>

📝 %s
💰 %s
📊 %s

El gasto está disponible nuevamente.`, telegram.EscapeHTML(expense.Description), telegram.FormatMoney(expense.TotalAmount), statusText))
}

// handleSimplify maneja simplificación de deudas
func (h *Handler) handleSimplify(ctx context.Context, chatID int64) error {
	transactions, err := h.db.SimplifyDebts(ctx, chatID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al simplificar deudas")
	}

	if len(transactions) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No hay deudas pendientes!")
	}

	members, _ := h.db.GetGroupMembers(ctx, chatID)
	memberNames := make(map[int64]string)
	for _, m := range members {
		memberNames[m.UserID] = m.DisplayName
	}

	var sb strings.Builder
	sb.WriteString("💡 <b>Deudas Simplificadas</b>\n\n")
	sb.WriteString("Para saldar todas las cuentas:\n\n")

	for debtorID, creditors := range transactions {
		debtorName := memberNames[debtorID]
		for creditorID, amount := range creditors {
			creditorName := memberNames[creditorID]
			sb.WriteString(fmt.Sprintf("• %s → %s: <b>%s</b>\n", debtorName, creditorName, telegram.FormatMoney(amount)))
		}
	}

	sb.WriteString("\n_Esto minimiza el número de transacciones necesarias._")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleCategories maneja categorías disponibles
func (h *Handler) handleCategories(ctx context.Context, chatID int64) error {
	message := `📂 <b>Categorías Disponibles</b>

• 🍕 comida - Comidas y restaurantes
• 🚗 transporte - Taxis, nafta, peajes
• 🏠 alojamiento - Hoteles, Airbnb
• 🎉 entretenimiento - Cine, eventos
• 🛒 compras - Supermercado, shopping
• 💊 salud - Farmacia, médicos
• 📱 servicios - Internet, luz, gas
• ✈️ viajes - Vuelos, turismo
• 🎓 educacion - Cursos, libros
• 🔧 otros - Otros gastos

Para asignar categoría al crear:
/nuevo_gasto Cena 15000 categoria:comida`

	return h.tg.SendMessage(ctx, chatID, message)
}
