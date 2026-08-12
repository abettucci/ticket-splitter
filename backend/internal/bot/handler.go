package bot

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/security"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// Messenger interfaz para enviar mensajes (Telegram o WhatsApp)
type Messenger interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
	SendMessageWithOptions(ctx context.Context, req *telegram.SendMessageRequest) error
	EditMessageText(ctx context.Context, chatID int64, messageID int64, text string) error
	AnswerCallbackQuery(ctx context.Context, callbackID string, text string) error
}

// Handler maneja los comandos del bot
type Handler struct {
	db     *db.Client
	tg     Messenger
	logger *log.Logger
	conv   *ConversationManager
}

// NewHandler crea un nuevo handler
func NewHandler(dbClient *db.Client, messenger Messenger, logger *log.Logger) *Handler {
	return &Handler{
		db:     dbClient,
		tg:     messenger,
		logger: logger,
		conv:   NewConversationManager(),
	}
}

// HandleUpdate procesa un update de Telegram
func (h *Handler) HandleUpdate(ctx context.Context, update *telegram.Update) error {
	// Manejar callbacks de inline keyboards
	if update.CallbackQuery != nil {
		return h.handleCallbackQuery(ctx, update.CallbackQuery)
	}

	// Manejar mensajes normales
	if update.Message == nil {
		return nil
	}

	msg := update.Message
	chatID := msg.Chat.ID
	userID := msg.From.ID

	// Asegurar que el usuario esté registrado como miembro
	displayName := msg.From.FirstName
	if msg.From.LastName != "" {
		displayName += " " + msg.From.LastName
	}

	// Registrar el grupo si no existe
	_, err := h.db.GetOrCreateGroup(ctx, chatID, msg.Chat.Title, msg.Chat.Type)
	if err != nil {
		h.logger.Printf("Error getting/creating group: %v", err)
	}

	// Registrar al usuario como miembro
	_, err = h.db.AddMember(ctx, chatID, userID, displayName, msg.From.Username)
	if err != nil {
		h.logger.Printf("Error adding member: %v", err)
	}

	// Procesar comandos
	text := strings.TrimSpace(msg.Text)
	if !strings.HasPrefix(text, "/") {
		// Verificar si hay una conversación activa para este usuario
		state := h.conv.Get(chatID, userID)
		if state != nil {
			return h.handleConversationStep(ctx, chatID, userID, displayName, text, state)
		}
		// En chats privados, intentar lenguaje natural
		if msg.Chat.Type == "private" {
			return h.handleNaturalLanguage(ctx, chatID, userID, displayName, text)
		}
		return nil
	}

	// Log del comando (para auditoría)
	go func() {
		_ = h.db.LogCommand(context.Background(), chatID, userID, text, nil)
	}()

	// Parsear comando
	parts := strings.Fields(text)
	command := strings.ToLower(parts[0])

	// Remover @botname del comando (Telegram lo agrega en grupos)
	// Ej: /dividir_select@group_split_bot -> /dividir_select
	if atIndex := strings.Index(command, "@"); atIndex != -1 {
		command = command[:atIndex]
	}

	args := parts[1:]

	// Log para debugging (temporal)
	h.logger.Printf("DEBUG: text='%s' command='%s' args=%v", text, command, args)

	switch command {
	case "/start":
		return h.handleStart(ctx, chatID, displayName)
	case "/menu":
		return h.handleMenu(ctx, chatID)
	case "/cancelar", "/cancel":
		h.conv.Clear(chatID, userID)
		return h.tg.SendMessage(ctx, chatID, "❌ Operación cancelada.")
	case "/test_users", "/crear_usuarios_prueba":
		return h.handleCreateTestUsers(ctx, chatID, userID, args)
	case "/limpiar_usuarios_prueba", "/clear_test_users":
		return h.handleClearTestUsers(ctx, chatID)
	case "/help":
		return h.handleHelp(ctx, chatID)
	case "/nuevo_gasto", "/newexpense":
		if len(args) == 0 {
			return h.startExpenseFlow(ctx, chatID, userID)
		}
		return h.handleNewExpense(ctx, chatID, userID, displayName, args)
	case "/ver_gastos", "/expenses":
		return h.handleViewExpenses(ctx, chatID)
	case "/dividir", "/split":
		return h.handleDivide(ctx, chatID, userID, args)
	case "/dividir_custom", "/split_custom":
		return h.handleDivideCustom(ctx, chatID, userID, args)
	case "/dividir_items", "/split_items":
		return h.handleDivideItems(ctx, chatID, userID, args)
	case "/entre", "/dividir_entre", "/dividir_select", "/split_select":
		return h.handleDivideSelect(ctx, chatID, userID, args)
	case "/redividir", "/redivide":
		return h.handleRedivide(ctx, chatID, userID, args)
	case "/redividir_custom", "/redivide_custom":
		return h.handleRedivideCustom(ctx, chatID, userID, args)
	case "/cambiar_pagador", "/change_payer":
		return h.handleChangePayer(ctx, chatID, userID, args)
	case "/editar", "/edit":
		return h.handleEdit(ctx, chatID, userID, args)
	case "/eliminar", "/delete":
		return h.handleDelete(ctx, chatID, userID, args)
	case "/papelera", "/trash":
		return h.handleTrash(ctx, chatID)
	case "/restaurar", "/restore":
		return h.handleRestore(ctx, chatID, userID, args)
	case "/simplificar", "/simplify":
		return h.handleSimplify(ctx, chatID)
	case "/mis_deudas", "/debts":
		return h.handleMyDebts(ctx, chatID, userID)
	case "/pagar", "/pay":
		return h.handlePay(ctx, chatID, userID, args)
	case "/miembros", "/members":
		return h.handleMembers(ctx, chatID)
	case "/balance":
		return h.handleBalance(ctx, chatID)
	case "/categorias", "/categories":
		return h.handleCategories(ctx, chatID)
	case "/recordar_pago", "/remind_payment":
		return h.handleCreateReminder(ctx, chatID, userID, args)
	case "/mis_recordatorios", "/my_reminders":
		return h.handleMyReminders(ctx, chatID, userID)
	case "/cancelar_recordatorio", "/cancel_reminder":
		return h.handleCancelReminder(ctx, chatID, userID, args)
	case "/recordar_deudas", "/remind_debtors":
		return h.handleRemindDebtors(ctx, chatID, userID)
	case "/calendario_pagos", "/payment_calendar":
		return h.handlePaymentCalendar(ctx, chatID, userID)
	case "/historial", "/history":
		return h.handleChangelog(ctx, chatID, args)
	default:
		return h.tg.SendMessage(ctx, chatID, "❓ Comando no reconocido. Usa /help para ver los comandos disponibles.")
	}
}

