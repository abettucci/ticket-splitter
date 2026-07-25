package whatsapp

// WebhookPayload es el payload que manda Meta al webhook
type WebhookPayload struct {
	Object string  `json:"object"`
	Entry  []Entry `json:"entry"`
}

type Entry struct {
	ID      string   `json:"id"`
	Changes []Change `json:"changes"`
}

type Change struct {
	Value Value  `json:"value"`
	Field string `json:"field"`
}

type Value struct {
	MessagingProduct string    `json:"messaging_product"`
	Metadata         Metadata  `json:"metadata"`
	Messages         []Message `json:"messages"`
	Contacts         []Contact `json:"contacts"`
}

type Metadata struct {
	DisplayPhoneNumber string `json:"display_phone_number"`
	PhoneNumberID      string `json:"phone_number_id"`
}

type Message struct {
	From      string   `json:"from"`      // número del remitente
	ID        string   `json:"id"`        // wamid
	Timestamp string   `json:"timestamp"`
	Type      string   `json:"type"`      // "text", "image", etc.
	Text      *Text    `json:"text,omitempty"`
}

type Text struct {
	Body string `json:"body"`
}

type Contact struct {
	WaID    string  `json:"wa_id"`
	Profile Profile `json:"profile"`
}

type Profile struct {
	Name string `json:"name"`
}
