package bot

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/abettucci/group-split-bot/internal/security"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// handleMenu muestra el menú principal con botones inline
func (h *Handler) handleMenu(ctx context.Context, chatID int64, userID ...int64) error {
	if len(userID) > 0 {
		h.conv.Set(chatID, userID[0], &ConversationState{Step: StepSelectMenuOption})
	}

	keyboard := telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "💰 Nuevo gasto", CallbackData: "menu:nuevo_gasto"},
				{Text: "📋 Ver gastos", CallbackData: "menu:ver_gastos"},
			},
			{
				{Text: "💳 Mis deudas", CallbackData: "menu:mis_deudas"},
				{Text: "📊 Balance", CallbackData: "menu:balance"},
			},
			{
				{Text: "➗ Dividir gasto", CallbackData: "menu:dividir"},
				{Text: "🔄 Redividir", CallbackData: "menu:redividir"},
			},
			{
				{Text: "🐀 Recordatorios", CallbackData: "menu:recordatorios"},
				{Text: "🐀 Avisar deudas", CallbackData: "menu:recordar_deudas"},
			},
			{
				{Text: "👥 Miembros", CallbackData: "menu:miembros"},
				{Text: "❓ Ayuda", CallbackData: "menu:ayuda"},
			},
			{
				{Text: "💸 Pagar una deuda", CallbackData: "menu:pagar_deuda"},
			},
			{
				{Text: "🧪 Crear personas de prueba", CallbackData: "menu:crear_prueba"},
				{Text: "🎲 Simular operaciones", CallbackData: "menu:simular_prueba"},
			},
			{
				{Text: "🧹 Limpiar datos de prueba", CallbackData: "menu:limpiar_prueba"},
			},
			{
				{Text: "🧾 Resumir una lista", CallbackData: "menu:resumir_lista"},
			},
		},
	}

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "🤖 <b>¿Qué querés hacer?</b>\n\nElegí una opción o escribí lo que necesitás:",
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
}

// handleMenuCallback enruta los callbacks del menú principal
func (h *Handler) handleMenuCallback(ctx context.Context, chatID, userID int64, userName, option, callbackID string) error {
	_ = h.tg.AnswerCallbackQuery(ctx, callbackID, "")
	return h.runMenuAction(ctx, chatID, userID, userName, option)
}

// runMenuAction is shared by Telegram callbacks, WhatsApp interactive
// responses, numbered fallbacks, and natural-language menu choices.
func (h *Handler) runMenuAction(ctx context.Context, chatID, userID int64, userName, option string) error {
	switch option {
	case "nuevo_gasto":
		return h.startExpenseFlow(ctx, chatID, userID)
	case "ver_gastos":
		return h.handleViewExpenses(ctx, chatID)
	case "mis_deudas":
		return h.handleMyDebts(ctx, chatID, userID)
	case "balance":
		return h.handleBalance(ctx, chatID)
	case "dividir":
		return h.handleMenuDivide(ctx, chatID, userID)
	case "redividir":
		return h.handleMenuRedivide(ctx, chatID, userID)
	case "recordatorios":
		return h.handleMyReminders(ctx, chatID, userID)
	case "recordar_deudas":
		return h.handleRemindDebtors(ctx, chatID, userID)
	case "miembros":
		return h.handleMembers(ctx, chatID)
	case "ayuda":
		return h.handleHelp(ctx, chatID)
	case "pagar_deuda":
		return h.handleMenuPay(ctx, chatID, userID)
	case "crear_prueba":
		return h.handleCreateTestUsers(ctx, chatID, userID, []string{"default"})
	case "simular_prueba":
		return h.handleRunTestSimulation(ctx, chatID, userID)
	case "limpiar_prueba":
		return h.handleClearTestData(ctx, chatID, userID)
	case "resumir_lista":
		return h.startExpenseSummary(ctx, chatID, userID)
	case "mas_opciones":
		return h.handleMoreMenu(ctx, chatID, userID, userName, 1)
	case "mas_opciones_2":
		return h.handleMoreMenu(ctx, chatID, userID, userName, 2)
	case "mas_opciones_3":
		return h.handleMoreMenu(ctx, chatID, userID, userName, 3)
	case "mas_opciones_4":
		return h.handleMoreMenu(ctx, chatID, userID, userName, 4)
	case "mas_opciones_5":
		return h.handleMoreMenu(ctx, chatID, userID, userName, 5)
	default:
		return nil
	}
}