// handleCallbackQuery procesa callback queries de inline keyboards
func (h *Handler) handleCallbackQuery(ctx context.Context, query *telegram.CallbackQuery) error {
	// Validar estructura
	if query == nil || query.Message == nil {
		return nil
	}

	chatID := query.Message.Chat.ID
	userID := query.From.ID

	// Parse "action:shortID"
	parts := strings.SplitN(query.Data, ":", 2)
	if len(parts) != 2 {
		return h.tg.AnswerCallbackQuery(ctx, query.ID, "❌ Formato inválido")
	}

	action, shortID := parts[0], parts[1]

	// Registrar el grupo si no existe
	_, err := h.db.GetOrCreateGroup(ctx, chatID, query.Message.Chat.Title, query.Message.Chat.Type)
	if err != nil {
		h.logger.Printf("Error getting/creating group: %v", err)
	}

	// Registrar al usuario como miembro
	displayName := query.From.FirstName
	if query.From.LastName != "" {
		displayName += " " + query.From.LastName
	}
	_, err = h.db.AddMember(ctx, chatID, userID, displayName, query.From.Username)
	if err != nil {
		h.logger.Printf("Error adding member: %v", err)
	}

	// Log del callback (para auditoría)
	go func() {
		_ = h.db.LogCommand(context.Background(), chatID, userID, fmt.Sprintf("callback:%s", query.Data), nil)
	}()

	// Enrutar a handler apropiado
	switch action {
	case "delete_confirm":
		return h.handleDeleteConfirmation(ctx, chatID, userID, shortID, query.ID, query.Message.MessageID)
	case "delete_cancel":
		return h.handleDeleteCancellation(ctx, chatID, shortID, query.ID, query.Message.MessageID)
	case "menu":
		return h.handleMenuCallback(ctx, chatID, userID, displayName, shortID, query.ID)
	case "divide":
		_ = h.tg.AnswerCallbackQuery(ctx, query.ID, "Dividiendo...")
		return h.handleDivide(ctx, chatID, userID, []string{shortID})
	case "conv_payer":
		return h.handlePayerCallback(ctx, chatID, userID, displayName, shortID, query.ID)
	default:
		return h.tg.AnswerCallbackQuery(ctx, query.ID, "❌ Acción desconocida")
	}
}

// handleDeleteConfirmation ejecuta la eliminación del gasto después de que el usuario confirma
func (h *Handler) handleDeleteConfirmation(ctx context.Context, chatID, userID int64, shortID, callbackID string, messageID int64) error {
	// Feedback inmediato
	err := h.tg.AnswerCallbackQuery(ctx, callbackID, "Eliminando...")
	if err != nil {
		h.logger.Printf("Error answering callback query: %v", err)
	}

	// Verificar que el gasto existe
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.EditMessageText(ctx, chatID, messageID, "❌ Error: El gasto ya no existe o fue eliminado.")
	}

	// Ejecutar soft delete
	err = h.db.DeleteExpense(ctx, expense.ID, expense.PK, expense.SK, userID)
	if err != nil {
		h.logger.Printf("Error deleting expense: %v", err)
		return h.tg.EditMessageText(ctx, chatID, messageID, "❌ Error al eliminar el gasto. Intenta nuevamente.")
	}

	h.logChangelog(chatID, userID, "", "expense", shortID, expense.Description, "deleted", map[string]string{
		"monto": telegram.FormatMoney(expense.TotalAmount),
	})

	// Editar mensaje mostrando éxito
	successMsg := fmt.Sprintf(`✅ <b>Gasto eliminado</b>

📝 %s
💰 %s
👤 Creado por: %s

<i>El gasto estará en la papelera por 30 días.</i>
Usa /restaurar %s para recuperarlo.`,
		telegram.EscapeHTML(expense.Description),
		telegram.FormatMoney(expense.TotalAmount),
		telegram.EscapeHTML(expense.CreatorName),
		shortID)

	return h.tg.EditMessageText(ctx, chatID, messageID, successMsg)
}

