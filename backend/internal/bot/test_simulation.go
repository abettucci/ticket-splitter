package bot

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
)

const (
	testUserIDStart     int64  = 999000001
	testUserIDEnd       int64  = 999000050
	testExpenseCategory string = "test_simulation"
)

var testPeople = []struct {
	name     string
	username string
}{
	{name: "Juan Pérez", username: "juan_perez_test"},
	{name: "María García", username: "maria_garcia_test"},
	{name: "Pedro López", username: "pedro_lopez_test"},
	{name: "Ana Martínez", username: "ana_martinez_test"},
	{name: "Carlos Rodríguez", username: "carlos_rodriguez_test"},
}

func isTestUserID(userID int64) bool {
	return userID >= testUserIDStart && userID <= testUserIDEnd
}

func (h *Handler) ensureTestPeople(ctx context.Context, chatID int64) ([]db.Member, error) {
	for index, person := range testPeople {
		userID := testUserIDStart + int64(index)
		if _, err := h.db.AddMember(ctx, chatID, userID, person.name, person.username); err != nil {
			return nil, fmt.Errorf("crear a %s: %w", person.name, err)
		}
	}

	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		return nil, err
	}
	testMembers := make([]db.Member, 0, len(testPeople))
	for _, member := range members {
		if isTestUserID(member.UserID) {
			testMembers = append(testMembers, member)
		}
	}
	sort.Slice(testMembers, func(i, j int) bool { return testMembers[i].UserID < testMembers[j].UserID })
	if len(testMembers) < 4 {
		return nil, fmt.Errorf("se esperaban al menos 4 personas de prueba, hay %d", len(testMembers))
	}
	return testMembers, nil
}

func (h *Handler) clearTestPeople(ctx context.Context, chatID int64) ([]string, error) {
	members, err := h.db.GetGroupMembers(ctx, chatID)
	if err != nil {
		return nil, err
	}

	deleted := make([]string, 0)
	for _, member := range members {
		if !isTestUserID(member.UserID) {
			continue
		}
		if err := h.db.RemoveMember(ctx, chatID, member.UserID); err != nil {
			return deleted, err
		}
		deleted = append(deleted, member.DisplayName)
	}
	sort.Strings(deleted)
	return deleted, nil
}

// clearTestExpenses only touches simulations that this feature created. Real
// expenses, including manually-created ones involving a test person, stay out
// of scope and are never removed here.
func (h *Handler) clearTestExpenses(ctx context.Context, chatID, deletedBy int64) (int, error) {
	expenses, err := h.db.GetGroupExpenses(ctx, chatID, 1000)
	if err != nil {
		return 0, err
	}

	deleted := 0
	for _, expense := range expenses {
		if expense.Category != testExpenseCategory {
			continue
		}
		if err := h.db.DeleteExpense(ctx, expense.ID, expense.PK, expense.SK, deletedBy); err != nil {
			return deleted, err
		}
		deleted++
	}
	return deleted, nil
}

// handleClearTestData removes the disposable simulation and deactivates only
// members inside the reserved test-ID range. It never sends WhatsApp messages.
func (h *Handler) handleClearTestData(ctx context.Context, chatID, userID int64) error {
	expenses, err := h.clearTestExpenses(ctx, chatID, userID)
	if err != nil {
		h.logger.Printf("Error clearing test expenses: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude limpiar los gastos de prueba.")
	}
	people, err := h.clearTestPeople(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error clearing test people: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ Limpié los gastos de prueba, pero no pude quitar las personas de prueba.")
	}
	return h.tg.SendMessage(ctx, chatID, fmt.Sprintf("🧹 <b>Datos de prueba limpiados</b>\n\n• Gastos de simulación eliminados: %d\n• Personas de prueba desactivadas: %d\n\nTus gastos reales no fueron modificados.", expenses, len(people)))
}

