package bot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/security"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// handleGiftLists is the entry point for collaborative gift planning. Lists
// belong to a stable gift-list partition, not to a single chat, and are linked
// to groups explicitly by their share code.
func (h *Handler) handleGiftLists(ctx context.Context, chatID, userID int64) error {
	links, err := h.db.GetGroupGiftLists(ctx, chatID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ No pude cargar las listas de regalos. Intentá de nuevo.")
	}

	options := make(map[int]string, len(links))
	var text strings.Builder
	text.WriteString("🎁 <b>Regalos compartidos</b>\n\n")
	text.WriteString("1. ➕ Crear una lista nueva\n")
	text.WriteString("2. 🔗 Vincular una lista de otro grupo\n")
	if len(links) == 0 {
		text.WriteString("\nTodavía no hay listas en este grupo. Creá una o pegá el código que te compartieron.")
	} else {
		text.WriteString("\n<b>Listas disponibles</b>\n")
		for index, link := range links {
			n := index + 3
			options[n] = link.ShareCode
			fmt.Fprintf(&text, "%d. %s <code>%s</code>\n", n, telegram.EscapeHTML(link.GiftListName), link.ShareCode)
		}
	}
	text.WriteString("\nRespondé con un número. Las listas vinculadas se ven igual desde todos los grupos que tengan el código.")

	h.conv.Set(chatID, userID, &ConversationState{Step: StepSelectGiftList, DivideOptions: options})
	keyboard := telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{
		{{Text: "➕ Crear lista", CallbackData: "gift:create"}, {Text: "🔗 Vincular lista", CallbackData: "gift:link"}},
	}}
	for _, link := range links {
		keyboard.InlineKeyboard = append(keyboard.InlineKeyboard, []telegram.InlineKeyboardButton{{
			Text: fmt.Sprintf("🎁 %s", truncateGiftLabel(link.GiftListName, 45)), CallbackData: fmt.Sprintf("gift:view:%s", link.ShareCode),
		}})
	}
	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID: chatID, Text: text.String(), ParseMode: "HTML", ReplyMarkup: keyboard,
	})
}

func (h *Handler) startGiftListCreation(ctx context.Context, chatID, userID int64) error {
	h.conv.Set(chatID, userID, &ConversationState{Step: StepGiftListName})
	return h.tg.SendMessage(ctx, chatID, "🎁 <b>Nueva lista de regalos</b>\n\n¿Cómo se llama? Ejemplo: <i>Cumple de Lauti</i>.")
}

func (h *Handler) startGiftListLink(ctx context.Context, chatID, userID int64) error {
	h.conv.Set(chatID, userID, &ConversationState{Step: StepGiftListLinkCode})
	return h.tg.SendMessage(ctx, chatID, "🔗 Pegá el código de la lista que querés compartir con este grupo.\n\nEjemplo: <code>AB12CD34</code>")
}