// handleDeleteCancellation maneja cuando el usuario cancela la eliminación
func (h *Handler) handleDeleteCancellation(ctx context.Context, chatID int64, shortID, callbackID string, messageID int64) error {
	err := h.tg.AnswerCallbackQuery(ctx, callbackID, "Operación cancelada")
	if err != nil {
		h.logger.Printf("Error answering callback query: %v", err)
	}

	cancelMsg := fmt.Sprintf(`❌ <b>Eliminación cancelada</b>

El gasto con ID <code>%s</code> no fue eliminado.

Usa /ver_gastos para ver tus gastos.`, shortID)

	return h.tg.EditMessageText(ctx, chatID, messageID, cancelMsg)
}

// handleStart maneja el comando /start
func (h *Handler) handleStart(ctx context.Context, chatID int64, userName string) error {
	escapedName := telegram.EscapeHTML(userName)

	message := fmt.Sprintf(`👋 ¡Hola %s! Bienvenido a <b>SplitBot</b>

🤖 Soy un bot que te ayuda a dividir gastos grupales de forma simple y segura.

<b>¿Cómo empezar?</b>
1️⃣ Agrega este bot a tu grupo
2️⃣ Todos los miembros envían al menos un mensaje
3️⃣ Registra gastos tocando el botón o con /nuevo_gasto
4️⃣ Divide con un click desde el menú

💡 <b>Tip</b>: Usá /menu para ver todas las opciones con botones.

🔒 <b>Seguridad</b>: Tus datos están encriptados y nunca compartimos información personal.`, escapedName)

	if err := h.tg.SendMessage(ctx, chatID, message); err != nil {
		return err
	}
	return h.handleMenu(ctx, chatID)
}

// handleHelp maneja el comando /help
func (h *Handler) handleHelp(ctx context.Context, chatID int64) error {
	message := `🤖 <b>Comandos de SplitBot</b>

📝 <b>Gestión de Gastos</b>
• /nuevo_gasto [desc] [monto] [usuario] - Crear gasto
  <i>Opcional: especifica quién pagó con @usuario o nombre completo</i>
• /ver_gastos - Ver últimos gastos
• /editar [id] [campo] [valor] - Editar gasto
• /eliminar [id] - Eliminar gasto (va a papelera)
• /papelera - Ver gastos eliminados
• /restaurar [id] - Recuperar gasto eliminado

💰 <b>División de Gastos</b>
• /dividir [id] - División equitativa entre todos
• /dividir_entre [id] @user1 @user2 - Entre usuarios específicos
• /dividir_custom [id] @user1 500 @user2 300 - Montos custom
• /dividir_items [id] - Por items
• /redividir [id] [@users] - Cambiar participantes
• /redividir_custom [id] @user1 monto1 @user2 monto2 - Redividir con montos custom
• /cambiar_pagador [id] @user - Cambiar quién pagó

💳 <b>Deudas y Pagos</b>
• /mis_deudas - Ver deudas pendientes
• /pagar [id] - Marcar como pagado
• /balance - Balance del grupo
• /simplificar - Optimizar deudas

👥 <b>Grupo</b>
• /miembros - Ver miembros
• /categorias - Ver categorías

🔔 <b>Recordatorios y Calendario</b>
• /recordar_pago [desc] [monto] [deudor] [acreedor] [cuotas] [frec] [fecha]
  <i>Yo pago: /recordar_pago Netflix 1500 yo Juan 1 monthly</i>
  <i>Me pagan: /recordar_pago Cuota ACDC 5000 Ana yo 6 monthly</i>
• /calendario_pagos - Ver tu calendario de pagos futuros
• /mis_recordatorios - Ver tus recordatorios activos
• /cancelar_recordatorio [id] - Cancelar recordatorio
• /recordar_deudas - Recordar a deudores del grupo

🧪 <b>Testing</b>
• /crear_usuarios_prueba [nombre1] [nombre2]... - Crear usuarios personalizados
  <i>Ej: /crear_usuarios_prueba Juan María Pedro</i>
• /crear_usuarios_prueba default - Crear 5 usuarios por defecto
• /limpiar_usuarios_prueba - Eliminar todos los usuarios de prueba

ℹ️ <b>Info</b>
• /start - Bienvenida
• /help - Esta ayuda

💡 Tip: Usa los primeros 6-8 caracteres del ID.`

	return h.tg.SendMessage(ctx, chatID, message)
}

