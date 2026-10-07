package whatsappweb

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/abettucci/group-split-bot/internal/telegram"
)

func TestInboundPayloadKeepsLargeGroupIDsExact(t *testing.T) {
	const groupID = "120363416683339098"

	var payload InboundPayload
	if err := json.Unmarshal([]byte(`{"chat_id":"`+groupID+`","sender_id":"11897126555748"}`), &payload); err != nil {
		t.Fatalf("unmarshal payload: %v", err)
	}

	if string(payload.ChatID) != groupID {
		t.Fatalf("chat id = %q, want %q", payload.ChatID, groupID)
	}
}

func TestSendMessageWithOptionsKeepsFifteenthMenuOption(t *testing.T) {
	var received sendRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &received); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	buttons := make([][]telegram.InlineKeyboardButton, 0, 15)
	for i := 1; i <= 15; i++ {
		buttons = append(buttons, []telegram.InlineKeyboardButton{{Text: "Opción " + string(rune('A'-1+i)), CallbackData: "menu:test"}})
	}
	client := &Client{httpClient: server.Client(), sidecarURL: server.URL, sharedSecret: "test-secret"}
	err := client.SendMessageWithOptions(context.Background(), &telegram.SendMessageRequest{
		ChatID: 1,
		Text:   "Menú",
		ReplyMarkup: telegram.InlineKeyboardMarkup{
			InlineKeyboard: buttons,
		},
	})
	if err != nil {
		t.Fatalf("SendMessageWithOptions: %v", err)
	}
	if !strings.Contains(received.Text, "15. Opción O") {
		t.Fatalf("fifteenth menu option is missing from WhatsApp text menu: %q", received.Text)
	}
	if received.Interactive == nil || len(received.Interactive.Options) != 15 {
		t.Fatalf("interactive fallback options = %#v, want all 15 options", received.Interactive)
	}
}