func (h *Handler) showGiftList(ctx context.Context, chatID, userID int64, code string) error {
	list, err := h.giftListLinkedToGroup(ctx, chatID, code)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa lista no está vinculada a este grupo. Abrí «Regalos» y elegí «Vincular una lista».")
	}
	items, err := h.db.GetGiftItems(ctx, list.ID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ No pude cargar los regalos de la lista.")
	}
	contributions, err := h.db.GetGiftContributions(ctx, list.ID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ No pude cargar los aportes de la lista.")
	}
	contributed := make(map[string]float64, len(items))
	for _, contribution := range contributions {
		contributed[contribution.GiftItemID] += contribution.Amount
	}

	var text strings.Builder
	fmt.Fprintf(&text, "🎁 <b>%s</b>\n", telegram.EscapeHTML(list.Name))
	fmt.Fprintf(&text, "🔗 Código para compartir: <code>%s</code>\n", list.ShareCode)
	text.WriteString("<i>Podés vincular este código desde otro grupo sin duplicar la lista.</i>\n")
	if len(items) == 0 {
		text.WriteString("\nTodavía no hay regalos cargados.")
	} else {
		text.WriteString("\n<b>Regalos y aportes</b>\n")
		for index, item := range items {
			amount := contributed[item.ID]
			fmt.Fprintf(&text, "\n%d. 🎁 <b>%s</b>\n", index+1, telegram.EscapeHTML(item.Description))
			if item.TargetAmount > 0 {
				remaining := item.TargetAmount - amount
				if remaining < 0 {
					remaining = 0
				}
				fmt.Fprintf(&text, "   🎯 Meta: %s · 💸 Aportado: %s · Falta: %s\n", telegram.FormatMoney(item.TargetAmount), telegram.FormatMoney(amount), telegram.FormatMoney(remaining))
			} else {
				fmt.Fprintf(&text, "   💸 Aportado: %s\n", telegram.FormatMoney(amount))
			}
		}
	}
	text.WriteString("\nRespondé: <b>1</b> para agregar un regalo, <b>2</b> para registrar un aporte o <b>3</b> para volver.")

	h.conv.Set(chatID, userID, &ConversationState{Step: StepSelectGiftListAction, GiftListCode: list.ShareCode})
	keyboard := telegram.InlineKeyboardMarkup{InlineKeyboard: [][]telegram.InlineKeyboardButton{
		{{Text: "➕ Agregar regalo", CallbackData: fmt.Sprintf("gift:add:%s", list.ShareCode)}, {Text: "💸 Registrar aporte", CallbackData: fmt.Sprintf("gift:contribute:%s", list.ShareCode)}},
		{{Text: "↩️ Mis listas", CallbackData: "gift:back"}},
	}}
	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID: chatID, Text: text.String(), ParseMode: "HTML", ReplyMarkup: keyboard,
	})
}

func (h *Handler) giftListLinkedToGroup(ctx context.Context, chatID int64, code string) (*db.GiftList, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	links, err := h.db.GetGroupGiftLists(ctx, chatID)
	if err != nil {
		return nil, err
	}
	for _, link := range links {
		if link.ShareCode == code {
			return h.db.GetGiftList(ctx, link.GiftListID)
		}
	}
	return nil, fmt.Errorf("gift list not linked")
}

func (h *Handler) startGiftItemFlow(ctx context.Context, chatID, userID int64, code string) error {
	if _, err := h.giftListLinkedToGroup(ctx, chatID, code); err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa lista ya no está disponible en este grupo.")
	}
	h.conv.Set(chatID, userID, &ConversationState{Step: StepGiftItemDescription, GiftListCode: strings.ToUpper(code)})
	return h.tg.SendMessage(ctx, chatID, "🎁 ¿Qué regalo querés agregar?\n\nEjemplo: <i>Auriculares para Lauti</i>")
}

func (h *Handler) startGiftContributionSelection(ctx context.Context, chatID, userID int64, code string) error {
	list, err := h.giftListLinkedToGroup(ctx, chatID, code)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa lista ya no está disponible en este grupo.")
	}
	items, err := h.db.GetGiftItems(ctx, list.ID)
	if err != nil || len(items) == 0 {
		return h.tg.SendMessage(ctx, chatID, "ℹ️ Primero agregá al menos un regalo a la lista.")
	}

	options := make(map[int]string, len(items))
	var text strings.Builder
	text.WriteString("💸 <b>¿A cuál regalo querés aportar?</b>\n\n")
	for index, item := range items {
		n := index + 1
		options[n] = item.ID
		fmt.Fprintf(&text, "%d. %s\n", n, telegram.EscapeHTML(item.Description))
	}
	h.conv.Set(chatID, userID, &ConversationState{Step: StepSelectGiftContributionItem, GiftListCode: list.ShareCode, DivideOptions: options})
	return h.tg.SendMessage(ctx, chatID, text.String())
}

