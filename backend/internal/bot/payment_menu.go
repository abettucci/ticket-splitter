package bot

import (
	"context"
	"fmt"
	"sort"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// pendingDebt keeps the split and its expense together so payment selection is
// scoped to the current group, even if the same person belongs to other groups.
type pendingDebt struct {
	split   db.ExpenseSplit
	expense *db.Expense
}

func (h *Handler) pendingDebtsForGroup(ctx context.Context, chatID, userID int64) ([]pendingDebt, error) {
	splits, err := h.db.GetUserPendingSplitsForGroup(ctx, userID, chatID)
	if err != nil {
		return nil, err
	}

	debts := make([]pendingDebt, 0, len(splits))
	for _, split := range splits {
		if len(split.ExpenseID) < 8 {
			continue
		}
		expense, err := h.db.GetExpenseByShortID(ctx, chatID, split.ExpenseID[:8])
		if err != nil {
			continue
		}
		debts = append(debts, pendingDebt{split: split, expense: expense})
	}

	// DynamoDB Scan order is not stable. A deterministic list makes a quoted
	// WhatsApp reply ("1", "2", …) resolve to the same debt on the next Lambda
	// invocation.
	sort.Slice(debts, func(i, j int) bool {
		if debts[i].expense.CreatedAt.Equal(debts[j].expense.CreatedAt) {
			return debts[i].expense.ID < debts[j].expense.ID
		}
		return debts[i].expense.CreatedAt.After(debts[j].expense.CreatedAt)
	})
	return debts, nil
}

// handleMenuPay lists only the caller's unpaid splits. It deliberately never
// lets one member mark another member's debt as paid.
func (h *Handler) handleMenuPay(ctx context.Context, chatID, userID int64) error {
	debts, err := h.pendingDebtsForGroup(ctx, chatID, userID)
	if err != nil {
		h.logger.Printf("Error getting pending debts for payment menu: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener tus deudas pendientes.")
	}
	if len(debts) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 No tenés deudas pendientes para marcar como pagadas.")
	}
	if len(debts) == 1 {
		return h.markPayment(ctx, chatID, userID, debts[0])
	}

	rows := make([][]telegram.InlineKeyboardButton, 0, len(debts))
	for _, debt := range debts {
		label := fmt.Sprintf("💸 %s — %s", debt.expense.Description, telegram.FormatMoney(debt.split.Amount))
		if len(label) > 60 {
			label = label[:57] + "..."
		}
		rows = append(rows, []telegram.InlineKeyboardButton{{
			Text:         label,
			CallbackData: fmt.Sprintf("pay:%s", debt.expense.ID[:8]),
		}})
	}

	h.conv.Set(chatID, userID, &ConversationState{Step: StepSelectPaymentExpense})
	return h.tg.SendMessageWithOptions(ctx, &telegram.SendMessageRequest{
		ChatID:      chatID,
		Text:        "💸 <b>¿Qué deuda querés marcar como pagada?</b>\n\nElegí una. Solo se actualizará tu parte del gasto.",
		ParseMode:   "HTML",
		ReplyMarkup: telegram.InlineKeyboardMarkup{InlineKeyboard: rows},
	})
}

func (h *Handler) handlePaymentChoice(ctx context.Context, chatID, userID int64, selection int) error {
	debts, err := h.pendingDebtsForGroup(ctx, chatID, userID)
	if err != nil {
		h.logger.Printf("Error getting pending debts for payment selection: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener tus deudas pendientes.")
	}
	if selection < 1 || selection > len(debts) {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa opción ya no es válida. Elegí «Pagar una deuda» otra vez.")
	}
	h.conv.Clear(chatID, userID)
	return h.markPayment(ctx, chatID, userID, debts[selection-1])
}

func (h *Handler) handlePayByShortID(ctx context.Context, chatID, userID int64, shortID string) error {
	debts, err := h.pendingDebtsForGroup(ctx, chatID, userID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener tus deudas pendientes.")
	}
	for _, debt := range debts {
		if debt.expense.ID == shortID || debt.expense.ID[:8] == shortID {
			return h.markPayment(ctx, chatID, userID, debt)
		}
	}
	return h.tg.SendMessage(ctx, chatID, "ℹ️ No tenés una deuda pendiente con ese gasto. Puede que ya esté marcada como pagada.")
}

func (h *Handler) markPayment(ctx context.Context, chatID, userID int64, debt pendingDebt) error {
	if err := h.db.MarkSplitAsPaid(ctx, debt.expense.ID, userID); err != nil {
		h.logger.Printf("Error marking split as paid: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude marcar el pago. Actualizá el menú e intentá de nuevo.")
	}

	shortID := debt.expense.ID[:8]
	h.logChangelog(chatID, userID, "", "expense", shortID, debt.expense.Description, "paid", nil)
	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ <b>Pago registrado</b>\n\nTu parte de <b>%s</b> (%s) quedó marcada como pagada. El balance del grupo ya está actualizado.", telegram.EscapeHTML(debt.expense.Description), telegram.FormatMoney(debt.split.Amount)))
}