// handleNewExpense maneja el comando /nuevo_gasto
func (h *Handler) handleNewExpense(ctx context.Context, chatID, userID int64, userName string, args []string) error {
	if len(args) < 2 {
		return h.tg.SendMessage(ctx, chatID, `❌ <b>Formato incorrecto</b>

Uso: /nuevo_gasto [descripción] [monto] [usuario]

Ejemplos:
• /nuevo_gasto Cena 15000
• /nuevo_gasto Super mercado 8500.50
• /nuevo_gasto Nafta viaje 12000 @juan
• /nuevo_gasto Hielo 8000 juan garcia

<i>Nota: Si especificas el usuario, ese será quien pagó el gasto. Podés usar @ o el nombre completo.</i>`)
	}

	// Buscar el monto en los argumentos (puede ser un número con o sin decimales)
	var payerUserID int64 = userID
	var payerUserName string = userName
	var amountStr string
	var descriptionArgs []string
	var payerNameArgs []string
	amountIndex := -1

	// Buscar dónde está el monto (primer número válido después de la descripción)
	for i := 1; i < len(args); i++ {
		testAmount := strings.ReplaceAll(args[i], ",", ".")
		if _, err := strconv.ParseFloat(testAmount, 64); err == nil {
			amountIndex = i
			amountStr = testAmount
			break
		}
	}

	if amountIndex == -1 {
		return h.tg.SendMessage(ctx, chatID, "❌ El monto debe ser un número válido.\n\nEjemplo: /nuevo_gasto Cena 15000")
	}

	// Descripción: todo antes del monto
	descriptionArgs = args[:amountIndex]

	// Pagador: todo después del monto (si hay algo)
	if amountIndex < len(args)-1 {
		payerNameArgs = args[amountIndex+1:]
		payerNameStr := strings.Join(payerNameArgs, " ")

		// Limpiar @ si está presente
		payerNameStr = strings.TrimPrefix(payerNameStr, "@")
		payerNameStr = strings.TrimSpace(payerNameStr)

		// Buscar el usuario en los miembros del grupo
		members, err := h.db.GetGroupMembers(ctx, chatID)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los miembros del grupo.")
		}

		found := false
		// Buscar por username primero
		for _, member := range members {
			if member.Username != "" && strings.EqualFold(member.Username, payerNameStr) {
				payerUserID = member.UserID
				payerUserName = member.DisplayName
				found = true
				break
			}
		}

		// Si no se encontró por username, buscar por display name (nombre completo)
		if !found {
			for _, member := range members {
				if strings.EqualFold(member.DisplayName, payerNameStr) {
					payerUserID = member.UserID
					payerUserName = member.DisplayName
					found = true
					break
				}
			}
		}

		// Si aún no se encontró, buscar por coincidencia parcial (nombre)
		if !found {
			payerNameLower := strings.ToLower(payerNameStr)
			for _, member := range members {
				displayNameLower := strings.ToLower(member.DisplayName)
				if strings.Contains(displayNameLower, payerNameLower) ||
					strings.Contains(payerNameLower, displayNameLower) {
					payerUserID = member.UserID
					payerUserName = member.DisplayName
					found = true
					break
				}
			}
		}

		if !found {
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ Usuario '%s' no encontrado en el grupo.\n\n💡 Tip: Asegúrate de que haya enviado al menos un mensaje al grupo.", telegram.EscapeHTML(payerNameStr)))
		}
	}

	description := strings.Join(descriptionArgs, " ")

	// Validar descripción
	description, valid := security.ValidateDescription(description)
	if !valid {
		return h.tg.SendMessage(ctx, chatID, "❌ La descripción contiene caracteres no permitidos.")
	}

	// Parsear monto
	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ El monto debe ser un número válido.\n\nEjemplo: /nuevo_gasto Cena 15000")
	}

	// Validar monto
	if !security.ValidateAmount(amount) {
		return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("❌ El monto debe estar entre %s y %s", telegram.FormatMoney(security.MinAmountValue), telegram.FormatMoney(security.MaxAmountValue)))
	}

	// Crear gasto con el pagador correcto
	expense, err := h.db.CreateExpense(ctx, chatID, description, amount, payerUserID, payerUserName)
	if err != nil {
		h.logger.Printf("Error creating expense: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al crear el gasto. Intenta nuevamente.")
	}

	shortID := expense.ID[:8]
	h.logChangelog(chatID, userID, userName, "expense", shortID, description, "created", map[string]string{
		"monto":  telegram.FormatMoney(amount),
		"pagado por": payerUserName,
	})

	// Mensaje diferente si lo cargó otra persona
	var registeredByMsg string
	if payerUserID != userID {
		registeredByMsg = fmt.Sprintf("\n📝 Registrado por: %s", userName)
	}

	message := fmt.Sprintf(`✅ <b>Gasto registrado</b>

📝 <b>%s</b>
💰 Monto: %s
👤 Pagado por: %s%s
🆔 ID: %s

Para dividirlo entre los miembros usa:
/dividir %s`, telegram.EscapeHTML(description), telegram.FormatMoney(amount), payerUserName, registeredByMsg, shortID, shortID)

	return h.tg.SendMessage(ctx, chatID, message)
}

