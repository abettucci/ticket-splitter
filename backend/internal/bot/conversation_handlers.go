package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/abettucci/group-split-bot/internal/security"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// handleMenu muestra el menú principal con botones inline
func (h *Handler) handleMenu(ctx context.Context, chatID int64) error {
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
				{Text: "💡 Simplificar", CallbackData: "menu:simplificar"},
			},
			{
				{Text: "👥 Miembros", CallbackData: "menu:miembros"},
				{Text: "❓ Ayuda", CallbackData: "menu:ayuda"},
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
		return h.handleMenuDivide(ctx, chatID)
	case "simplificar":
		return h.handleSimplify(ctx, chatID)
	case "miembros":
		return h.handleMembers(ctx, chatID)
	case "ayuda":
		return h.handleHelp(ctx, chatID)
	default:
		return nil
	}
}

// handleMenuDivide muestra los gastos pendientes de dividir como botones clickeables
func (h *Handler) handleMenuDivide(ctx context.Context, chatID int64) error {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 10)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los gastos.")
	}

	var rows [][]telegram.InlineKeyboardButton
	for _, exp := range expenses {
		if !exp.IsDivided {
			shortID := exp.ID[:8]
			label := fmt.Sprintf("📝 %s — %s", exp.Description, telegram.FormatMoney(exp.TotalAmount))
			if len(label) > 60 {
				label = label[:57] + "..."
			}
			rows = append(rows, []telegram.InlineKeyboardButton{
				{Text: label, CallbackData: fmt.Sprintf("divide:%s", shortID)},
			})
		}
	}

	if len(rows) == 0 {
		return h.tg.SendMessage(ctx, chatID, "✅ Todos los gastos ya fueron divididos.")
	}

	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "➗ <b>¿Qué gasto querés dividir?</b>\n\nTocá uno para dividirlo entre todos:",
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

// handleConversationStep procesa el siguiente paso de una conversación en curso
func (h *Handler) handleConversationStep(ctx context.Context, chatID, userID int64, userName, text string, state *ConversationState) error {
	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "/cancelar" || lower == "cancelar" {
		h.conv.Clear(chatID, userID)
		return h.tg.SendMessage(ctx, chatID, "❌ Operación cancelada.")
	}

	switch state.Step {
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
		ChatID:    chatID,
		Text:      fmt.Sprintf("👤 ¿Quién pagó <b>%s</b>?\n\nElegí con los botones o escribí el nombre:", telegram.EscapeHTML(description)),
		ParseMode: "HTML",
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

// handleNaturalLanguage interpreta texto libre en chats privados
func (h *Handler) handleNaturalLanguage(ctx context.Context, chatID, userID int64, userName, text string) error {
	lower := strings.ToLower(strings.TrimSpace(text))

	// Acceso por número (para WhatsApp donde no hay botones)
	menuNumbers := map[string]func() error{
		"1": func() error { return h.startExpenseFlow(ctx, chatID, userID) },
		"2": func() error { return h.handleViewExpenses(ctx, chatID) },
		"3": func() error { return h.handleMyDebts(ctx, chatID, userID) },
		"4": func() error { return h.handleBalance(ctx, chatID) },
		"5": func() error { return h.handleMenuDivide(ctx, chatID) },
		"6": func() error { return h.handleSimplify(ctx, chatID) },
		"7": func() error { return h.handleMembers(ctx, chatID) },
		"8": func() error { return h.handleHelp(ctx, chatID) },
	}
	if fn, ok := menuNumbers[lower]; ok {
		return fn()
	}

	type trigger struct {
		keywords []string
		action   func() error
	}

	triggers := []trigger{
		{
			keywords: []string{"hola", "holi", "buenas", "buenos dias", "buenos días", "buen dia", "buen día"},
			action:   func() error { return h.handleMenu(ctx, chatID) },
		},
		{
			keywords: []string{"nuevo gasto", "gasto nuevo", "agregar gasto", "anotar gasto", "cargar gasto", "cargar un gasto", "registrar gasto", "sumar gasto", "nuevo", "gasto"},
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
			action:   func() error { return h.handleMenuDivide(ctx, chatID) },
		},
		{
			keywords: []string{"ayuda", "cómo funciona", "como funciona", "qué podés hacer", "que podes hacer", "comandos"},
			action:   func() error { return h.handleHelp(ctx, chatID) },
		},
		{
			keywords: []string{"menú", "menu", "opciones", "qué puedo hacer", "que puedo hacer", "inicio"},
			action:   func() error { return h.handleMenu(ctx, chatID) },
		},
	}

	for _, t := range triggers {
		for _, kw := range t.keywords {
			if strings.Contains(lower, kw) {
				return t.action()
			}
		}
	}

	// No match - suggest the menu
	return h.tg.SendMessage(ctx, chatID, "🤖 No entendí eso. Escribí /menu o un número del 1 al 8 para ver las opciones.")
}