// handleMoreMenu creates compact pages that experimental WhatsApp reply
// buttons can render (three actions at a time). Telegram still receives the
// complete inline keyboard from handleMenu.
func (h *Handler) handleMoreMenu(ctx context.Context, chatID, userID int64, userName string, page int) error {
	pages := [][][]telegram.InlineKeyboardButton{
		{{{Text: "💳 Mis deudas", CallbackData: "menu:mis_deudas"}, {Text: "📊 Balance", CallbackData: "menu:balance"}, {Text: "➡️ Más", CallbackData: "menu:mas_opciones_2"}}},
		{{{Text: "➗ Dividir gasto", CallbackData: "menu:dividir"}, {Text: "🔄 Redividir", CallbackData: "menu:redividir"}, {Text: "➡️ Más", CallbackData: "menu:mas_opciones_3"}}},
		{{{Text: "🐀 Recordatorios", CallbackData: "menu:recordatorios"}, {Text: "🐀 Avisar deudas", CallbackData: "menu:recordar_deudas"}, {Text: "➡️ Más", CallbackData: "menu:mas_opciones_4"}}},
		{{{Text: "👥 Miembros", CallbackData: "menu:miembros"}, {Text: "💸 Pagar una deuda", CallbackData: "menu:pagar_deuda"}, {Text: "➡️ Más", CallbackData: "menu:mas_opciones_5"}}},
		{{{Text: "❓ Ayuda", CallbackData: "menu:ayuda"}, {Text: "🧾 Resumir una lista", CallbackData: "menu:resumir_lista"}, {Text: "🧪 Crear pruebas", CallbackData: "menu:crear_prueba"}}, {{Text: "🎲 Simular operaciones", CallbackData: "menu:simular_prueba"}, {Text: "🧹 Limpiar pruebas", CallbackData: "menu:limpiar_prueba"}}},
	}
	if page < 1 || page > len(pages) {
		return h.handleMenu(ctx, chatID, userID)
	}

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "🤖 <b>Más opciones</b>\n\nElegí qué querés hacer:",
		ParseMode:   "HTML",
		ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: pages[page-1]},
	})
}

// handleMenuRedivide muestra los gastos ya divididos. Elegir uno lo vuelve a
// dividir entre todos los miembros actuales del grupo.
func (h *Handler) handleMenuRedivide(ctx context.Context, chatID, userID int64) error {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 10)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los gastos.")
	}

	var rows [][]telegram.InlineKeyboardButton
	redivideOptions := map[int]string{}
	n := 1
	for _, exp := range expenses {
		if !exp.IsDivided {
			continue
		}
		shortID := exp.ID[:8]
		label := fmt.Sprintf("🔄 %s — %s", exp.Description, telegram.FormatMoney(exp.TotalAmount))
		if len(label) > 60 {
			label = label[:57] + "..."
		}
		rows = append(rows, []telegram.InlineKeyboardButton{{
			Text: label, CallbackData: fmt.Sprintf("redivide:%s", shortID),
		}})
		redivideOptions[n] = shortID
		n++
	}

	if len(rows) == 0 {
		return h.tg.SendMessage(ctx, chatID, "ℹ️ Todavía no hay gastos divididos para redividir.")
	}
	h.conv.Set(chatID, userID, &ConversationState{
		Step:          StepSelectRedivideExpense,
		DivideOptions: redivideOptions,
	})

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "🔄 <b>¿Qué gasto querés redividir?</b>\n\nElegí uno para repartirlo entre los miembros actuales del grupo.",
		ParseMode:   "HTML",
		ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
}

// handleMenuDivide muestra los gastos pendientes de dividir como botones clickeables.
// También guarda estado de conversación para que en WhatsApp el usuario pueda responder con un número.
func (h *Handler) handleMenuDivide(ctx context.Context, chatID int64, userID ...int64) error {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 10)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los gastos.")
	}

	divideOptions := map[int]string{}
	var rows [][]telegram.InlineKeyboardButton
	var onlyPendingExpenseID string
	n := 1
	for _, exp := range expenses {
		if !exp.IsDivided {
			shortID := exp.ID[:8]
			divideOptions[n] = shortID
			onlyPendingExpenseID = shortID
			label := fmt.Sprintf("📝 %s — %s", exp.Description, telegram.FormatMoney(exp.TotalAmount))
			if len(label) > 60 {
				label = label[:57] + "..."
			}
			rows = append(rows, []telegram.InlineKeyboardButton{
				{Text: label, CallbackData: fmt.Sprintf("divide:%s", shortID)},
			})
			n++
		}
	}

	if len(rows) == 0 {
		return h.tg.SendMessage(ctx, chatID, "✅ Todos los gastos ya fueron divididos.")
	}

	// WhatsApp no preserva un estado de conversación entre invocaciones de
	// Lambda. Si sólo queda un gasto pendiente, no le pedimos a la persona un
	// segundo número: "Dividir gasto" completa la operación en ese momento.
	// Esto también evita que una respuesta a un menú anterior quede ambigua.
	if len(divideOptions) == 1 {
		initiatorID := int64(0)
		if len(userID) > 0 {
			initiatorID = userID[0]
		}
		return h.handleDivide(ctx, chatID, initiatorID, []string{onlyPendingExpenseID})
	}

	// Telegram usa botones; WhatsApp puede responder citando esta lista. El
	// contexto citado se resuelve de forma stateless en handleQuotedMenuChoice.
	if len(userID) > 0 {
		h.conv.Set(chatID, userID[0], &ConversationState{
			Step:          StepSelectDivideExpense,
			DivideOptions: divideOptions,
		})
	}

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "➗ <b>¿Qué gasto querés dividir?</b>\n\nElegí uno para dividirlo entre todos:",
		ParseMode:   "HTML",
		ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
}

