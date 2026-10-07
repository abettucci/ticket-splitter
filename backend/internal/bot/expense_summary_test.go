package bot

import (
	"math"
	"testing"

	"github.com/abettucci/group-split-bot/internal/db"
)

func TestExpenseListSummaryCalculatesExpensesReimbursementsAndRoles(t *testing.T) {
	parties := buildSummaryParties([]db.Member{
		{UserID: 1, DisplayName: "Juan Pérez", Username: "juan"},
		{UserID: 2, DisplayName: "Ana García", Username: "ana"},
		{UserID: 3, DisplayName: "Pedro López", Username: "pedro"},
	}, 1, "Juan Pérez")

	summary := summarizeExpenseList(`Juan pagó cena $24.000. Ana debe $12.000 y Pedro debe $12.000
Heladera: 3 cuotas de $10.000
Tarjeta: cuota 2/6 de $5.000
Reintegro del banco $2.500 a Juan`, parties)

	assertSummaryAmount(t, summary.expenses, 59000)
	assertSummaryAmount(t, summary.reimbursements, 2500)
	assertSummaryAmount(t, summary.expenses-summary.reimbursements, 56500)
	assertSummaryAmount(t, summary.balances["member:1"], 21500)
	assertSummaryAmount(t, summary.balances["member:2"], -12000)
	assertSummaryAmount(t, summary.balances["member:3"], -12000)

	if len(summary.entries) != 4 {
		t.Fatalf("got %d interpreted entries, want 4", len(summary.entries))
	}
	if got := summary.entries[1].installment; got == nil || got.count != 3 || got.currentCharge || got.planTotal != 30000 {
		t.Fatalf("three-installment expense parsed incorrectly: %#v", got)
	}
	if got := summary.entries[2].installment; got == nil || got.current != 2 || got.count != 6 || got.remaining != 20000 {
		t.Fatalf("current installment parsed incorrectly: %#v", got)
	}
}

func TestExpenseListSummaryAcceptsAmountsWithoutCurrencyWhenContextIsClear(t *testing.T) {
	parties := buildSummaryParties([]db.Member{
		{UserID: 1, DisplayName: "Juan"},
		{UserID: 2, DisplayName: "Ana"},
		{UserID: 3, DisplayName: "Pedro"},
	}, 1, "Juan")

	summary := summarizeExpenseList("Juan pagó 10000; Ana debe 4000 y Pedro debe 6000", parties)

	assertSummaryAmount(t, summary.expenses, 10000)
	assertSummaryAmount(t, summary.balances["member:1"], 10000)
	assertSummaryAmount(t, summary.balances["member:2"], -4000)
	assertSummaryAmount(t, summary.balances["member:3"], -6000)
}

func TestExpenseListSummaryDoesNotMultiplyAnExplicitInstallmentTotal(t *testing.T) {
	summary := summarizeExpenseList("Notebook: total $36.000 en 3 cuotas", nil)

	assertSummaryAmount(t, summary.expenses, 36000)
	if len(summary.entries) != 1 || summary.entries[0].installment == nil {
		t.Fatalf("expected an installment entry, got %#v", summary.entries)
	}
	installment := summary.entries[0].installment
	if installment.planTotal != 36000 || installment.monthly != 12000 {
		t.Fatalf("explicit total was multiplied or monthly amount is wrong: %#v", installment)
	}
}

func TestExpenseListSummaryUnderstandsPayerAndDebtorWording(t *testing.T) {
	parties := buildSummaryParties([]db.Member{
		{UserID: 1, DisplayName: "Juan"},
		{UserID: 2, DisplayName: "Ana"},
	}, 1, "Juan")

	summary := summarizeExpenseList("El pagador Juan puso $8000; Ana le debe a Juan $8000", parties)

	assertSummaryAmount(t, summary.expenses, 8000)
	assertSummaryAmount(t, summary.balances["member:1"], 8000)
	assertSummaryAmount(t, summary.balances["member:2"], -8000)
}

func TestExpenseListSummaryUnderstandsBareAmountsSectionsAndShortInstallments(t *testing.T) {
	summary := summarizeExpenseList(`Desde 30-9
*Consumos*
Dietética 5.700
Nafta 39.000
Dexter 1 de 6 50.000
Bronceador 2 de 6 12.000
Seguro KA 159.200

*Reintegros*
Dietética 1.700
Nafta 8.700
Dexter 75.000`, nil)

	assertSummaryAmount(t, summary.expenses, 265900)
	assertSummaryAmount(t, summary.reimbursements, 85400)
	assertSummaryAmount(t, summary.expenses-summary.reimbursements, 180500)
	if summary.skippedLines != 0 { // Date and Markdown headings are metadata, not malformed expenses.
		t.Fatalf("got %d skipped lines, want 0", summary.skippedLines)
	}
	if len(summary.entries) != 8 {
		t.Fatalf("got %d interpreted entries, want 8", len(summary.entries))
	}
	if got := summary.entries[2].installment; got == nil || got.current != 1 || got.count != 6 || got.monthly != 50000 {
		t.Fatalf("short installment 1 de 6 parsed incorrectly: %#v", got)
	}
	if got := summary.entries[3].installment; got == nil || got.current != 2 || got.count != 6 || got.monthly != 12000 {
		t.Fatalf("short installment 2 de 6 parsed incorrectly: %#v", got)
	}
}

func assertSummaryAmount(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Fatalf("got %.2f, want %.2f", got, want)
	}
}