// handleGiftInteractive keeps the interactive UX and the numbered fallback on
// the same implementation, which matters because WhatsApp support differs by
// client and list messages can be unavailable.
func (h *Handler) handleGiftInteractive(ctx context.Context, chatID, userID int64, userName, payload string) error {
	parts := strings.SplitN(payload, ":", 2)
	action := parts[0]
	code := ""
	if len(parts) == 2 {
		code = parts[1]
	}
	switch action {
	case "create":
		return h.startGiftListCreation(ctx, chatID, userID)
	case "link":
		return h.startGiftListLink(ctx, chatID, userID)
	case "view":
		return h.showGiftList(ctx, chatID, userID, code)
	case "add":
		return h.startGiftItemFlow(ctx, chatID, userID, code)
	case "contribute":
		return h.startGiftContributionSelection(ctx, chatID, userID, code)
	case "back":
		return h.handleGiftLists(ctx, chatID, userID)
	default:
		return h.tg.SendMessage(ctx, chatID, "❌ Esa acción de regalos ya no está disponible. Abrí «Regalos» de nuevo.")
	}
}

func (h *Handler) handleGiftConversationStep(ctx context.Context, chatID, userID int64, userName, text string, state *ConversationState) error {
	switch state.Step {
	case StepSelectGiftList:
		n, err := strconv.Atoi(strings.TrimSpace(text))
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ Elegí 1 para crear, 2 para vincular o el número de una lista.")
		}
		switch n {
		case 1:
			return h.startGiftListCreation(ctx, chatID, userID)
		case 2:
			return h.startGiftListLink(ctx, chatID, userID)
		default:
			if code, ok := state.DivideOptions[n]; ok {
				return h.showGiftList(ctx, chatID, userID, code)
			}
			return h.tg.SendMessage(ctx, chatID, "❌ Esa lista no existe. Elegí una opción del mensaje anterior.")
		}

	case StepGiftListName:
		name, valid := security.ValidateDescription(text)
		if !valid || len([]rune(name)) < 2 {
			return h.tg.SendMessage(ctx, chatID, "❌ Escribí un nombre válido para la lista.")
		}
		list, err := h.db.CreateGiftList(ctx, chatID, userID, userName, name)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ No pude crear la lista. Intentá de nuevo.")
		}
		h.conv.Clear(chatID, userID)
		if err := h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ Lista <b>%s</b> creada.\n\n🔗 Código para compartir: <code>%s</code>", telegram.EscapeHTML(list.Name), list.ShareCode)); err != nil {
			return err
		}
		return h.showGiftList(ctx, chatID, userID, list.ShareCode)

	case StepGiftListLinkCode:
		list, err := h.db.LinkGiftList(ctx, chatID, userID, text)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ No encontré ese código. Pedile a alguien del otro grupo que copie el código completo de su lista.")
		}
		h.conv.Clear(chatID, userID)
		if err := h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ Vinculé <b>%s</b> a este grupo.", telegram.EscapeHTML(list.Name))); err != nil {
			return err
		}
		return h.showGiftList(ctx, chatID, userID, list.ShareCode)

	case StepSelectGiftListAction:
		lower := strings.ToLower(strings.TrimSpace(text))
		switch {
		case lower == "1" || strings.Contains(lower, "agregar"):
			return h.startGiftItemFlow(ctx, chatID, userID, state.GiftListCode)
		case lower == "2" || strings.Contains(lower, "aporte"):
			return h.startGiftContributionSelection(ctx, chatID, userID, state.GiftListCode)
		case lower == "3" || strings.Contains(lower, "volver"):
			return h.handleGiftLists(ctx, chatID, userID)
		default:
			return h.tg.SendMessage(ctx, chatID, "❌ Elegí 1 para agregar un regalo, 2 para aportar o 3 para volver.")
		}

	case StepGiftItemDescription:
		description, valid := security.ValidateDescription(text)
		if !valid || len([]rune(description)) < 2 {
			return h.tg.SendMessage(ctx, chatID, "❌ Escribí una descripción válida para el regalo.")
		}
		state.Description = description
		state.Step = StepGiftItemTarget
		h.conv.Set(chatID, userID, state)
		return h.tg.SendMessage(ctx, chatID, "🎯 ¿Cuál es el monto objetivo en ARS? Escribí solo el número o <b>sin monto</b> si todavía no lo saben.")

	case StepGiftItemTarget:
		target, ok := parseGiftAmount(text, true)
		if !ok || target < 0 || !security.ValidateAmount(target) && target != 0 {
			return h.tg.SendMessage(ctx, chatID, "❌ Ingresá un monto válido o escribí «sin monto».")
		}
		list, err := h.giftListLinkedToGroup(ctx, chatID, state.GiftListCode)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ La lista dejó de estar disponible en este grupo.")
		}
		item, err := h.db.CreateGiftItem(ctx, list.ID, state.Description, target, userID)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ No pude agregar el regalo. Intentá de nuevo.")
		}
		h.conv.Clear(chatID, userID)
		if err := h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ Agregué <b>%s</b> a %s.", telegram.EscapeHTML(item.Description), telegram.EscapeHTML(list.Name))); err != nil {
			return err
		}
		return h.showGiftList(ctx, chatID, userID, list.ShareCode)

	case StepSelectGiftContributionItem:
		n, err := strconv.Atoi(strings.TrimSpace(text))
		itemID, ok := state.DivideOptions[n]
		if err != nil || !ok {
			return h.tg.SendMessage(ctx, chatID, "❌ Elegí el número del regalo al que querés aportar.")
		}
		state.GiftItemID = itemID
		state.Step = StepGiftContributionAmount
		h.conv.Set(chatID, userID, state)
		return h.tg.SendMessage(ctx, chatID, "💸 ¿Cuánto aportás? Escribí solo el monto en ARS.")

	case StepGiftContributionAmount:
		amount, ok := parseGiftAmount(text, false)
		if !ok || amount <= 0 || !security.ValidateAmount(amount) {
			return h.tg.SendMessage(ctx, chatID, "❌ Ingresá un aporte válido mayor a cero.")
		}
		list, err := h.giftListLinkedToGroup(ctx, chatID, state.GiftListCode)
		if err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ La lista dejó de estar disponible en este grupo.")
		}
		items, err := h.db.GetGiftItems(ctx, list.ID)
		if err != nil || !giftItemExists(items, state.GiftItemID) {
			return h.tg.SendMessage(ctx, chatID, "❌ Ese regalo ya no está disponible. Abrí la lista de nuevo.")
		}
		if _, err := h.db.CreateGiftContribution(ctx, list.ID, state.GiftItemID, amount, userID, userName); err != nil {
			return h.tg.SendMessage(ctx, chatID, "❌ No pude registrar tu aporte. Intentá de nuevo.")
		}
		h.conv.Clear(chatID, userID)
		if err := h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ Registré tu aporte de <b>%s</b>. No crea una deuda: sólo actualiza el fondo del regalo.", telegram.FormatMoney(amount))); err != nil {
			return err
		}
		return h.showGiftList(ctx, chatID, userID, list.ShareCode)
	}
	return nil
}

func parseGiftAmount(text string, allowEmpty bool) (float64, bool) {
	value := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "$")))
	if allowEmpty && (value == "sin monto" || value == "sin meta" || value == "no se" || value == "no sé") {
		return 0, true
	}
	value = strings.ReplaceAll(value, " ", "")
	if strings.Contains(value, ",") {
		value = strings.ReplaceAll(value, ".", "")
		value = strings.ReplaceAll(value, ",", ".")
	} else if dot := strings.LastIndex(value, "."); dot >= 0 && len(value)-dot-1 == 3 {
		// In Argentine Spanish, a dot followed by exactly three digits is much
		// more likely a thousands separator ($15.000) than a decimal amount.
		value = strings.ReplaceAll(value, ".", "")
	}
	amount, err := strconv.ParseFloat(value, 64)
	return amount, err == nil
}

func giftItemExists(items []db.GiftItem, itemID string) bool {
	for _, item := range items {
		if item.ID == itemID {
			return true
		}
	}
	return false
}

func truncateGiftLabel(label string, max int) string {
	runes := []rune(label)
	if len(runes) <= max {
		return label
	}
	return string(runes[:max-1]) + "…"
}