// startExpenseFlow inicia el flujo conversacional para registrar un nuevo gasto
func (h *Handler) startExpenseFlow(ctx context.Context, chatID, userID int64) error {
	h.conv.Set(chatID, userID, &ConversationState{Step: StepNewExpenseDescription})
	return h.tg.SendMessage(ctx, chatID, `💰 <b>Nuevo gasto</b>

¿Cuál es la descripción del gasto?

<i>Escribe /cancelar en cualquier momento para cancelar.</i>`)
}

// cancelKeywords son las palabras que cancelan cualquier flujo activo
var cancelKeywords = []string{
	"/cancelar", "cancelar", "cancel", "salir", "volver", "atras", "atrás",
	"menu", "menú", "inicio", "stop", "exit", "no", "nada",
}

// handleConversationStep procesa el siguiente paso de una conversación en curso
func (h *Handler) handleConversationStep(ctx context.Context, chatID, userID int64, userName, text string, state *ConversationState) error {
	lower := strings.ToLower(strings.TrimSpace(text))

	for _, kw := range cancelKeywords {
		if lower == kw {
			h.conv.Clear(chatID, userID)
			return h.tg.SendMessage(ctx, chatID, "❌ Operación cancelada. Escribí /menu o «menu» para ver las opciones.")
		}
	}

	switch state.Step {
	case StepSelectNewExpenseAction:
		lower := strings.ToLower(strings.TrimSpace(text))
		switch {
		case lower == "1" || strings.Contains(lower, "dividir"):
			h.conv.Clear(chatID, userID)
			return h.handleDivide(ctx, chatID, userID, []string{state.ExpenseShortID})
		case lower == "2" || strings.Contains(lower, "ver gasto"):
			h.conv.Clear(chatID, userID)
			return h.handleViewExpenses(ctx, chatID)
		default:
			return h.tg.SendMessage(ctx, chatID, "❌ Elegí 1 para dividir el gasto recién creado o 2 para ver los gastos.")
		}

	case StepSelectMenuOption:
		h.conv.Clear(chatID, userID)
		if handled, err := h.handleMenuNumber(ctx, chatID, userID, text); handled {
			return err
		}
		if option, ok := resolveMenuOptionText(text); ok {
			return h.runMenuAction(ctx, chatID, userID, userName, option)
		}
		if args, ok := naturalExpenseArgs(text); ok {
			return h.handleNewExpense(ctx, chatID, userID, userName, args)
		}
		return h.tg.SendMessage(ctx, chatID, "❌ Opción no válida. Mencioná a Splitter o escribí «menu» para ver las opciones de nuevo.")

	case StepNewExpenseDescription:
		description, valid := security.ValidateDescription(text)
		if !valid {
			return h.tg.SendMessage(ctx, chatID, "❌ La descripción contiene caracteres no permitidos. Intentá de nuevo:")
		}
		state.Description = description
		state.Step = StepNewExpenseAmount
		h.conv.Set(chatID, userID, state)
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf(
			"💵 ¿Cuánto fue el total de <b>%s</b>?\n\n<i>Solo el número (ej: 15000 o 1500.50)</i>",
			telegram.EscapeHTML(description),
		))

	case StepNewExpenseAmount:
		amountStr := strings.ReplaceAll(text, ",", ".")
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil || amount <= 0 {
			return h.tg.SendMessage(ctx, chatID, "❌ Ingresá un monto válido (ej: 15000 o 1500.50):")
		}
		if !security.ValidateAmount(amount) {
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf(
				"❌ El monto debe estar entre %s y %s",
				telegram.FormatMoney(security.MinAmountValue),
				telegram.FormatMoney(security.MaxAmountValue),
			))
		}
		state.Amount = amount
		state.Step = StepNewExpensePayer
		h.conv.Set(chatID, userID, state)
		return h.showPayerSelection(ctx, chatID, userID, userName, state.Description)

	case StepNewExpensePayer:
		return h.resolvePayerFromText(ctx, chatID, userID, userName, text, state)

	case StepSelectDivideExpense:
		// Intentar número primero
		if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
			if shortID, ok := state.DivideOptions[n]; ok {
				h.conv.Clear(chatID, userID)
				return h.handleDivide(ctx, chatID, userID, []string{shortID})
			}
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Opción %d no válida. Escribí «dividir» para ver la lista de nuevo.", n))
		}
		// Intentar como shortID directo
		h.conv.Clear(chatID, userID)
		return h.handleDivide(ctx, chatID, userID, []string{strings.TrimSpace(text)})

	case StepSelectRedivideExpense:
		h.conv.Clear(chatID, userID)
		if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
			if shortID, ok := state.DivideOptions[n]; ok {
				return h.handleRedivide(ctx, chatID, userID, []string{shortID})
			}
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Opción %d no válida. Pedí «redividir» para ver la lista de nuevo.", n))
		}
		return h.handleRedivide(ctx, chatID, userID, []string{strings.TrimSpace(text)})

	case StepSelectPaymentExpense:
		if n, err := strconv.Atoi(strings.TrimSpace(text)); err == nil {
			return h.handlePaymentChoice(ctx, chatID, userID, n)
		}
		return h.tg.SendMessage(ctx, chatID, "❌ Respondé con el número de una deuda de la lista.")

	case StepExpenseSummary:
		h.conv.Clear(chatID, userID)
		return h.handleExpenseSummary(ctx, chatID, userID, userName, text)
	}
	return nil
}

