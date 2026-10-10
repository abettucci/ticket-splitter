package bot

import "testing"

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
		{name: "free expense description is not accepted", state: &ConversationState{Step: StepNewExpenseDescription}, text: "nafta y seguro", want: false},
		{name: "ordinary chat is not accepted", state: &ConversationState{Step: StepSelectMenuOption}, text: "che llego en diez", want: false},
		{name: "no active flow", state: nil, text: "1", want: false},
	}

	for _, test := range tests {
		if got := isSafeWhatsAppGroupContinuation(test.state, test.text); got != test.want {
			t.Fatalf("%s: isSafeWhatsAppGroupContinuation(%q) = %t, want %t", test.name, test.text, got, test.want)
		}
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
