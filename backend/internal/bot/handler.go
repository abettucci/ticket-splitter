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
	db              *db.Client
	tg              Messenger
	logger          *log.Logger
	conv            *ConversationManager
	reminderChannel string
}

// NewHandler crea un nuevo handler
func NewHandler(dbClient *db.Client, messenger Messenger, logger *log.Logger, reminderChannel string) *Handler {
	if reminderChannel == "" {
		reminderChannel = db.ReminderChannelTelegram
	}

	return &Handler{
		db:              dbClient,
		tg:              messenger,
		logger:          logger,
		conv:            NewConversationManager(),
		reminderChannel: reminderChannel,
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

	// WhatsApp lists/buttons carry an opaque action ID. Process it before the
	// text parser so only an actual interactive response can invoke callbacks.
	if msg.InteractiveID != "" {
		return h.handleInteractiveSelection(ctx, chatID, userID, displayName, msg.InteractiveID)
	}

	// Procesar comandos
	text := strings.TrimSpace(msg.Text)
	// En WhatsApp es natural escribir "@Splitter /pagar ...". Normalizamos
	// ese formato antes de decidir si se trata de un comando, sin afectar las
	// menciones que son texto normal.
	if fields := strings.Fields(text); len(fields) > 1 && strings.HasPrefix(fields[0], "@") && strings.HasPrefix(fields[1], "/") {
		text = strings.Join(fields[1:], " ")
	}
	// Fetch the current state before applying the WhatsApp group guard. Once
	// Splitter has asked this exact person a free-text question, their next
	// answer can be plain text; other group conversation stays ignored.
	state := h.conv.Get(chatID, userID)

	// In WhatsApp groups, never treat ordinary group conversation as bot input.
	// A person must @mention Splitter, reply to one of its messages (the
	// sidecar marks that as IsMentioned), select an actual interactive action,
	// or answer a pending question for their own active flow. Telegram keeps its
	// established command behavior because it has native command routing.
	if msg.IsWhatsAppWeb && msg.Chat.Type == "group" && !msg.IsMentioned && !isSafeWhatsAppGroupContinuation(state, text) {
		h.logger.Printf("WA Web group message ignored: chat=%d sender=%d mentioned=false active_step=%d reason=not_addressed", chatID, userID, conversationStepForLog(state))
		return nil
	}
	// In WhatsApp groups, a reply can target an older bot message while a newer
	// conversation state is still active. The quoted menu is more specific than
	// that transient state, so resolve it first.
	if msg.QuotedText != "" {
		if handled, err := h.handleQuotedMenuChoice(ctx, chatID, userID, text, msg.QuotedText); handled {
			return err
		}
	}
	if !strings.HasPrefix(text, "/") {
		// Verificar si hay una conversación activa para este usuario
		if state != nil {
			return h.handleConversationStep(ctx, chatID, userID, displayName, text, state)
		}
		// In private chats, natural language is always safe. In WhatsApp groups,
		// the guard above also applies to active flows, so a user's unrelated
		// messages are never consumed as answers to a bot question.
		if msg.Chat.Type == "private" {
			return h.handleNaturalLanguage(ctx, chatID, userID, displayName, text, false)
		}
		if msg.Chat.Type == "group" && msg.IsMentioned {
			return h.handleNaturalLanguage(ctx, chatID, userID, displayName, text, true)
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
		return h.handleStart(ctx, chatID, userID, displayName)
	case "/menu":
		return h.handleMenu(ctx, chatID, userID)
	case "/cancelar", "/cancel":
		h.conv.Clear(chatID, userID)
		return h.tg.SendMessage(ctx, chatID, "❌ Operación cancelada.")
	case "/test_users", "/crear_usuarios_prueba":
		return h.handleCreateTestUsers(ctx, chatID, userID, args)
	case "/limpiar_usuarios_prueba", "/clear_test_users":
		return h.handleClearTestUsers(ctx, chatID)
	case "/simular_operaciones", "/simulate_operations":
		return h.handleRunTestSimulation(ctx, chatID, userID)
	case "/limpiar_pruebas", "/clear_test_data":
		return h.handleClearTestData(ctx, chatID, userID)
	case "/resumir_gastos", "/resumen_gastos", "/resumir_lista":
		if len(args) == 0 {
			return h.startExpenseSummary(ctx, chatID, userID)
		}
		return h.handleExpenseSummary(ctx, chatID, userID, displayName, strings.Join(args, " "))
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

// isSafeWhatsAppGroupContinuation accepts a person's next free-text answer
// only when Splitter is actively waiting for that specific field. Selectors
// remain strict, so an unrelated "1" or chat message never triggers a menu
// action just because a menu happened to be shown earlier.
func isSafeWhatsAppGroupContinuation(state *ConversationState, text string) bool {
	if state == nil {
		return false
	}

	selection := strings.TrimSpace(text)
	switch state.Step {
	case StepNewExpenseDescription, StepNewExpenseAmount, StepNewExpensePayer, StepExpenseSummary:
		return selection != ""
	case StepSelectMenuOption:
		_, ok := resolveMenuOptionText(selection)
		return ok
	case StepSelectNewExpenseAction:
		lower := strings.ToLower(selection)
		return lower == "1" || lower == "2" || strings.Contains(lower, "dividir") || strings.Contains(lower, "ver gasto")
	case StepSelectDivideExpense, StepSelectRedivideExpense, StepSelectPaymentExpense:
		n, err := strconv.Atoi(selection)
		return err == nil && n > 0
	default:
		return false
	}
}

func conversationStepForLog(state *ConversationState) ConversationStep {
	if state == nil {
		return StepNone
	}
	return state.Step
}

// handleInteractiveSelection routes WhatsApp list/button selections. Telegram
// uses CallbackQuery instead, so this never changes its existing behavior.
func (h *Handler) handleInteractiveSelection(ctx context.Context, chatID, userID int64, userName, selection string) error {
	parts := strings.SplitN(selection, ":", 2)
	if len(parts) != 2 {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa opción ya no es válida. Pedile el menú a Splitter de nuevo.")
	}

	switch parts[0] {
	case "menu":
		return h.handleMenuCallback(ctx, chatID, userID, userName, parts[1], "")
	case "divide":
		return h.handleDivide(ctx, chatID, userID, []string{parts[1]})
	case "redivide":
		return h.handleRedivide(ctx, chatID, userID, []string{parts[1]})
	case "pay":
		return h.handlePayByShortID(ctx, chatID, userID, parts[1])
	case "conv_payer":
		return h.handlePayerCallback(ctx, chatID, userID, userName, parts[1], "")
	default:
		return h.tg.SendMessage(ctx, chatID, "❌ Esa opción no está disponible. Pedile el menú a Splitter de nuevo.")
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
	case "redivide":
		_ = h.tg.AnswerCallbackQuery(ctx, query.ID, "Redividiendo...")
		return h.handleRedivide(ctx, chatID, userID, []string{shortID})
	case "pay":
		return h.handlePayByShortID(ctx, chatID, userID, shortID)
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
func (h *Handler) handleStart(ctx context.Context, chatID, userID int64, userName string) error {
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
	return h.handleMenu(ctx, chatID, userID)
}

// handleHelp maneja el comando /help
func (h *Handler) handleHelp(ctx context.Context, chatID int64) error {
	message := `🤖 <b>Guía de Splitter</b>

Para lo cotidiano, elegí una opción del menú o escribí lo que necesitás. Los atajos con “/” quedan como referencia avanzada.

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

🐀 <b>Recordatorios y Calendario</b>
• /recordar_pago [desc] [monto] [deudor] [acreedor] [cuotas] [frec] [fecha] [hora]
  <i>Yo pago: /recordar_pago Netflix 1500 yo Juan 1 monthly</i>
  <i>Me pagan: /recordar_pago Cuota ACDC 5000 Ana yo 6 monthly</i>
  <i>Con hora: /recordar_pago Regalo 30000 yo Nico 1 once 30/09/2026 18:30</i>
• /calendario_pagos - Ver tu calendario de pagos futuros
• /mis_recordatorios - Ver tus recordatorios activos
• /cancelar_recordatorio [id] - Cancelar recordatorio
• /recordar_deudas - Recordar a deudores del grupo
  <i>En WhatsApp, cada deudor debe haber enviado /start por chat privado.</i>

🧪 <b>Testing</b>
• Crear personas de prueba - Agrega 5 personas ficticias al grupo
• Simular operaciones - Genera gastos, divisiones, pagos y cambios de pagador, y valida los cálculos
• Limpiar datos de prueba - Borra sólo la simulación y esas personas ficticias

🧾 <b>Resumir una lista</b>
• /resumir_gastos - Suma gastos, descuenta reintegros e interpreta cuotas y personas mencionadas
  <i>Ejemplo: Juan pagó cena $24000. Ana debe $12000 y Pedro debe $12000.</i>

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
		"monto":      telegram.FormatMoney(amount),
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

	sb.WriteString("\n💸 Cada persona puede elegir «Pagar una deuda» en el menú para marcar su parte.")

	h.logChangelog(chatID, userID, "", "expense", shortID, expense.Description, "divided", map[string]string{
		"participantes": fmt.Sprintf("%d", len(members)),
		"por persona":   telegram.FormatMoney(amountPerPerson),
	})

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleMyDebts maneja el comando /mis_deudas
func (h *Handler) handleMyDebts(ctx context.Context, chatID, userID int64) error {
	group, err := h.db.GetGroup(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error getting current chat metadata for debts: %v", err)
		return h.handleGroupMyDebts(ctx, chatID, userID)
	}
	if group.Type == "private" {
		return h.handleConsolidatedMyDebts(ctx, chatID, userID)
	}
	return h.handleGroupMyDebts(ctx, chatID, userID)
}

// handleGroupMyDebts shows only the caller's unpaid shares for this group.
// The message is intentionally sent to the group so members can coordinate
// openly without exposing debts from other groups.
func (h *Handler) handleGroupMyDebts(ctx context.Context, chatID, userID int64) error {
	debts, err := h.pendingDebtsForGroup(ctx, chatID, userID)
	if err != nil {
		h.logger.Printf("Error getting group debts: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener tus deudas de este grupo.")
	}
	if len(debts) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 No tenés deudas pendientes en este grupo.")
	}

	var sb strings.Builder
	sb.WriteString("💳 <b>Tus deudas en este grupo</b>\n\n")
	var total float64
	for _, debt := range debts {
		sb.WriteString(fmt.Sprintf("• %s — <b>%s</b>\n", telegram.FormatMoney(debt.split.Amount), telegram.EscapeHTML(debt.expense.Description)))
		sb.WriteString(fmt.Sprintf("  Le debés a: %s\n", telegram.EscapeHTML(debtPayeeName(debt.expense))))
		total += debt.split.Amount
	}
	sb.WriteString(fmt.Sprintf("\n💰 <b>Total en este grupo: %s</b>", telegram.FormatMoney(total)))
	sb.WriteString("\n\n💸 Elegí «Pagar una deuda» para marcar tu parte como pagada.")
	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handleConsolidatedMyDebts is the private-chat view. Splits retain their
// source chat ID, so each debt can be loaded and labeled with its own group
// instead of being incorrectly looked up in the direct conversation.
func (h *Handler) handleConsolidatedMyDebts(ctx context.Context, chatID, userID int64) error {
	splits, err := h.db.GetUserPendingSplits(ctx, userID)
	if err != nil {
		h.logger.Printf("Error getting user splits: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Error al obtener tus deudas.")
	}

	if len(splits) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No tienes deudas pendientes!")
	}

	var total float64
	var entries []string

	for _, split := range splits {
		// Legacy splits created before chat_id was introduced cannot be safely
		// attributed to a group. Never delete them as a side effect of a read.
		if split.ChatID == 0 || len(split.ExpenseID) < 8 {
			h.logger.Printf("Skipping legacy split %s without source group", split.ExpenseID)
			continue
		}
		shortID := split.ExpenseID[:8]
		expense, err := h.db.GetExpenseByShortID(ctx, split.ChatID, shortID)
		if err != nil {
			h.logger.Printf("Skipping unavailable expense %s for consolidated debts: %v", split.ExpenseID, err)
			continue
		}
		debtGroup, err := h.db.GetGroup(ctx, split.ChatID)
		if err != nil {
			h.logger.Printf("Skipping debt %s because its group is unavailable: %v", split.ExpenseID, err)
			continue
		}
		groupName := debtGroup.Title
		if groupName == "" {
			groupName = fmt.Sprintf("Grupo %d", split.ChatID)
		}
		entries = append(entries, fmt.Sprintf("• %s — <b>%s</b>\n  Grupo: %s · Le debés a: %s\n", telegram.FormatMoney(split.Amount), telegram.EscapeHTML(expense.Description), telegram.EscapeHTML(groupName), telegram.EscapeHTML(debtPayeeName(expense))))
		total += split.Amount
	}

	if len(entries) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 ¡No tienes deudas pendientes!")
	}

	var sb strings.Builder
	sb.WriteString("💳 <b>Tus deudas pendientes</b>\n<i>Todos tus grupos</i>\n\n")
	for _, entry := range entries {
		sb.WriteString(entry)
	}
	sb.WriteString(fmt.Sprintf("\n💰 <b>Total adeudado: %s</b>", telegram.FormatMoney(total)))
	sb.WriteString("\n\n💸 Para pagar una deuda, abrí el grupo correspondiente y elegí «Pagar una deuda».")

	return h.tg.SendMessage(ctx, chatID, sb.String())
}

func debtPayeeName(expense *db.Expense) string {
	if len(expense.Payers) == 1 && expense.Payers[0].UserName != "" {
		return expense.Payers[0].UserName
	}
	if len(expense.Payers) > 1 {
		return "quienes pagaron el gasto"
	}
	if expense.CreatorName != "" {
		return expense.CreatorName
	}
	return "la persona que registró el gasto"
}

// handlePay maneja el comando /pagar
func (h *Handler) handlePay(ctx context.Context, chatID, userID int64, args []string) error {
	if len(args) < 1 {
		return h.handleMenuPay(ctx, chatID, userID)
	}
	return h.handlePayByShortID(ctx, chatID, userID, strings.TrimSpace(args[0]))
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
		sb.WriteString("\n🧪 Son personas ficticias: no reciben WhatsApp ni recordatorios.\n")
		sb.WriteString("Elegí «Simular operaciones» para generar y validar una prueba completa, o «Limpiar datos de prueba» cuando termines.")
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
	deletedNames, err := h.clearTestPeople(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error clearing test users: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude eliminar las personas de prueba.")
	}
	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("🗑️ <b>Personas de prueba desactivadas:</b> %d\n\nUsá «Miembros» para verificar.", len(deletedNames)))
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