// showPayerSelection muestra botones con los miembros del grupo para elegir quién pagó
func (h *Handler) showPayerSelection(ctx context.Context, chatID, userID int64, userName, description string) error {
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil || len(members) == 0 {
		return h.tg.SendMessage(ctx, chatID, "👤 ¿Quién pagó? Escribí el nombre o @usuario, o <b>yo</b> si fuiste vos:")
	}

	var rows [][]telegram.InlineKeyboardButton

	// "Yo pagué" siempre primero
	rows = append(rows, []telegram.InlineKeyboardButton{
		{Text: fmt.Sprintf("🙋 Yo pagué (%s)", userName), CallbackData: "conv_payer:self"},
	})

	// Miembros en filas de 2
	var row []telegram.InlineKeyboardButton
	for _, m := range members {
		if m.UserID == userID {
			continue
		}
		row = append(row, telegram.InlineKeyboardButton{
			Text:         m.DisplayName,
			CallbackData: fmt.Sprintf("conv_payer:%d", m.UserID),
		})
		if len(row) == 2 {
			rows = append(rows, row)
			row = nil
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        fmt.Sprintf("👤 ¿Quién pagó <b>%s</b>?\n\nElegí con los botones o escribí el nombre:", telegram.EscapeHTML(description)),
		ParseMode:   "HTML",
		ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
}

// resolvePayerFromText busca un miembro por nombre/username escrito en texto libre
func (h *Handler) resolvePayerFromText(ctx context.Context, chatID, userID int64, userName, text string, state *ConversationState) error {
	payerName := strings.TrimPrefix(strings.TrimSpace(text), "@")
	lower := strings.ToLower(payerName)

	if lower == "yo" || lower == "yo mismo" || lower == "yo misma" {
		return h.createExpenseFromConversation(ctx, chatID, userID, userName, userID, userName, state)
	}

	members, _ := h.db.GetGroupMembers(ctx, chatID)
	var payerUserID int64
	var payerUserName string

	// Buscar por username exacto
	for _, m := range members {
		if m.Username != "" && strings.EqualFold(m.Username, payerName) {
			payerUserID, payerUserName = m.UserID, m.DisplayName
			break
		}
	}

	// Buscar por nombre completo
	if payerUserID == 0 {
		for _, m := range members {
			if strings.EqualFold(m.DisplayName, payerName) {
				payerUserID, payerUserName = m.UserID, m.DisplayName
				break
			}
		}
	}

	// Coincidencia parcial
	if payerUserID == 0 {
		for _, m := range members {
			if strings.Contains(strings.ToLower(m.DisplayName), lower) {
				payerUserID, payerUserName = m.UserID, m.DisplayName
				break
			}
		}
	}

	if payerUserID == 0 {
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf(
			"❌ No encontré a <b>%s</b> en el grupo.\n\nEscribí <b>yo</b> si fuiste vos, o elegí con los botones de arriba.",
			telegram.EscapeHTML(payerName),
		))
	}

	return h.createExpenseFromConversation(ctx, chatID, userID, userName, payerUserID, payerUserName, state)
}

// handlePayerCallback procesa la selección de pagador via botón inline
func (h *Handler) handlePayerCallback(ctx context.Context, chatID, userID int64, userName, payerIDStr, callbackID string) error {
	_ = h.tg.AnswerCallbackQuery(ctx, callbackID, "")

	state := h.conv.Get(chatID, userID)
	if state == nil || state.Step != StepNewExpensePayer {
		return h.tg.SendMessage(ctx, chatID, "❌ La sesión expiró. Iniciá el gasto de nuevo con /nuevo_gasto o /menu")
	}

	payerUserID := userID
	payerUserName := userName

	if payerIDStr != "self" {
		id, err := strconv.ParseInt(payerIDStr, 10, 64)
		if err == nil {
			members, _ := h.db.GetGroupMembers(ctx, chatID)
			for _, m := range members {
				if m.UserID == id {
					payerUserID, payerUserName = id, m.DisplayName
					break
				}
			}
		}
	}

	return h.createExpenseFromConversation(ctx, chatID, userID, userName, payerUserID, payerUserName, state)
}

// createExpenseFromConversation persiste el gasto al final del flujo conversacional
func (h *Handler) createExpenseFromConversation(ctx context.Context, chatID, userID int64, userName string, payerUserID int64, payerUserName string, state *ConversationState) error {
	h.conv.Clear(chatID, userID)

	expense, err := h.db.CreateExpense(ctx, chatID, state.Description, state.Amount, payerUserID, payerUserName)
	if err != nil {
		h.logger.Printf("Error creating expense from conversation: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al crear el gasto. Intentá de nuevo con /nuevo_gasto")
	}

	shortID := expense.ID[:8]
	h.logChangelog(chatID, userID, userName, "expense", shortID, state.Description, "created", map[string]string{
		"monto":      telegram.FormatMoney(state.Amount),
		"pagado por": payerUserName,
	})

	var registeredByMsg string
	if payerUserID != userID {
		registeredByMsg = fmt.Sprintf("\n📝 Registrado por: %s", userName)
	}

	keyboard := telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{Text: "➗ Dividir entre todos", CallbackData: fmt.Sprintf("divide:%s", shortID)},
				{Text: "📋 Ver gastos", CallbackData: "menu:ver_gastos"},
			},
		},
	}
	h.conv.Set(chatID, userID, &ConversationState{
		Step:           StepSelectNewExpenseAction,
		ExpenseShortID: shortID,
	})

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID: chatID,
		Text: fmt.Sprintf(
			"✅ <b>Gasto registrado</b>\n\n📝 <b>%s</b>\n💰 Monto: %s\n👤 Pagado por: %s%s\n🆔 ID: <code>%s</code>",
			telegram.EscapeHTML(state.Description),
			telegram.FormatMoney(state.Amount),
			telegram.EscapeHTML(payerUserName),
			registeredByMsg,
			shortID,
		),
		ParseMode:   "HTML",
		ReplyMarkup: keyboard,
	})
}

