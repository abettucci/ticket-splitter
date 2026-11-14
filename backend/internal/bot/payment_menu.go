package bot

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

// paymentMenuLimit keeps the numbered payment list short enough to read in a
// WhatsApp chat.
const paymentMenuLimit = 20

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

	sortDebts(debts)
	return debts, nil
}

// groupOpenDebts lists the unpaid splits of every member of the group.
func (h *Handler) groupOpenDebts(ctx context.Context, chatID int64) ([]pendingDebt, error) {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 1000)
	if err != nil {
		return nil, err
	}

	debts := make([]pendingDebt, 0)
	for _, expense := range expenses {
		exp := expense
		splits, err := h.db.GetExpenseSplits(ctx, exp.ID)
		if err != nil {
			continue
		}
		for _, split := range splits {
			if !split.IsPaid {
				debts = append(debts, pendingDebt{split: split, expense: &exp})
			}
		}
	}

	sortDebts(debts)
	return debts, nil
}

// DynamoDB Scan order is not stable. A deterministic list makes a quoted
// WhatsApp reply ("1", "2", …) resolve to the same debt on the next Lambda
// invocation.
func sortDebts(debts []pendingDebt) {
	sort.Slice(debts, func(i, j int) bool {
		if debts[i].expense.CreatedAt.Equal(debts[j].expense.CreatedAt) {
			return debts[i].expense.ID < debts[j].expense.ID
		}
		return debts[i].expense.CreatedAt.After(debts[j].expense.CreatedAt)
	})
}

// handleMenuPay lists the unpaid debts of the whole group, so the caller can
// record a payment made for any member, then asks who paid it.
func (h *Handler) handleMenuPay(ctx context.Context, chatID, userID int64) error {
	debts, err := h.groupOpenDebts(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error getting group debts for payment menu: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener las deudas pendientes del grupo.")
	}
	if len(debts) == 0 {
		return h.tg.SendMessage(ctx, chatID, "🎉 No hay deudas pendientes en este grupo.")
	}

	shown := debts
	if len(shown) > paymentMenuLimit {
		shown = shown[:paymentMenuLimit]
	}

	var sb strings.Builder
	sb.WriteString("💸 <b>¿Qué deuda querés marcar como pagada?</b>\n\nRespondé con el número:\n\n")
	for i, debt := range shown {
		fmt.Fprintf(&sb, "%d. %s debe %s — %s\n", i+1,
			telegram.EscapeHTML(debt.split.UserName),
			telegram.FormatMoney(debt.split.Amount),
			telegram.EscapeHTML(debt.expense.Description))
	}
	if len(debts) > len(shown) {
		fmt.Fprintf(&sb, "\n<i>Mostrando las primeras %d de %d.</i>", len(shown), len(debts))
	}

	h.conv.Set(chatID, userID, &ConversationState{Step: StepSelectPaymentExpense})
	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handlePaymentChoice stores the chosen debt and asks who paid it.
func (h *Handler) handlePaymentChoice(ctx context.Context, chatID, userID int64, selection int) error {
	debts, err := h.groupOpenDebts(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error getting group debts for payment selection: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener las deudas pendientes del grupo.")
	}
	if len(debts) > paymentMenuLimit {
		debts = debts[:paymentMenuLimit]
	}
	if selection < 1 || selection > len(debts) {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa opción ya no es válida. Elegí «Pagar una deuda» otra vez.")
	}
	chosen := debts[selection-1]

	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error getting members for payer selection: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude obtener los miembros del grupo.")
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "🙋 <b>¿Quién pagó la parte de %s</b> (%s)?\n\nRespondé con el número:\n\n",
		telegram.EscapeHTML(chosen.split.UserName),
		telegram.FormatMoney(chosen.split.Amount))
	options := make(map[int]int64, len(members))
	for i, member := range members {
		options[i+1] = member.UserID
		fmt.Fprintf(&sb, "%d. %s\n", i+1, telegram.EscapeHTML(member.DisplayName))
	}

	// The debtor is stored in SelectedParticipantIDs so the flow needs no new
	// persisted fields: the expense short ID, the debtor, and the payer options.
	h.conv.Set(chatID, userID, &ConversationState{
		Step:                   StepSelectPaymentPayer,
		ExpenseShortID:         chosen.expense.ID[:8],
		SelectedParticipantIDs: []int64{chosen.split.UserID},
		ParticipantOptions:     options,
	})
	return h.tg.SendMessage(ctx, chatID, sb.String())
}

// handlePaymentPayerChoice records the payment of the chosen debtor's split
// and the member who paid it.
func (h *Handler) handlePaymentPayerChoice(ctx context.Context, chatID, userID int64, state *ConversationState, selection int) error {
	payerID, ok := state.ParticipantOptions[selection]
	if !ok {
		return h.tg.SendMessage(ctx, chatID, "❌ Esa opción no es válida. Respondé con el número de quien pagó.")
	}
	if len(state.SelectedParticipantIDs) != 1 || state.ExpenseShortID == "" {
		h.conv.Clear(chatID, userID)
		return h.tg.SendMessage(ctx, chatID, "❌ Se perdió la deuda elegida. Elegí «Pagar una deuda» otra vez.")
	}
	debtorID := state.SelectedParticipantIDs[0]
	h.conv.Clear(chatID, userID)

	expense, err := h.db.GetExpenseByShortID(ctx, chatID, state.ExpenseShortID)
	if err != nil {
		return h.tg.SendMessage(ctx, chatID, "❌ No encontré ese gasto. Elegí «Pagar una deuda» otra vez.")
	}
	splits, err := h.db.GetExpenseSplits(ctx, expense.ID)
	if err != nil {
		h.logger.Printf("Error getting splits for payment: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude leer las partes del gasto.")
	}
	var split *db.ExpenseSplit
	for i := range splits {
		if splits[i].UserID == debtorID && !splits[i].IsPaid {
			split = &splits[i]
			break
		}
	}
	if split == nil {
		return h.tg.SendMessage(ctx, chatID, "ℹ️ Esa deuda ya no está pendiente.")
	}

	payerName := fmt.Sprintf("%d", payerID)
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err == nil {
		for _, member := range members {
			if member.UserID == payerID {
				payerName = member.DisplayName
				break
			}
		}
	}

	if err := h.db.MarkSplitAsPaidBy(ctx, expense.ID, debtorID, payerID, payerName); err != nil {
		h.logger.Printf("Error marking split as paid by another member: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude marcar el pago. Actualizá el menú e intentá de nuevo.")
	}

	shortID := expense.ID[:8]
	h.logChangelog(chatID, userID, "", "expense", shortID, expense.Description, "paid", map[string]string{
		"debtor":  split.UserName,
		"paid_by": payerName,
	})
	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("✅ <b>Pago registrado</b>\n\nLa parte de <b>%s</b> en <b>%s</b> (%s) quedó marcada como pagada por <b>%s</b>. El balance del grupo ya está actualizado.",
		telegram.EscapeHTML(split.UserName),
		telegram.EscapeHTML(expense.Description),
		telegram.FormatMoney(split.Amount),
		telegram.EscapeHTML(payerName)))
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