// handleRunTestSimulation executes a small, random but reproducible set of
// real operations over disposable data. It intentionally runs only with fake
// members and marks every generated expense with testExpenseCategory.
func (h *Handler) handleRunTestSimulation(ctx context.Context, chatID, userID int64) error {
	previousExpenses, err := h.clearTestExpenses(ctx, chatID, userID)
	if err != nil {
		h.logger.Printf("Error resetting prior test simulation: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude reiniciar la simulación anterior.")
	}

	members, err := h.ensureTestPeople(ctx, chatID)
	if err != nil {
		h.logger.Printf("Error preparing test people: %v", err)
		return h.tg.SendMessage(ctx, chatID, "❌ No pude preparar las personas de prueba.")
	}
	members = members[:4]

	seed := time.Now().UnixNano()
	rng := rand.New(rand.NewSource(seed))
	rng.Shuffle(len(members), func(i, j int) { members[i], members[j] = members[j], members[i] })

	created := make([]*db.Expense, 0, 3)
	var operations []string

	// 1. Equal split and a payment by one of the debtors.
	equalTotal := float64(rng.Intn(6)+5) * 4000 // $20.000 … $40.000
	equalExpense, err := h.createSimulationExpense(ctx, chatID, "🧪 Simulación · Cena compartida", equalTotal, members[0])
	if err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.CreateSplits(ctx, equalExpense, members); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.MarkSplitAsPaid(ctx, equalExpense.ID, members[1].UserID); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	created = append(created, equalExpense)
	operations = append(operations, fmt.Sprintf("1. <b>Cena compartida</b>: %s ÷ 4 = <b>%s</b> por persona. Pagó %s; %s marcó su parte como pagada.", telegram.FormatMoney(equalTotal), telegram.FormatMoney(equalTotal/4), telegram.EscapeHTML(members[0].DisplayName), telegram.EscapeHTML(members[1].DisplayName)))

	// 2. Custom division. The amounts are generated as whole ARS values and
	// always sum exactly to the expense total.
	base := float64(rng.Intn(5)+2) * 1000
	customAmounts := map[int64]float64{
		members[0].UserID: base,
		members[1].UserID: base * 2,
		members[2].UserID: base * 3,
		members[3].UserID: base * 4,
	}
	customTotal := base * 10
	customExpense, err := h.createSimulationExpense(ctx, chatID, "🧪 Simulación · Regalo", customTotal, members[1])
	if err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.CreateSplitsWithCustomAmounts(ctx, customExpense, customAmounts, []db.Payer{{UserID: members[1].UserID, UserName: members[1].DisplayName, Amount: customTotal}}); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.MarkSplitAsPaid(ctx, customExpense.ID, members[2].UserID); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	created = append(created, customExpense)
	operations = append(operations, fmt.Sprintf("2. <b>Regalo</b>: división personalizada %s + %s + %s + %s = <b>%s</b>. Pagó %s y %s saldó su parte.", telegram.FormatMoney(base), telegram.FormatMoney(base*2), telegram.FormatMoney(base*3), telegram.FormatMoney(base*4), telegram.FormatMoney(customTotal), telegram.EscapeHTML(members[1].DisplayName), telegram.EscapeHTML(members[2].DisplayName)))

	// 3. Change payer, then redivide. This validates that old payer becomes a
	// debtor, new payer becomes paid, and a subsequent redistribution rewrites
	// the splits instead of adding duplicate debt.
	redivideBase := float64(rng.Intn(5)+2) * 1000
	redivideTotal := redivideBase * 10
	redivideExpense, err := h.createSimulationExpense(ctx, chatID, "🧪 Simulación · Salida", redivideTotal, members[2])
	if err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.CreateSplits(ctx, redivideExpense, members); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	oldPayer := members[2]
	newPayer := members[3]
	redivideExpense.CreatedBy = newPayer.UserID
	redivideExpense.CreatorName = newPayer.DisplayName
	if err := h.db.UpdateExpense(ctx, redivideExpense); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.UpdateSplitsPayer(ctx, redivideExpense.ID, oldPayer.UserID, newPayer.UserID); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	if err := h.db.ResetExpenseDivision(ctx, redivideExpense); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	redividedAmounts := map[int64]float64{
		members[0].UserID: redivideBase,
		members[1].UserID: redivideBase * 2,
		members[2].UserID: redivideBase * 3,
		members[3].UserID: redivideBase * 4,
	}
	if err := h.db.CreateSplitsWithCustomAmounts(ctx, redivideExpense, redividedAmounts, []db.Payer{{UserID: newPayer.UserID, UserName: newPayer.DisplayName, Amount: redivideTotal}}); err != nil {
		return h.simulationFailure(ctx, chatID, err)
	}
	created = append(created, redivideExpense)
	operations = append(operations, fmt.Sprintf("3. <b>Salida</b>: %s. Se cambió el pagador de %s a %s y se redividió con nuevas partes %s + %s + %s + %s = <b>%s</b>.", telegram.FormatMoney(redivideTotal), telegram.EscapeHTML(oldPayer.DisplayName), telegram.EscapeHTML(newPayer.DisplayName), telegram.FormatMoney(redivideBase), telegram.FormatMoney(redivideBase*2), telegram.FormatMoney(redivideBase*3), telegram.FormatMoney(redivideBase*4), telegram.FormatMoney(redivideTotal)))

	return h.sendSimulationReport(ctx, chatID, seed, previousExpenses, members, created, operations)
}