// handleNaturalLanguage interpreta texto libre. En grupos solo se invoca al
// mencionar Splitter, por lo que no interrumpe conversaciones ajenas.
func (h *Handler) handleNaturalLanguage(ctx context.Context, chatID, userID int64, userName, text string, showMenuOnUnknown bool) error {
	lower := strings.ToLower(strings.TrimSpace(text))

	// Acceso por número: sirve de respaldo en WhatsApp y también funciona si
	// la persona responde citando el mensaje del bot.
	if handled, err := h.handleMenuNumber(ctx, chatID, userID, lower); handled {
		return err
	}
	if option, ok := resolveMenuOptionText(text); ok {
		return h.runMenuAction(ctx, chatID, userID, userName, option)
	}
	// A short sentence with an explicit expense verb and an amount is safe to
	// register directly when it was addressed to Splitter. For example:
	// "@Splitter cargué la cena 15000". In a group this function is reached
	// only after a mention (or an intentional reply), so normal chat is never
	// interpreted as a financial operation.
	if args, ok := naturalExpenseArgs(text); ok {
		return h.handleNewExpense(ctx, chatID, userID, userName, args)
	}

	type trigger struct {
		keywords []string
		action   func() error
	}

	triggers := []trigger{
		{
			keywords: []string{"resumir gastos", "resumir una lista", "resumen de gastos", "resumen de la lista", "sumar gastos y reintegros", "sumar gastos", "calcular reintegros", "resumen con reintegros"},
			action:   func() error { return h.startExpenseSummary(ctx, chatID, userID) },
		},
		{
			keywords: []string{"hola", "holi", "buenas", "buenos dias", "buenos días", "buen dia", "buen día"},
			action:   func() error { return h.handleMenu(ctx, chatID, userID) },
		},
		{
			// Keep this intent explicit. The generic word "gasto" also appears
			// in requests such as "ver gastos" and "dividir gasto", which must
			// route to their own menu actions below.
			keywords: []string{"nuevo gasto", "gasto nuevo", "agregar gasto", "anotar gasto", "cargar gasto", "cargar un gasto", "registrar gasto", "sumar gasto", "nuevo"},
			action:   func() error { return h.startExpenseFlow(ctx, chatID, userID) },
		},
		{
			keywords: []string{"ver gastos", "mis gastos", "lista de gastos", "gastos del grupo", "últimos gastos", "listar gastos"},
			action:   func() error { return h.handleViewExpenses(ctx, chatID) },
		},
		{
			keywords: []string{"mis deudas", "cuánto debo", "cuanto debo", "qué debo", "que debo", "deudas pendientes", "deudas"},
			action:   func() error { return h.handleMyDebts(ctx, chatID, userID) },
		},
		{
			keywords: []string{"pagar una deuda", "marcar pago", "marcar como pagado", "ya pagué", "ya pague", "pagué", "pague", "pagar deuda"},
			action:   func() error { return h.handleMenuPay(ctx, chatID, userID) },
		},
		{
			keywords: []string{"balance", "estado de cuentas", "cómo estamos", "como estamos", "resumen"},
			action:   func() error { return h.handleBalance(ctx, chatID) },
		},
		{
			keywords: []string{"simplificar", "optimizar deudas", "simplify"},
			action:   func() error { return h.handleSimplify(ctx, chatID) },
		},
		{
			keywords: []string{"miembros", "integrantes", "quiénes somos", "quienes somos", "quién está", "quien esta"},
			action:   func() error { return h.handleMembers(ctx, chatID) },
		},
		{
			keywords: []string{"dividir gasto", "quiero dividir", "separar gasto", "dividir"},
			action:   func() error { return h.handleMenuDivide(ctx, chatID, userID) },
		},
		{
			keywords: []string{"redividir", "volver a dividir", "repartir de nuevo"},
			action:   func() error { return h.handleMenuRedivide(ctx, chatID, userID) },
		},
		{
			keywords: []string{"mis recordatorios", "ver recordatorios", "recordatorios", "recordatorio"},
			action:   func() error { return h.handleMyReminders(ctx, chatID, userID) },
		},
		{
			keywords: []string{"recordar deudas", "avisar deudas", "recordarles", "avisales", "avisar a los deudores"},
			action:   func() error { return h.handleRemindDebtors(ctx, chatID, userID) },
		},
		{
			keywords: []string{"ayuda", "cómo funciona", "como funciona", "qué podés hacer", "que podes hacer", "comandos"},
			action:   func() error { return h.handleHelp(ctx, chatID) },
		},
		{
			keywords: []string{"menú", "menu", "opciones", "qué puedo hacer", "que puedo hacer", "inicio"},
			action:   func() error { return h.handleMenu(ctx, chatID, userID) },
		},
	}

	for _, t := range triggers {
		for _, kw := range t.keywords {
			if strings.Contains(lower, kw) {
				return t.action()
			}
		}
	}

	if showMenuOnUnknown {
		return h.handleMenu(ctx, chatID, userID)
	}

	// No match - suggest the menu
	return h.tg.SendMessage(ctx, chatID, "🤖 No entendí eso. Escribí /menu o pedime una opción como «nuevo gasto», «recordatorios» o «balance».")
}

