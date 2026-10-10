package bot

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/abettucci/group-split-bot/internal/db"
)

// ConversationStep representa el paso actual de una conversación en curso
type ConversationStep int

const (
	StepNone ConversationStep = iota
	StepNewExpenseDescription
	StepNewExpenseAmount
	StepNewExpensePayer
	StepSelectNewExpenseAction // dividir o ver el gasto recién creado
	StepSelectExpenseSplitMode // entre todos o sólo participantes elegidos
	StepSelectExpenseParticipants
	StepSelectRedivideMode         // cómo reemplazar una división existente
	StepSelectRedivideParticipants // nuevos deudores de una división existente
	StepConfirmRedivide            // confirmación antes de reemplazar deudas y pagos
	StepSelectMenuOption           // selección del menú numerado de WhatsApp
	StepSelectDivideExpense        // selección de gasto a dividir (para WhatsApp sin botones)
	StepSelectRedivideExpense      // selección de gasto a redividir (para WhatsApp sin botones)
	StepSelectPaymentExpense       // selección de una deuda propia para marcar como pagada
	StepExpenseSummary             // espera una lista libre de gastos/reintegros para resumirla sin guardarla
)

// ConversationState guarda el estado de una conversación en curso para un usuario
type ConversationState struct {
	Step                   ConversationStep
	Description            string
	Amount                 float64
	ExpenseShortID         string
	ExpiresAt              time.Time
	DivideOptions          map[int]string // índice (1-based) -> shortID, usado en los selectores de gastos
	ParticipantOptions     map[int]int64  // índice (1-based) -> userID, usado para una división selectiva
	SelectedParticipantIDs []int64        // user IDs listos para confirmar una redivisión
}

// ConversationManager maneja el estado conversacional por usuario/chat
type ConversationManager struct {
	mu       sync.RWMutex
	sessions map[string]*ConversationState
	store    *db.Client
}

// NewConversationManager crea un nuevo manager de conversaciones
func NewConversationManager(store *db.Client) *ConversationManager {
	return &ConversationManager{
		sessions: make(map[string]*ConversationState),
		store:    store,
	}
}

func sessionKey(chatID, userID int64) string {
	return fmt.Sprintf("%d:%d", chatID, userID)
}

// Get retorna el estado de conversación activo, o nil si expiró/no existe
func (cm *ConversationManager) Get(chatID, userID int64) *ConversationState {
	if cm.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conversation, err := cm.store.GetConversation(ctx, chatID, userID)
		if err == nil && conversation != nil {
			state := conversationStateFromRecord(conversation)
			cm.mu.Lock()
			cm.sessions[sessionKey(chatID, userID)] = state
			cm.mu.Unlock()
			return state
		}
		if err == nil {
			return nil
		}
		// A warmed Lambda can still serve the active flow if DynamoDB has a
		// transient issue. A cold instance remains fail-closed rather than
		// treating an ordinary group message as a command.
	}

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
	cm.sessions[sessionKey(chatID, userID)] = state
	cm.mu.Unlock()

	if cm.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cm.store.SaveConversation(ctx, conversationRecordFromState(chatID, userID, state))
	}
}

// Clear elimina el estado de conversación de un usuario
func (cm *ConversationManager) Clear(chatID, userID int64) {
	cm.mu.Lock()
	delete(cm.sessions, sessionKey(chatID, userID))
	cm.mu.Unlock()

	if cm.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = cm.store.DeleteConversation(ctx, chatID, userID)
	}
}

func conversationRecordFromState(chatID, userID int64, state *ConversationState) *db.Conversation {
	divideOptions := make(map[string]string, len(state.DivideOptions))
	for index, shortID := range state.DivideOptions {
		divideOptions[fmt.Sprintf("%d", index)] = shortID
	}
	participantOptions := make(map[string]int64, len(state.ParticipantOptions))
	for index, memberID := range state.ParticipantOptions {
		participantOptions[fmt.Sprintf("%d", index)] = memberID
	}
	return &db.Conversation{
		ChatID:                 chatID,
		UserID:                 userID,
		Step:                   int(state.Step),
		Description:            state.Description,
		Amount:                 state.Amount,
		ExpenseShortID:         state.ExpenseShortID,
		DivideOptions:          divideOptions,
		ParticipantOptions:     participantOptions,
		SelectedParticipantIDs: state.SelectedParticipantIDs,
		ExpiresAt:              state.ExpiresAt,
	}
}

func conversationStateFromRecord(conversation *db.Conversation) *ConversationState {
	divideOptions := make(map[int]string, len(conversation.DivideOptions))
	for rawIndex, shortID := range conversation.DivideOptions {
		if index, err := strconv.Atoi(rawIndex); err == nil {
			divideOptions[index] = shortID
		}
	}
	participantOptions := make(map[int]int64, len(conversation.ParticipantOptions))
	for rawIndex, memberID := range conversation.ParticipantOptions {
		if index, err := strconv.Atoi(rawIndex); err == nil {
			participantOptions[index] = memberID
		}
	}
	return &ConversationState{
		Step:                   ConversationStep(conversation.Step),
		Description:            conversation.Description,
		Amount:                 conversation.Amount,
		ExpenseShortID:         conversation.ExpenseShortID,
		ExpiresAt:              conversation.ExpiresAt,
		DivideOptions:          divideOptions,
		ParticipantOptions:     participantOptions,
		SelectedParticipantIDs: conversation.SelectedParticipantIDs,
	}
}
