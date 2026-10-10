package bot

import (
	"testing"
	"time"

	"github.com/abettucci/group-split-bot/internal/db"
)

func TestResolveMenuOptionTextHandlesLabelsNumbersAndTypos(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "@Splitter ver gastos", want: "ver_gastos", ok: true},
		{input: "ver gastso", want: "ver_gastos", ok: true},
		{input: "resumir lista", want: "resumir_lista", ok: true},
		{input: "Opción 15", want: "resumir_lista", ok: true},
		{input: "estoy hablando de gastos", ok: false},
	}

	for _, test := range tests {
		got, ok := resolveMenuOptionText(test.input)
		if ok != test.ok || got != test.want {
			t.Fatalf("resolveMenuOptionText(%q) = (%q, %t), want (%q, %t)", test.input, got, ok, test.want, test.ok)
		}
	}
}

func TestSafeWhatsAppGroupContinuationOnlyAllowsStructuredSelections(t *testing.T) {
	tests := []struct {
		name  string
		state *ConversationState
		text  string
		want  bool
	}{
		{name: "menu number", state: &ConversationState{Step: StepSelectMenuOption}, text: "1", want: true},
		{name: "menu label with typo", state: &ConversationState{Step: StepSelectMenuOption}, text: "ver gastso", want: true},
		{name: "divide selector number", state: &ConversationState{Step: StepSelectDivideExpense}, text: "2", want: true},
		{name: "participant selector numeric list", state: &ConversationState{Step: StepSelectExpenseParticipants}, text: "1, 3", want: true},
		{name: "participant selector ignores ordinary text", state: &ConversationState{Step: StepSelectExpenseParticipants}, text: "Cecilia llega más tarde", want: false},
		{name: "redivide selector number", state: &ConversationState{Step: StepSelectRedivideMode}, text: "2", want: true},
		{name: "redivide confirmation", state: &ConversationState{Step: StepConfirmRedivide}, text: "confirmar", want: true},
		{name: "free expense description is accepted", state: &ConversationState{Step: StepNewExpenseDescription}, text: "nafta y seguro", want: true},
		{name: "free expense amount is accepted", state: &ConversationState{Step: StepNewExpenseAmount}, text: "180500", want: true},
		{name: "ordinary chat is not accepted", state: &ConversationState{Step: StepSelectMenuOption}, text: "che llego en diez", want: false},
		{name: "no active flow", state: nil, text: "1", want: false},
	}

	for _, test := range tests {
		if got := isSafeWhatsAppGroupContinuation(test.state, test.text); got != test.want {
			t.Fatalf("%s: isSafeWhatsAppGroupContinuation(%q) = %t, want %t", test.name, test.text, got, test.want)
		}
	}
}

func TestConversationRecordRoundTripKeepsUserFlow(t *testing.T) {
	expiresAt := time.Now().Add(5 * time.Minute).Round(0)
	state := &ConversationState{
		Step:                   StepConfirmRedivide,
		Description:            "Regalo",
		Amount:                 180500,
		ExpenseShortID:         "a1b2c3d4",
		ExpiresAt:              expiresAt,
		DivideOptions:          map[int]string{1: "a1b2c3d4"},
		ParticipantOptions:     map[int]int64{1: 10, 2: 20},
		SelectedParticipantIDs: []int64{10},
	}

	record := conversationRecordFromState(500, 600, state)
	restored := conversationStateFromRecord(record)
	if record.PK != "" || record.SK != "" {
		t.Fatalf("keys should be created by the storage client, got PK=%q SK=%q", record.PK, record.SK)
	}
	if restored.Step != StepConfirmRedivide || restored.ExpenseShortID != "a1b2c3d4" || restored.ParticipantOptions[2] != 20 || len(restored.SelectedParticipantIDs) != 1 || restored.SelectedParticipantIDs[0] != 10 {
		t.Fatalf("conversation round trip lost state: %#v", restored)
	}
}

func TestNaturalExpenseArgs(t *testing.T) {
	tests := []struct {
		input string
		want  []string
		ok    bool
	}{
		{input: "@Splitter cargue la cena 15000", want: []string{"cena", "15000"}, ok: true},
		{input: "cargué un regalo 15.000", want: []string{"regalo", "15000"}, ok: true},
		{input: "registré nafta $12.500,50", want: []string{"nafta", "12500.5"}, ok: true},
		{input: "la cena salió 15000", ok: false},
	}

	for _, test := range tests {
		got, ok := naturalExpenseArgs(test.input)
		if ok != test.ok || len(got) != len(test.want) {
			t.Fatalf("naturalExpenseArgs(%q) = (%v, %t), want (%v, %t)", test.input, got, ok, test.want, test.ok)
		}
		for i := range got {
			if got[i] != test.want[i] {
				t.Fatalf("naturalExpenseArgs(%q) = %v, want %v", test.input, got, test.want)
			}
		}
	}
}

func TestSelectSplitParticipants(t *testing.T) {
	members := []db.Member{
		{UserID: 10, DisplayName: "Agustin Bettucci"},
		{UserID: 20, DisplayName: "Cecilia"},
		{UserID: 30, DisplayName: "Lauti"},
	}
	options := map[int]int64{1: 10, 2: 20, 3: 30}

	selected, invalid := selectSplitParticipants("1, 3", options, members)
	if invalid != "" || len(selected) != 2 || selected[0].UserID != 10 || selected[1].UserID != 30 {
		t.Fatalf("numeric selection = (%v, %q), want Agustin and Lauti", selected, invalid)
	}

	selected, invalid = selectSplitParticipants("Cecilia", options, members)
	if invalid != "" || len(selected) != 1 || selected[0].UserID != 20 {
		t.Fatalf("name selection = (%v, %q), want Cecilia", selected, invalid)
	}

	_, invalid = selectSplitParticipants("4", options, members)
	if invalid != "4" {
		t.Fatalf("invalid selection = %q, want 4", invalid)
	}
}
