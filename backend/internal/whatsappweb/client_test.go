package whatsappweb

import (
	"encoding/json"
	"testing"
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