// handleMenuNumber ejecuta una opción del menú de texto. El bool permite
// distinguir una opción inválida de un error de negocio al procesarla.
func (h *Handler) handleMenuNumber(ctx context.Context, chatID, userID int64, text string) (bool, error) {
	number := strings.TrimSpace(text)
	if match := menuOptionNumberPattern.FindStringSubmatch(number); len(match) == 2 {
		number = match[1]
	}
	option, ok := menuOptionNumbers[number]
	if !ok {
		return false, nil
	}
	return true, h.runMenuAction(ctx, chatID, userID, "", option)
}

var (
	shortExpenseIDPattern   = regexp.MustCompile(`(?i)\b[a-f0-9]{8}\b`)
	menuOptionNumberPattern = regexp.MustCompile(`(?i)^\s*(?:opci[oó]n\s*)?([0-9]{1,2})(?:[.)\s]|$)`)
	menuMentionPattern      = regexp.MustCompile(`@[[:alnum:]_+.-]+`)
	menuNoisePattern        = regexp.MustCompile(`[\p{P}\p{S}]+`)
	naturalExpensePrefix    = regexp.MustCompile(`(?i)^\s*(?:yo\s+)?(?:cargu[eé]|registr[eé]|anot[eé]|agregu[eé]|sum[eé]|pag[ué]|pague)\s+(?:(?:el|la|un|una)\s+)?`)
)