// handleViewExpenses maneja el comando /ver_gastos
func (h *Handler) handleViewExpenses(ctx context.Context, chatID int64) error {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 10)
	if err != nil {
		h.logger.Printf("Error getting expenses: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los gastos.")
	}

	if len(expenses) == 0 {
		return h.tg.SendMessage(ctx, chatID, "📋 No hay gastos registrados en este grupo.\n\nUsa /nuevo_gasto para crear uno.")
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Últimos gastos del grupo</b>\n\n")

	for i, expense := range expenses {
		shortID := expense.ID[:8]
		status := "⏳"
		if expense.IsDivided {
			status = "✅"
		}

		sb.WriteString(fmt.Sprintf("%d. %s <b>%s</b>\n", i+1, status, expense.Description))
		sb.WriteString(fmt.Sprintf("   💰 %s | 👤 %s\n", telegram.FormatMoney(expense.TotalAmount), expense.CreatorName))
		sb.WriteString(fmt.Sprintf("   🆔 <code>%s</code> | 📅 %s\n\n", shortID, expense.CreatedAt.Format("02/01")))
	}

	sb.WriteString("_✅ = Dividido | ⏳ = Pendiente_")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleDivide maneja el comando /dividir
func (h *Handler) handleDivide(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, `❌ *Formato incorrecto*

Uso: /dividir [id_gasto]

Ejemplo: /dividir a1b2c3

Tip: Usa /ver_gastos para ver los IDs de los gastos.`)
	}

	shortID := args[0]

	// Buscar el gasto
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado. Verifica el ID con /ver_gastos")
	}

	if expense.IsDivided {
		return h.tg.SendMessage(ctx, chatID, "⚠️ Este gasto ya fue dividido.")
	}

	// Obtener miembros del grupo
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil || len(members) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No hay miembros registrados en el grupo.\n\nCada persona debe enviar al menos un mensaje al grupo para registrarse.")
	}

	// Crear las divisiones
	err = h.db.CreateSplits(ctx, expense, members)
	if err != nil {
		h.logger.Printf("Error creating splits: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al dividir el gasto.")
	}

	amountPerPerson := expense.TotalAmount / float64(len(members))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("💰 <b>Gasto dividido: %s</b>\n\n", expense.Description))
	sb.WriteString(fmt.Sprintf("📊 Total: %s\n", telegram.FormatMoney(expense.TotalAmount)))
	sb.WriteString(fmt.Sprintf("👥 Participantes: %d\n", len(members)))
	sb.WriteString(fmt.Sprintf("💵 Por persona: <b>%s</b>\n\n", telegram.FormatMoney(amountPerPerson)))
	sb.WriteString("<b>Estado de pagos:</b>\n")

	for _, member := range members {
		status := "⏳ Pendiente"
		if member.UserID == expense.CreatedBy {
			status = "✅ Pagó (creador)"
		}
		sb.WriteString(fmt.Sprintf("• %s: %s\n", member.DisplayName, status))
	}

	sb.WriteString(fmt.Sprintf("\n_Usa /pagar %s para marcar tu pago_", shortID))

	h.logChangelog(chatID, userID, "", "expense", shortID, expense.Description, "divided", map[string]string{
		"participantes": fmt.Sprintf("%d", len(members)),
		"por persona":   telegram.FormatMoney(amountPerPerson),
	})

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleMyDebts maneja el comando /mis_deudas
func (h *Handler) handleMyDebts(ctx context.Context, chatID, userID int64) error {
	splits, err := h.db.GetUserPendingSplits(ctx, userID)
	if err != nil {
		h.logger.Printf("Error getting user splits: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener tus deudas.")
	}

	if len(splits) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No tienes deudas pendientes!")
	}

	var sb strings.Builder
	sb.WriteString("💳 <b>Tus deudas pendientes</b>\n\n")

	var total float64
	var validSplitsCount int
	var orphanedSplits []string // IDs de splits huérfanos para limpiar

	for _, split := range splits {
		// Verificar que el gasto padre no esté eliminado
		shortID := split.ExpenseID[:8]
		expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
		if err != nil {
			// El gasto fue eliminado, marcar split como huérfano para limpiar
			orphanedSplits = append(orphanedSplits, split.ExpenseID)
			continue
		}

		sb.WriteString(fmt.Sprintf("• %s (%s) - ID: <code>%s</code>\n", telegram.FormatMoney(split.Amount), telegram.EscapeHTML(expense.Description), shortID))
		total += split.Amount
		validSplitsCount++
	}

	// Limpiar splits huérfanos en background (gastos eliminados)
	if len(orphanedSplits) > 0 {
		go func() {
			for _, expenseID := range orphanedSplits {
				if err := h.db.DeleteExpenseSplits(context.Background(), expenseID); err != nil {
					h.logger.Printf("Error cleaning orphaned splits for expense %s: %v", expenseID, err)
				}
			}
		}()
	}

	if validSplitsCount == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No tienes deudas pendientes!")
	}

	sb.WriteString(fmt.Sprintf("\n💰 <b>Total adeudado: %s</b>", telegram.FormatMoney(total)))
	sb.WriteString("\n\n_Usa /pagar [id] para marcar como pagado_")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handlePay maneja el comando /pagar
