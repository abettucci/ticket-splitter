package bot

import (
	"fmt"
	"sync"
	"time"
)

// ConversationStep representa el paso actual de una conversación en curso
type ConversationStep int

const (
	StepNone ConversationStep = iota
	StepNewExpenseDescription
	StepNewExpenseAmount
	StepNewExpensePayer
)

// ConversationState guarda el estado de una conversación en curso para un usuario
type ConversationState struct {
	Step        ConversationStep
	Description string
	Amount      float64
	ExpiresAt   time.Time
}

// ConversationManager maneja el estado conversacional por usuario/chat
type ConversationManager struct {
	mu       sync.RWMutex
	sessions map[string]*ConversationState
}

// NewConversationManager crea un nuevo manager de conversaciones
func NewConversationManager() *ConversationManager {
	return &ConversationManager{
		sessions: make(map[string]*ConversationState),
	}
}

func sessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// Get retorna el estado de conversación activo, o nil si expiró/no existe
func (cm *ConversationManager) Get(chatID, userID int64) *ConversationState {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	state, ok := cm.sessions[sessionKey(chatID, userID)]
	if !ok || time.Now().After(state.ExpiresAt) {
		return nil
	}
	return state
}

// Set guarda o actualiza el estado de conversación con TTL de 5 minutos
func (cm *ConversationManager) Set(chatID, userID int64, state *ConversationState) {
	state.ExpiresAt = time.Now().Add(5 * time.Minute)
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.sessions[sessionKey(chatID, userID)] = state
}

// Clear elimina el estado de conversación de un usuario
func (cm *ConversationManager) Clear(chatID, userID int64) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.sessions, sessionKey(chatID, userID))
}
