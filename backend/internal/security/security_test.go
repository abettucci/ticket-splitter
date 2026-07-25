package security

import (
	"testing"
)

func TestSanitizeInput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "Cena con amigos",
			expected: "Cena con amigos",
		},
		{
			name:     "script tag removal",
			input:    "Cena <script>alert('xss')</script> amigos",
			expected: "Cena alert('xss') amigos",
		},
		{
			name:     "sql injection attempt",
			input:    "Cena'; DROP TABLE expenses;--",
			expected: "Cena'; expenses;--",
		},
		{
			name:     "path traversal",
			input:    "../../../etc/passwd",
			expected: "etc/passwd",
		},
		{
			name:     "javascript protocol",
			input:    "javascript:alert(1)",
			expected: "alert(1)",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "whitespace only",
			input:    "   ",
			expected: "",
		},
		{
			name:     "spanish characters",
			input:    "Almuerzo en café con María",
			expected: "Almuerzo en café con María",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SanitizeInput(tt.input)
			if result != tt.expected {
				t.Errorf("SanitizeInput(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestValidateDescription(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantValid   bool
		wantNonEmpty bool
	}{
		{
			name:        "valid description",
			input:       "Cena en restaurante",
			wantValid:   true,
			wantNonEmpty: true,
		},
		{
			name:        "empty description",
			input:       "",
			wantValid:   false,
			wantNonEmpty: false,
		},
		{
			name:        "description with numbers",
			input:       "Nafta 50 litros",
			wantValid:   true,
			wantNonEmpty: true,
		},
		{
			name:        "description with special chars",
			input:       "Super mercado (compras)",
			wantValid:   true,
			wantNonEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, valid := ValidateDescription(tt.input)
			if valid != tt.wantValid {
				t.Errorf("ValidateDescription(%q) valid = %v, want %v", tt.input, valid, tt.wantValid)
			}
			if tt.wantNonEmpty && len(result) == 0 {
				t.Errorf("ValidateDescription(%q) returned empty string", tt.input)
			}
		})
	}
}

func TestValidateAmount(t *testing.T) {
	tests := []struct {
		name   string
		amount float64
		want   bool
	}{
		{"valid amount", 100.50, true},
		{"minimum amount", 0.01, true},
		{"zero amount", 0, false},
		{"negative amount", -100, false},
		{"very large amount", 1000000000, false},
		{"max valid amount", 999999999.99, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAmount(tt.amount); got != tt.want {
				t.Errorf("ValidateAmount(%v) = %v, want %v", tt.amount, got, tt.want)
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	tests := []struct {
		name    string
		command string
		want    bool
	}{
		{"start command", "/start", true},
		{"help command", "/help", true},
		{"new expense", "/nuevo_gasto Cena 100", true},
		{"view expenses", "/ver_gastos", true},
		{"invalid command", "/invalid_cmd", false},
		{"not a command", "hello", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateCommand(tt.command); got != tt.want {
				t.Errorf("ValidateCommand(%q) = %v, want %v", tt.command, got, tt.want)
			}
		})
	}
}

func TestCheckRateLimit(t *testing.T) {
	userID := int64(123456)

	// First requests should pass
	for i := 0; i < RateLimitMaxRequests; i++ {
		if !CheckRateLimit(userID) {
			t.Errorf("Request %d should have passed rate limit", i+1)
		}
	}

	// Next request should fail
	if CheckRateLimit(userID) {
		t.Error("Request should have been rate limited")
	}

	// Different user should pass
	if !CheckRateLimit(int64(654321)) {
		t.Error("Different user should not be rate limited")
	}
}

func TestMaskSensitiveData(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"short string", "abc", "****"},
		{"normal string", "123456789", "12*****89"},
		{"token-like", "abcdefghij", "ab******ij"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MaskSensitiveData(tt.input); got != tt.want {
				t.Errorf("MaskSensitiveData(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestValidatePhoneNumber(t *testing.T) {
	tests := []struct {
		name  string
		phone string
		want  bool
	}{
		{"valid with plus", "+5491123456789", true},
		{"valid without plus", "5491123456789", true},
		{"too short", "12345", false},
		{"too long", "12345678901234567890", false},
		{"with spaces", "+54 911 2345 6789", true},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidatePhoneNumber(tt.phone); got != tt.want {
				t.Errorf("ValidatePhoneNumber(%q) = %v, want %v", tt.phone, got, tt.want)
			}
		})
	}
}