func (h *Handler) handlePay(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.tg.SendMessage(ctx, chatID, "❌ Uso: /pagar [id_gasto]\n\nEjemplo: /pagar a1b2c3")
	}

	shortID := args[0]

	// Buscar el gasto
	expense, err := h.db.GetExpenseByShortID(ctx, chatID, shortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Gasto no encontrado.")
	}

	// Marcar como pagado
	err = h.db.MarkSplitAsPaid(ctx, expense.ID, userID)
	if err != nil {
		h.logger.Printf("Error marking split as paid: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al marcar el pago. ¿Ya estaba pagado?")
	}

	h.logChangelog(chatID, userID, "", "expense", shortID, expense.Description, "paid", nil)

	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ ¡Pago registrado!\n\nTu parte del gasto <b>%s</b> ha sido marcada como pagada.", expense.Description))
}

// handleMembers maneja el comando /miembros
func (h *Handler) handleMembers(ctx context.Context, chatID int64) error {
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error getting members: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener los miembros.")
	}

	if len(members) == 0 {
		return h.tg.SendMessage(ctx, chatID, "👥 No hay miembros registrados.\n\nCada persona debe enviar al menos un mensaje para registrarse.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👥 <b>Miembros del grupo</b> (%d)\n\n", len(members)))

	for i, member := range members {
		username := ""
		if member.Username != "" {
			username = fmt.Sprintf(" (@%s)", member.Username)
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, member.DisplayName, username))
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// isQuoteChar verifica si un caracter es una comilla (normal o tipográfica)
func isQuoteChar(r rune) bool {
	return r == '"' || r == '\u201C' || r == '\u201D' // " " "
}

// parseQuotedArgs parsea argumentos respetando comillas para nombres compuestos
// Ejemplo: "Ana Kozameh", "Flor Menconi" -> ["Ana Kozameh", "Flor Menconi"]
func parseQuotedArgs(input string) []string {
	var result []string
	var current strings.Builder
	inQuotes := false

	for _, r := range input {
		switch {
		case isQuoteChar(r) && !inQuotes:
			// Inicio de comillas
			inQuotes = true
		case isQuoteChar(r) && inQuotes:
			// Fin de comillas
			inQuotes = false
			if current.Len() > 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
		case r == ',' && !inQuotes:
			// Separador de coma fuera de comillas
			if current.Len() > 0 {
				result = append(result, strings.TrimSpace(current.String()))
				current.Reset()
			}
		case r == ' ' && !inQuotes && current.Len() == 0:
			// Ignorar espacios iniciales fuera de comillas
			continue
		default:
			current.WriteRune(r)
		}
	}

	// Agregar el último argumento si existe
	if current.Len() > 0 {
		result = append(result, strings.TrimSpace(current.String()))
	}

	// Limpiar comillas residuales de cada resultado
	for i, s := range result {
		s = strings.Trim(s, "\"\u201C\u201D")
		s = strings.TrimSpace(s)
		result[i] = s
	}

	return result
}