type menuTextOption struct {
	action  string
	aliases []string
}

var menuOptionNumbers = map[string]string{
	"1": "nuevo_gasto", "2": "ver_gastos", "3": "mis_deudas", "4": "balance",
	"5": "dividir", "6": "redividir", "7": "recordatorios", "8": "recordar_deudas",
	"9": "miembros", "10": "ayuda", "11": "pagar_deuda", "12": "crear_prueba",
	"13": "simular_prueba", "14": "limpiar_prueba", "15": "resumir_lista",
}

var menuTextOptions = []menuTextOption{
	{action: "nuevo_gasto", aliases: []string{"nuevo gasto", "gasto nuevo", "agregar gasto", "anotar gasto", "cargar gasto", "registrar gasto", "sumar gasto"}},
	{action: "ver_gastos", aliases: []string{"ver gastos", "mis gastos", "lista de gastos", "gastos del grupo", "ultimos gastos", "listar gastos"}},
	{action: "mis_deudas", aliases: []string{"mis deudas", "cuanto debo", "que debo", "deudas pendientes"}},
	{action: "balance", aliases: []string{"balance", "estado de cuentas", "como estamos"}},
	{action: "dividir", aliases: []string{"dividir gasto", "quiero dividir", "separar gasto", "dividir"}},
	{action: "redividir", aliases: []string{"redividir", "volver a dividir", "repartir de nuevo"}},
	{action: "recordatorios", aliases: []string{"mis recordatorios", "ver recordatorios", "recordatorios", "recordatorio"}},
	{action: "recordar_deudas", aliases: []string{"recordar deudas", "avisar deudas", "recordarles", "avisales", "avisar a los deudores"}},
	{action: "miembros", aliases: []string{"miembros", "integrantes", "quienes somos", "quien esta"}},
	{action: "ayuda", aliases: []string{"ayuda", "como funciona", "que podes hacer"}},
	{action: "pagar_deuda", aliases: []string{"pagar una deuda", "marcar pago", "marcar como pagado", "ya pague", "pagar deuda"}},
	{action: "crear_prueba", aliases: []string{"crear personas de prueba", "crear pruebas", "personas de prueba"}},
	{action: "simular_prueba", aliases: []string{"simular operaciones", "simulacion", "simular"}},
	{action: "limpiar_prueba", aliases: []string{"limpiar datos de prueba", "limpiar pruebas"}},
	{action: "resumir_lista", aliases: []string{"resumir una lista", "resumir lista", "resumir gastos", "resumen de gastos", "resumen lista", "sumar gastos y reintegros", "calcular reintegros"}},
}

// resolveMenuOptionText handles visible menu labels as well as small typing
// mistakes. It only returns a fuzzy result when it is unambiguous, so the bot
// never turns a casual message into an unrelated financial action.
func resolveMenuOptionText(text string) (string, bool) {
	selection := normalizeMenuText(text)
	if selection == "" {
		return "", false
	}

	if match := menuOptionNumberPattern.FindStringSubmatch(selection); len(match) == 2 {
		if action, ok := menuOptionNumbers[match[1]]; ok {
			return action, true
		}
	}

	for _, option := range menuTextOptions {
		for _, alias := range option.aliases {
			if selection == alias || strings.Contains(selection, alias) {
				return option.action, true
			}
		}
	}

	bestAction := ""
	bestDistance := -1
	ambiguous := false
	for _, option := range menuTextOptions {
		for _, alias := range option.aliases {
			distance := menuLevenshteinDistance(selection, alias)
			if distance > allowedMenuTypos(alias) {
				continue
			}
			if bestDistance == -1 || distance < bestDistance {
				bestAction, bestDistance, ambiguous = option.action, distance, false
			} else if distance == bestDistance && option.action != bestAction {
				ambiguous = true
			}
		}
	}
	return bestAction, bestDistance >= 0 && !ambiguous
}

func normalizeMenuText(text string) string {
	text = strings.ToLower(menuMentionPattern.ReplaceAllString(text, " "))
	replacer := strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
	text = replacer.Replace(text)
	text = menuNoisePattern.ReplaceAllString(text, " ")
	return strings.Join(strings.Fields(text), " ")
}

