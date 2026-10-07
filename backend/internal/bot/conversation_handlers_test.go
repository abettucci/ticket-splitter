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