// handleCreateTestUsers crea usuarios de prueba para testing
func (h *Handler) handleCreateTestUsers(ctx context.Context, chatID, userID int64, args []string) error {
	// Nombres por defecto si no se especifican
	defaultNames := []string{"Juan Pérez", "María García", "Pedro López", "Ana Martínez", "Carlos Rodríguez"}

	var names []string

	if len(args) == 0 {
		// Sin argumentos, usar nombres por defecto
		return h.tg.SendMessage(ctx, chatID, `👥 <b>Crear usuarios de prueba</b>

<b>Uso:</b>
/crear_usuarios_prueba [nombre1] [nombre2] [nombre3]...

<b>Ejemplos:</b>
• /crear_usuarios_prueba Juan María Pedro
  <i>(Crea 3 usuarios: Juan, María y Pedro)</i>

• /crear_usuarios_prueba "Ana Kozameh", "Flor Menconi", "Pauli Taffarel"
  <i>(Crea 3 usuarios con nombres completos)</i>

• /crear_usuarios_prueba default
  <i>(Crea 5 usuarios con nombres por defecto)</i>

<b>Nombres por defecto:</b>
• Juan Pérez
• María García
• Pedro López
• Ana Martínez
• Carlos Rodríguez

💡 Tip: Usa comillas y comas para nombres compuestos`)
	}

	// Caso especial: "default" crea usuarios por defecto
	if len(args) == 1 && strings.ToLower(args[0]) == "default" {
		names = defaultNames
	} else {
		// Reconstruir el string original para parsear comillas
		fullInput := strings.Join(args, " ")

		// Si contiene comillas, parsear respetando comillas
		if strings.ContainsAny(fullInput, "\"\"\"") {
			names = parseQuotedArgs(fullInput)
		} else {
			// Sin comillas, usar los argumentos directamente
			names = args
		}
	}

	// Limitar a un máximo razonable
	if len(names) > 20 {
		return h.tg.SendMessage(ctx, chatID, "❌ Máximo 20 usuarios de prueba permitidos.")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("👥 <b>Usuarios de prueba creados:</b> (%d)\n\n", len(names)))

	var createdUsernames []string

	for i, name := range names {
		// Generar ID único basado en el índice
		testUserID := int64(999000001 + i)

		// Generar username normalizado (sin espacios, acentos, minúsculas)
		username := normalizeUsername(name)
		displayName := name

		_, err := h.db.AddMember(ctx, chatID, testUserID, displayName, username)
		if err != nil {
			h.logger.Printf("Error adding test user %s: %v", displayName, err)
			sb.WriteString(fmt.Sprintf("⚠️ %s (@%s) - Error al crear\n", displayName, username))
			continue
		}
		sb.WriteString(fmt.Sprintf("✅ %s (@%s)\n", telegram.EscapeHTML(displayName), username))
		createdUsernames = append(createdUsernames, username)
	}

	if len(createdUsernames) > 0 {
		sb.WriteString("\n<i>Ahora puedes usar estos usuarios en comandos:</i>\n")
		sb.WriteString(fmt.Sprintf("• /nuevo_gasto Cena 15000 @%s\n", createdUsernames[0]))
		if len(createdUsernames) > 1 {
			sb.WriteString(fmt.Sprintf("• /dividir_select [id] @%s @%s\n", createdUsernames[0], createdUsernames[1]))
		}
		if len(createdUsernames) > 2 {
			sb.WriteString(fmt.Sprintf("• /dividir_custom [id] @%s 5000 @%s 10000\n", createdUsernames[0], createdUsernames[1]))
		}
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// normalizeUsername convierte un nombre en un username válido
func normalizeUsername(name string) string {
	// Convertir a minúsculas
	username := strings.ToLower(name)

	// Remover acentos y caracteres especiales
	replacements := map[string]string{
		"á": "a", "é": "e", "í": "i", "ó": "o", "ú": "u",
		"ñ": "n", "ü": "u",
		" ": "_", ".": "", ",": "", "-": "_",
		"'": "", "\"": "",
	}

	for old, new := range replacements {
		username = strings.ReplaceAll(username, old, new)
	}

	// Remover caracteres no alfanuméricos (excepto _)
	var result strings.Builder
	for _, r := range username {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			result.WriteRune(r)
		}
	}

	username = result.String()

	// Truncar si es muy largo
	if len(username) > 20 {
		username = username[:20]
	}

	// Asegurar que no esté vacío
	if username == "" {
		username = "user_test"
	}

	return username + "_test"
}

// handleClearTestUsers elimina los usuarios de prueba
func (h *Handler) handleClearTestUsers(ctx context.Context, chatID int64) error {
	// Eliminar usuarios de prueba del rango 999000001 a 999000050
	// (suficiente para cubrir cualquier cantidad creada con /crear_usuarios_prueba)
	deletedCount := 0
	var deletedNames []string

	for i := 1; i <= 50; i++ {
		testUserID := int64(999000000 + i)

		// Obtener información del miembro antes de eliminarlo (para mostrar el nombre)
		members, err := h.db.GetGroupMembers(ctx, chatID)
		if err == nil {
			for _, member := range members {
				if member.UserID == testUserID {
					deletedNames = append(deletedNames, member.DisplayName)
					break
				}
			}
		}

		err = h.db.RemoveMember(ctx, chatID, testUserID)
		if err == nil {
			deletedCount++
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🗑️ <b>Usuarios de prueba eliminados:</b> %d\n\n", deletedCount))

	if len(deletedNames) > 0 {
		sb.WriteString("<b>Usuarios eliminados:</b>\n")
		for i, name := range deletedNames {
			if i < 10 { // Mostrar máximo 10
				sb.WriteString(fmt.Sprintf("• %s\n", telegram.EscapeHTML(name)))
			}
		}
		if len(deletedNames) > 10 {
			sb.WriteString(fmt.Sprintf("• ... y %d más\n", len(deletedNames)-10))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("Usa /miembros para verificar.")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleBalance maneja el comando /balance
func (h *Handler) handleBalance(ctx context.Context, chatID int64) error {
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener el balance.")
	}

	if len(members) == 0 {
		return h.tg.SendMessage(ctx, chatID, "❌ No hay miembros en el grupo.")
	}

	// Crear mapa de UserID -> DisplayName para lookups rápidos
	memberNames := make(map[int64]string)
	for _, member := range members {
		memberNames[member.UserID] = member.DisplayName
	}

	var sb strings.Builder
	sb.WriteString("📊 <b>Balance del grupo</b>\n\n")

	for _, member := range members {
		splits, _ := h.db.GetUserPendingSplits(ctx, member.UserID)

		// Agrupar deudas por acreedor
		debtsPerCreditor := make(map[int64]float64)
		var totalDebt float64

		for _, split := range splits {
			// Obtener el gasto para saber quién pagó
			expense, err := h.db.GetExpenseByShortID(ctx, chatID, split.ExpenseID)
			if err != nil {
				continue
			}

			// Determinar quién pagó
			creditorID := expense.CreatedBy
			if len(expense.Payers) > 0 {
				// Si hay múltiples pagadores, asignar proporcionalmente
				for _, payer := range expense.Payers {
					payerShare := (payer.Amount / expense.TotalAmount) * split.Amount
					debtsPerCreditor[payer.UserID] += payerShare
					totalDebt += payerShare
				}
			} else {
				debtsPerCreditor[creditorID] += split.Amount
				totalDebt += split.Amount
			}
		}

		status := "✅"
		if totalDebt > 0 {
			status = "⏳"
		}

		sb.WriteString(fmt.Sprintf("%s <b>%s</b>: ", status, telegram.EscapeHTML(member.DisplayName)))
		if totalDebt > 0 {
			sb.WriteString(fmt.Sprintf("Debe %s\n", telegram.FormatMoney(totalDebt)))

			// Mostrar desglose por acreedor
			for creditorID, amount := range debtsPerCreditor {
				if amount > 0.01 { // Evitar mostrar montos insignificantes
					creditorName := memberNames[creditorID]
					if creditorName == "" {
						creditorName = fmt.Sprintf("Usuario %d", creditorID)
					}
					sb.WriteString(fmt.Sprintf("   → %s: %s\n", telegram.EscapeHTML(creditorName), telegram.FormatMoney(amount)))
				}
			}
		} else {
			sb.WriteString("Sin deudas\n")
		}
		sb.WriteString("\n")
	}

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleChangelog muestra el historial de cambios del grupo o de un gasto específico
func (h *Handler) handleChangelog(ctx context.Context, chatID int64, args []string) error {
	actionEmoji := map[string]string{
		"created":  "✅",
		"updated":  "✏️",
		"deleted":  "🗑️",
		"restored": "♻️",
		"paid":     "💸",
		"divided":  "➗",
		"added":    "👤",
		"removed":  "👤",
	}

	if len(args) > 0 {
		entityID := args[0]
		entries, err := h.db.GetEntityChangelog(ctx, chatID, entityID)
		if err != nil {
			h.logger.Printf("Error getting entity changelog: %v", err)
			return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener el historial.")
		}
		if len(entries) == 0 {
			return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("📋 No hay historial para el ID <code>%s</code>.", telegram.EscapeHTML(entityID)))
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("📋 <b>Historial de %s</b>\n\n", telegram.EscapeHTML(entries[0].EntityName)))
		for _, e := range entries {
			emoji := actionEmoji[e.Action]
			if emoji == "" {
				emoji = "🔹"
			}
			sb.WriteString(fmt.Sprintf("%s <b>%s</b> — %s\n", emoji, e.Action, e.ChangedByName))
			for campo, cambio := range e.Changes {
				sb.WriteString(fmt.Sprintf("   • %s: %s\n", campo, cambio))
			}
			sb.WriteString(fmt.Sprintf("   <i>%s</i>\n\n", e.CreatedAt.Format("02/01/2006 15:04")))
		}
		return h.tg.SendMessage(ctx, chatID, sb.String())
	}

	entries, err := h.db.GetGroupChangelog(ctx, chatID, 15)
	if err != nil {
		h.logger.Printf("Error getting group changelog: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener el historial.")
	}
	if len(entries) == 0 {
		return h.tg.SendMessage(ctx, chatID, "📋 No hay cambios registrados en este grupo todavía.")
	}

	var sb strings.Builder
	sb.WriteString("📋 <b>Últimos cambios del grupo</b>\n\n")
	for _, e := range entries {
		emoji := actionEmoji[e.Action]
		if emoji == "" {
			emoji = "🔹"
		}
		sb.WriteString(fmt.Sprintf("%s <b>%s</b> %s <i>%s</i>\n", emoji, e.ChangedByName, e.Action, e.EntityName))
		sb.WriteString(fmt.Sprintf("   <i>%s</i>\n\n", e.CreatedAt.Format("02/01/2006 15:04")))
	}
	sb.WriteString("<i>Usá /historial [id] para ver el detalle de un gasto específico</i>")
	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// logChangelog guarda un changelog de forma asíncrona sin bloquear el flujo principal
func (h *Handler) logChangelog(chatID, userID int64, userName, entityType, entityID, entityName, action string, changes map[string]string) {
	go func() {
		_ = h.db.SaveChangelog(context.Background(), &db.ChangelogEntry{
			ChatID:        chatID,
			EntityType:    entityType,
			EntityID:      entityID,
			EntityName:    entityName,
			Action:        action,
			ChangedByID:   userID,
			ChangedByName: userName,
			Changes:       changes,
		})
	}()
}