// naturalExpenseArgs recognizes a deliberately narrow, conversational form of
// /nuevo_gasto. It returns regular command arguments so the established
// validation, member lookup and expense creation logic remains the one source
// of truth.
func naturalExpenseArgs(text string) ([]string, bool) {
	// The command may arrive as "@Splitter cargué ...". Remove mentions only
	// for intent recognition; a payer can still be selected in the normal flow.
	candidate := strings.TrimSpace(menuMentionPattern.ReplaceAllString(text, " "))
	if !naturalExpensePrefix.MatchString(candidate) {
		return nil, false
	}
	candidate = strings.TrimSpace(naturalExpensePrefix.ReplaceAllString(candidate, ""))
	fields := strings.Fields(candidate)
	if len(fields) < 2 {
		return nil, false
	}

	for i, field := range fields {
		amountToken := strings.Trim(strings.TrimSpace(field), "$€£")
		amount, ok := parseSummaryAmount(amountToken)
		if !ok || i == 0 {
			continue
		}
		// handleNewExpense accepts its standard [description] [amount] shape.
		// Formatting through ParseFloat makes "$15.000" and "15,000" safe too.
		args := append([]string{}, fields[:i]...)
		args = append(args, strconv.FormatFloat(amount, 'f', -1, 64))
		return args, true
	}
	return nil, false
}

func allowedMenuTypos(alias string) int {
	length := len([]rune(strings.ReplaceAll(alias, " ", "")))
	if length <= 6 {
		return 1
	}
	if length <= 14 {
		return 2
	}
	return 3
}

func menuLevenshteinDistance(left, right string) int {
	leftRunes, rightRunes := []rune(left), []rune(right)
	previous := make([]int, len(rightRunes)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, leftRune := range leftRunes {
		current := make([]int, len(rightRunes)+1)
		current[0] = i + 1
		for j, rightRune := range rightRunes {
			cost := 0
			if leftRune != rightRune {
				cost = 1
			}
			current[j+1] = minInt(current[j]+1, previous[j+1]+1, previous[j]+cost)
		}
		previous = current
	}
	return previous[len(rightRunes)]
}

func minInt(values ...int) int {
	minimum := values[0]
	for _, value := range values[1:] {
		if value < minimum {
			minimum = value
		}
	}
	return minimum
}

// handleQuotedMenuChoice resolves a numeric reply against the bot message it
// quotes, instead of against whichever temporary conversation happened last.
// This lets people return to an older menu after viewing expenses.
func (h *Handler) handleQuotedMenuChoice(ctx context.Context, chatID, userID int64, text, quotedText string) (bool, error) {
	selection := strings.TrimSpace(text)
	quotedLower := strings.ToLower(quotedText)

	// WhatsApp suele mostrar en la cita solamente la cabecera y "...". La
	// cabecera es única entre los mensajes del bot, por lo que alcanza para
	// reconocer el menú principal aunque el preview haya sido truncado.
	if strings.Contains(quotedLower, "¿qué querés hacer?") {
		h.conv.Clear(chatID, userID)
		if _, err := strconv.Atoi(selection); err == nil {
			return h.handleMenuNumber(ctx, chatID, userID, selection)
		}

		// WhatsApp users often reply with the visible label instead of its
		// number (for example, "resumir una lista"). Treat that reply as a
		// natural-language menu selection while keeping the quoted menu as the
		// authoritative context.
		return true, h.handleNaturalLanguage(ctx, chatID, userID, "", selection, false)
	}

	if strings.Contains(quotedLower, "¿qué gasto querés dividir?") {
		n, err := strconv.Atoi(selection)
		if err != nil || n < 1 {
			return false, nil
		}

		expenses, err := h.db.GetGroupExpenses(ctx, chatID, 10)
		if err != nil {
			return true, h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los gastos.")
		}
		position := 0
		for _, expense := range expenses {
			if expense.IsDivided {
				continue
			}
			position++
			if position == n {
				h.conv.Clear(chatID, userID)
				return true, h.handleDivide(ctx, chatID, userID, []string{expense.ID[:8]})
			}
		}
		return true, h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Opción %d no válida. Respondé con un gasto de la lista citada.", n))
	}

	if strings.Contains(quotedLower, "¿qué deuda querés marcar como pagada?") {
		n, err := strconv.Atoi(selection)
		if err != nil || n < 1 {
			return false, nil
		}
		return true, h.handlePaymentChoice(ctx, chatID, userID, n)
	}

	if strings.Contains(quotedLower, "gasto registrado") && strings.Contains(quotedLower, "dividir entre todos") {
		if selection != "1" && !strings.Contains(strings.ToLower(selection), "dividir") {
			if selection == "2" || strings.Contains(strings.ToLower(selection), "ver gasto") {
				h.conv.Clear(chatID, userID)
				return true, h.handleViewExpenses(ctx, chatID)
			}
			return false, nil
		}
		if shortID := shortExpenseIDPattern.FindString(quotedText); shortID != "" {
			h.conv.Clear(chatID, userID)
			return true, h.handleDivide(ctx, chatID, userID, []string{shortID})
		}
	}

	return false, nil
}