func (h *Handler) createSimulationExpense(ctx context.Context, chatID int64, description string, amount float64, payer db.Member) (*db.Expense, error) {
	expense, err := h.db.CreateExpense(ctx, chatID, description, amount, payer.UserID, payer.DisplayName)
	if err != nil {
		return nil, err
	}
	expense.Category = testExpenseCategory
	if err := h.db.UpdateExpense(ctx, expense); err != nil {
		return nil, err
	}
	return expense, nil
}

func (h *Handler) simulationFailure(ctx context.Context, chatID int64, err error) error {
	h.logger.Printf("Test simulation failed: %v", err)
	return h.tg.SendMessage(ctx, chatID, "❌ La simulación no pudo completarse. Podés tocar «Limpiar datos de prueba» e intentarlo otra vez.")
}

func (h *Handler) sendSimulationReport(ctx context.Context, chatID int64, seed int64, previousExpenses int, members []db.Member, expenses []*db.Expense, operations []string) error {
	balances := make(map[int64]float64, len(members))
	names := make(map[int64]string, len(members))
	for _, member := range members {
		balances[member.UserID] = 0
		names[member.UserID] = member.DisplayName
	}

	var sb strings.Builder
	sb.WriteString("🎲 <b>Simulación de prueba completada</b>\n")
	sb.WriteString(fmt.Sprintf("<i>Semilla: %d · gastos anteriores eliminados: %d</i>\n\n", seed, previousExpenses))
	sb.WriteString("<b>Operaciones ejecutadas</b>\n")
	for _, operation := range operations {
		sb.WriteString(operation + "\n\n")
	}

	sb.WriteString("<b>Verificación de cálculos</b>\n")
	allValid := true
	for _, expense := range expenses {
		splits, err := h.db.GetExpenseSplits(ctx, expense.ID)
		if err != nil {
			return h.simulationFailure(ctx, chatID, err)
		}
		var splitTotal, paid, pending float64
		for _, split := range splits {
			splitTotal += split.Amount
			if split.IsPaid {
				paid += split.Amount
			} else {
				pending += split.Amount
				balances[split.UserID] -= split.Amount
				balances[expense.CreatedBy] += split.Amount
			}
		}
		valid := len(splits) > 0 && math.Abs(splitTotal-expense.TotalAmount) < 0.01 && math.Abs((paid+pending)-expense.TotalAmount) < 0.01
		if !valid {
			allValid = false
		}
		status := "✅"
		if !valid {
			status = "❌"
		}
		sb.WriteString(fmt.Sprintf("%s %s: divisiones %s = total %s · pagado %s · pendiente %s\n", status, telegram.EscapeHTML(expense.Description), telegram.FormatMoney(splitTotal), telegram.FormatMoney(expense.TotalAmount), telegram.FormatMoney(paid), telegram.FormatMoney(pending)))
	}

	sb.WriteString("\n<b>Balance de esta simulación</b>\n")
	for _, member := range members {
		balance := balances[member.UserID]
		switch {
		case balance > 0.01:
			sb.WriteString(fmt.Sprintf("• %s debe cobrar %s\n", telegram.EscapeHTML(names[member.UserID]), telegram.FormatMoney(balance)))
		case balance < -0.01:
			sb.WriteString(fmt.Sprintf("• %s debe pagar %s\n", telegram.EscapeHTML(names[member.UserID]), telegram.FormatMoney(-balance)))
		default:
			sb.WriteString(fmt.Sprintf("• %s está saldado\n", telegram.EscapeHTML(names[member.UserID])))
		}
	}

	if allValid {
		sb.WriteString("\n✅ <b>Validación OK:</b> en cada gasto, pagado + pendiente coincide exactamente con el total.")
	} else {
		sb.WriteString("\n❌ <b>Validación con diferencias:</b> revisá los detalles antes de usar estas operaciones como referencia.")
	}
	sb.WriteString("\n\n🧹 Usá «Limpiar datos de prueba» para borrar únicamente esta simulación y las personas ficticias.")
	return h.tg.SendMessage(ctx, chatID, sb.String())
}
