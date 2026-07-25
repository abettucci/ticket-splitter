package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

const (
	// Table names
	TableGroups   = "splitbot_groups"
	TableMembers  = "splitbot_members"
	TableExpenses = "splitbot_expenses"
	TableSplits   = "splitbot_splits"
	TableCommands = "splitbot_commands"

	// GSI names
	GSIGroupExpenses = "group-expenses-index"
	GSIGroupMembers  = "group-members-index"
	GSIUserSplits    = "user-splits-index"
)

// Client cliente de DynamoDB
type Client struct {
	db        *dynamodb.Client
	tableName string
}

// Group representa un grupo de Telegram
type Group struct {
	PK        string    `dynamodbav:"PK"` // GROUP#<chat_id>
	SK        string    `dynamodbav:"SK"` // METADATA
	ID        string    `dynamodbav:"id"`
	ChatID    int64     `dynamodbav:"chat_id"`
	Title     string    `dynamodbav:"title"`
	Type      string    `dynamodbav:"type"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	UpdatedAt time.Time `dynamodbav:"updated_at"`
	IsActive  bool      `dynamodbav:"is_active"`
}

// Member representa un miembro del grupo
type Member struct {
	PK          string    `dynamodbav:"PK"` // GROUP#<chat_id>
	SK          string    `dynamodbav:"SK"` // MEMBER#<user_id>
	ID          string    `dynamodbav:"id"`
	UserID      int64     `dynamodbav:"user_id"`
	ChatID      int64     `dynamodbav:"chat_id"`
	DisplayName string    `dynamodbav:"display_name"`
	Username    string    `dynamodbav:"username,omitempty"`
	JoinedAt    time.Time `dynamodbav:"joined_at"`
	IsActive    bool      `dynamodbav:"is_active"`
}

// Expense representa un gasto
type Expense struct {
	PK          string        `dynamodbav:"PK"` // GROUP#<chat_id>
	SK          string        `dynamodbav:"SK"` // EXPENSE#<expense_id>
	ID          string        `dynamodbav:"id"`
	ChatID      int64         `dynamodbav:"chat_id"`
	Description string        `dynamodbav:"description"`
	TotalAmount float64       `dynamodbav:"total_amount"`
	Currency    string        `dynamodbav:"currency"`
	CreatedBy   int64         `dynamodbav:"created_by"`
	CreatorName string        `dynamodbav:"creator_name"`
	CreatedAt   time.Time     `dynamodbav:"created_at"`
	UpdatedAt   time.Time     `dynamodbav:"updated_at,omitempty"`
	IsDivided   bool          `dynamodbav:"is_divided"`
	IsDeleted   bool          `dynamodbav:"is_deleted"`
	DeletedAt   time.Time     `dynamodbav:"deleted_at,omitempty"` // Cuándo fue eliminado
	DeletedBy   int64         `dynamodbav:"deleted_by,omitempty"` // Quién lo eliminó
	TTL         int64         `dynamodbav:"ttl,omitempty"`        // TTL para borrado permanente (30 días)
	Category    string        `dynamodbav:"category,omitempty"`
	Notes       string        `dynamodbav:"notes,omitempty"`
	Items       []ExpenseItem `dynamodbav:"items,omitempty"`
	Payers      []Payer       `dynamodbav:"payers,omitempty"`
	SplitType   string        `dynamodbav:"split_type"` // "equal", "exact", "percentage", "items"
}

// ExpenseItem representa un item individual en un gasto
type ExpenseItem struct {
	Description string  `dynamodbav:"description"`
	Amount      float64 `dynamodbav:"amount"`
	PaidBy      int64   `dynamodbav:"paid_by"`
	SharedBy    []int64 `dynamodbav:"shared_by"` // User IDs que comparten este item
}

// Payer representa quién pagó el gasto (puede ser más de uno)
type Payer struct {
	UserID   int64   `dynamodbav:"user_id"`
	UserName string  `dynamodbav:"user_name"`
	Amount   float64 `dynamodbav:"amount"`
}

// ExpenseSplit representa la división de un gasto
type ExpenseSplit struct {
	PK         string    `dynamodbav:"PK"` // EXPENSE#<expense_id>
	SK         string    `dynamodbav:"SK"` // SPLIT#<user_id>
	ID         string    `dynamodbav:"id"`
	ExpenseID  string    `dynamodbav:"expense_id"`
	UserID     int64     `dynamodbav:"user_id"`
	UserName   string    `dynamodbav:"user_name"`
	Amount     float64   `dynamodbav:"amount"`
	Percentage float64   `dynamodbav:"percentage,omitempty"` // % del gasto (si aplica)
	IsPaid     bool      `dynamodbav:"is_paid"`
	PaidAt     time.Time `dynamodbav:"paid_at,omitempty"`
	CreatedAt  time.Time `dynamodbav:"created_at"`
	OwedTo     int64     `dynamodbav:"owed_to,omitempty"` // A quién le debe (para optimización)
	// GSI para buscar por usuario
	GSI1PK string `dynamodbav:"GSI1PK"` // USER#<user_id>
	GSI1SK string `dynamodbav:"GSI1SK"` // SPLIT#<expense_id>
}

// BotCommand representa un comando ejecutado (para auditoría)
type BotCommand struct {
	PK        string                 `dynamodbav:"PK"` // GROUP#<chat_id>
	SK        string                 `dynamodbav:"SK"` // CMD#<timestamp>#<id>
	ID        string                 `dynamodbav:"id"`
	ChatID    int64                  `dynamodbav:"chat_id"`
	UserID    int64                  `dynamodbav:"user_id"`
	Command   string                 `dynamodbav:"command"`
	Params    map[string]interface{} `dynamodbav:"params,omitempty"`
	CreatedAt time.Time              `dynamodbav:"created_at"`
}

// Reminder representa un recordatorio de pago
type Reminder struct {
	PK                 string  `dynamodbav:"PK"` // USER#<user_id>
	SK                 string  `dynamodbav:"SK"` // REMINDER#<reminder_id>
	ID                 string  `dynamodbav:"id"`
	UserID             int64   `dynamodbav:"user_id"`
	ChatID             int64   `dynamodbav:"chat_id,omitempty"` // Para futuros pagos grupales
	Description        string  `dynamodbav:"description"`
	Amount             float64 `dynamodbav:"amount"`
	PayeeName          string  `dynamodbav:"payee_name"` // A quién le debe
	PayeeUsername      string  `dynamodbav:"payee_username,omitempty"`
	PayeeUserID        int64   `dynamodbav:"payee_user_id,omitempty"` // Para tracking
	ReminderType       string  `dynamodbav:"reminder_type"`           // "payment" (debo pagar) o "collection" (debo cobrar)
	TotalInstallments  int     `dynamodbav:"total_installments"`      // Total de cuotas
	CurrentInstallment int     `dynamodbav:"current_installment"`     // Cuota actual
	// Recurrencia básica (legacy)
	Frequency string `dynamodbav:"frequency,omitempty"` // "monthly", "weekly", "daily", "once"
	// Recurrencia avanzada (estilo Google Calendar)
	RecurrenceRule        string    `dynamodbav:"recurrence_rule,omitempty"`          // Regla de recurrencia
	RecurrenceInterval    int       `dynamodbav:"recurrence_interval,omitempty"`      // Ej: cada 2 semanas
	RecurrenceDayOfWeek   int       `dynamodbav:"recurrence_day_of_week,omitempty"`   // 0=domingo, 1=lunes, etc
	RecurrenceDayOfMonth  int       `dynamodbav:"recurrence_day_of_month,omitempty"`  // Día del mes (1-31)
	RecurrenceWeekOfMonth int       `dynamodbav:"recurrence_week_of_month,omitempty"` // 1=primer, 2=segundo, -1=último
	NextReminderDate      time.Time `dynamodbav:"next_reminder_date"`
	StartDate             time.Time `dynamodbav:"start_date"`
	EndDate               time.Time `dynamodbav:"end_date,omitempty"`
	IsActive              bool      `dynamodbav:"is_active"`
	IsRecurring           bool      `dynamodbav:"is_recurring"`
	CreatedAt             time.Time `dynamodbav:"created_at"`
	LastNotifiedAt        time.Time `dynamodbav:"last_notified_at,omitempty"`
	// GSI para buscar recordatorios pendientes por fecha
	GSI2PK string `dynamodbav:"GSI2PK"` // REMINDER#PENDING
	GSI2SK string `dynamodbav:"GSI2SK"` // DATE#<next_reminder_date>
}

// NewClient crea un nuevo cliente de DynamoDB
func NewClient(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	tableName := os.Getenv("DYNAMODB_TABLE")
	if tableName == "" {
		tableName = "splitbot"
	}

	return &Client{
		db:        dynamodb.NewFromConfig(cfg),
		tableName: tableName,
	}, nil
}

// ============================================
// GROUP OPERATIONS
// ============================================

// GetOrCreateGroup obtiene o crea un grupo
func (c *Client) GetOrCreateGroup(ctx context.Context, chatID int64, title, chatType string) (*Group, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)
	sk := "METADATA"

	// Intentar obtener el grupo existente
	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get group: %w", err)
	}

	if result.Item != nil {
		var group Group
		if err := attributevalue.UnmarshalMap(result.Item, &group); err != nil {
			return nil, fmt.Errorf("failed to unmarshal group: %w", err)
		}
		return &group, nil
	}

	// Crear nuevo grupo
	now := time.Now()
	group := &Group{
		PK:        pk,
		SK:        sk,
		ID:        uuid.New().String(),
		ChatID:    chatID,
		Title:     title,
		Type:      chatType,
		CreatedAt: now,
		UpdatedAt: now,
		IsActive:  true,
	}

	item, err := attributevalue.MarshalMap(group)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal group: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create group: %w", err)
	}

	return group, nil
}

// ============================================
// MEMBER OPERATIONS
// ============================================

// AddMember añade un miembro al grupo
func (c *Client) AddMember(ctx context.Context, chatID, userID int64, displayName, username string) (*Member, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)
	sk := fmt.Sprintf("MEMBER#%d", userID)

	now := time.Now()
	member := &Member{
		PK:          pk,
		SK:          sk,
		ID:          uuid.New().String(),
		UserID:      userID,
		ChatID:      chatID,
		DisplayName: displayName,
		Username:    username,
		JoinedAt:    now,
		IsActive:    true,
	}

	item, err := attributevalue.MarshalMap(member)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal member: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	return member, nil
}

// RemoveMember elimina un miembro del grupo (soft delete)
func (c *Client) RemoveMember(ctx context.Context, chatID, userID int64) error {
	pk := fmt.Sprintf("GROUP#%d", chatID)
	sk := fmt.Sprintf("MEMBER#%d", userID)

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET is_active = :inactive"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inactive": &types.AttributeValueMemberBOOL{Value: false},
		},
	})

	return err
}

// GetGroupMembers obtiene todos los miembros de un grupo
func (c *Client) GetGroupMembers(ctx context.Context, chatID int64) ([]Member, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)

	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: "MEMBER#"},
		},
		FilterExpression:         aws.String("is_active = :active"),
		ExpressionAttributeNames: map[string]string{},
	})
	// Corregir el filtro
	result, err = c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":sk":     &types.AttributeValueMemberS{Value: "MEMBER#"},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
		FilterExpression: aws.String("is_active = :active"),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query members: %w", err)
	}

	var members []Member
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &members); err != nil {
		return nil, fmt.Errorf("failed to unmarshal members: %w", err)
	}

	return members, nil
}

// ============================================
// EXPENSE OPERATIONS
// ============================================

// CreateExpense crea un nuevo gasto
func (c *Client) CreateExpense(ctx context.Context, chatID int64, description string, amount float64, createdBy int64, creatorName string) (*Expense, error) {
	expenseID := uuid.New().String()
	pk := fmt.Sprintf("GROUP#%d", chatID)
	sk := fmt.Sprintf("EXPENSE#%s", expenseID)

	expense := &Expense{
		PK:          pk,
		SK:          sk,
		ID:          expenseID,
		ChatID:      chatID,
		Description: description,
		TotalAmount: amount,
		Currency:    "ARS",
		CreatedBy:   createdBy,
		CreatorName: creatorName,
		CreatedAt:   time.Now(),
		IsDivided:   false,
		IsDeleted:   false,
		SplitType:   "equal",
	}

	item, err := attributevalue.MarshalMap(expense)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal expense: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create expense: %w", err)
	}

	return expense, nil
}

// UpdateExpense actualiza un gasto existente
func (c *Client) UpdateExpense(ctx context.Context, expense *Expense) error {
	expense.UpdatedAt = time.Now()

	item, err := attributevalue.MarshalMap(expense)
	if err != nil {
		return fmt.Errorf("failed to marshal expense: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})

	return err
}

// DeleteExpense marca un gasto como eliminado (soft delete)
// Guarda quién lo eliminó y cuándo para poder restaurarlo después
// También elimina los splits asociados para evitar deudas huérfanas
func (c *Client) DeleteExpense(ctx context.Context, expenseID, pk, sk string, deletedBy int64) error {
	now := time.Now()
	// TTL: 30 días después de eliminar, se borra permanentemente
	ttlValue := now.AddDate(0, 0, 30).Unix()

	// Primero eliminar los splits asociados
	if err := c.DeleteExpenseSplits(ctx, expenseID); err != nil {
		// Log pero no fallar - el gasto igual se puede eliminar
		// Los splits huérfanos se limpiarán después
	}

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET is_deleted = :deleted, updated_at = :now, deleted_at = :now, deleted_by = :deleted_by, #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl", // ttl es palabra reservada en DynamoDB
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":deleted":    &types.AttributeValueMemberBOOL{Value: true},
			":now":        &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
			":deleted_by": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", deletedBy)},
			":ttl":        &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttlValue)},
		},
	})

	return err
}

// RestoreExpense restaura un gasto eliminado
func (c *Client) RestoreExpense(ctx context.Context, pk, sk string) error {
	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET is_deleted = :deleted, updated_at = :now REMOVE deleted_at, deleted_by, #ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":deleted": &types.AttributeValueMemberBOOL{Value: false},
			":now":     &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return err
}

// GetExpenseByShortIDIncludeDeleted busca un gasto por ID corto incluyendo eliminados
func (c *Client) GetExpenseByShortIDIncludeDeleted(ctx context.Context, chatID int64, shortID string) (*Expense, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)

	// Consulta sin filtro de is_deleted
	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: "EXPENSE#"},
		},
		Limit: aws.Int32(200),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}

	var expenses []Expense
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &expenses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expenses: %w", err)
	}

	for _, expense := range expenses {
		if len(expense.ID) >= len(shortID) && expense.ID[:len(shortID)] == shortID {
			return &expense, nil
		}
	}

	return nil, fmt.Errorf("expense not found")
}

// GetGroupExpenses obtiene los gastos activos de un grupo (excluye eliminados)
func (c *Client) GetGroupExpenses(ctx context.Context, chatID int64, limit int32) ([]Expense, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)

	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("is_deleted = :deleted OR attribute_not_exists(is_deleted)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":      &types.AttributeValueMemberS{Value: pk},
			":sk":      &types.AttributeValueMemberS{Value: "EXPENSE#"},
			":deleted": &types.AttributeValueMemberBOOL{Value: false},
		},
		ScanIndexForward: aws.Bool(false), // Más recientes primero
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query expenses: %w", err)
	}

	var expenses []Expense
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &expenses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expenses: %w", err)
	}

	return expenses, nil
}

// GetDeletedExpenses obtiene los gastos eliminados de un grupo (papelera)
func (c *Client) GetDeletedExpenses(ctx context.Context, chatID int64, limit int32) ([]Expense, error) {
	pk := fmt.Sprintf("GROUP#%d", chatID)

	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("is_deleted = :deleted"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":      &types.AttributeValueMemberS{Value: pk},
			":sk":      &types.AttributeValueMemberS{Value: "EXPENSE#"},
			":deleted": &types.AttributeValueMemberBOOL{Value: true},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query deleted expenses: %w", err)
	}

	var expenses []Expense
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &expenses); err != nil {
		return nil, fmt.Errorf("failed to unmarshal expenses: %w", err)
	}

	return expenses, nil
}

// GetExpenseByShortID busca un gasto por ID corto
func (c *Client) GetExpenseByShortID(ctx context.Context, chatID int64, shortID string) (*Expense, error) {
	expenses, err := c.GetGroupExpenses(ctx, chatID, 100)
	if err != nil {
		return nil, err
	}

	for _, expense := range expenses {
		if len(expense.ID) >= len(shortID) && expense.ID[:len(shortID)] == shortID {
			return &expense, nil
		}
	}

	return nil, fmt.Errorf("expense not found")
}

// ============================================
// SPLIT OPERATIONS
// ============================================

// CreateSplits crea las divisiones de un gasto (equitativo)
func (c *Client) CreateSplits(ctx context.Context, expense *Expense, members []Member) error {
	if len(members) == 0 {
		return fmt.Errorf("no members to split")
	}

	amountPerPerson := expense.TotalAmount / float64(len(members))
	now := time.Now()

	for _, member := range members {
		split := &ExpenseSplit{
			PK:        fmt.Sprintf("EXPENSE#%s", expense.ID),
			SK:        fmt.Sprintf("SPLIT#%d", member.UserID),
			ID:        uuid.New().String(),
			ExpenseID: expense.ID,
			UserID:    member.UserID,
			UserName:  member.DisplayName,
			Amount:    amountPerPerson,
			IsPaid:    member.UserID == expense.CreatedBy,
			CreatedAt: now,
			GSI1PK:    fmt.Sprintf("USER#%d", member.UserID),
			GSI1SK:    fmt.Sprintf("SPLIT#%s", expense.ID),
		}

		if split.IsPaid {
			split.PaidAt = now
		}

		item, err := attributevalue.MarshalMap(split)
		if err != nil {
			return fmt.Errorf("failed to marshal split: %w", err)
		}

		_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(c.tableName),
			Item:      item,
		})
		if err != nil {
			return fmt.Errorf("failed to create split: %w", err)
		}
	}

	// Marcar el gasto como dividido
	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: expense.PK},
			"SK": &types.AttributeValueMemberS{Value: expense.SK},
		},
		UpdateExpression: aws.String("SET is_divided = :divided"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":divided": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	return err
}

// CreateSplitsWithCustomAmounts crea divisiones con montos personalizados
func (c *Client) CreateSplitsWithCustomAmounts(ctx context.Context, expense *Expense, customSplits map[int64]float64, payers []Payer) error {
	now := time.Now()

	for userID, amount := range customSplits {
		isPaid := false
		for _, payer := range payers {
			if payer.UserID == userID {
				isPaid = true
				break
			}
		}

		split := &ExpenseSplit{
			PK:        fmt.Sprintf("EXPENSE#%s", expense.ID),
			SK:        fmt.Sprintf("SPLIT#%d", userID),
			ID:        uuid.New().String(),
			ExpenseID: expense.ID,
			UserID:    userID,
			Amount:    amount,
			IsPaid:    isPaid,
			CreatedAt: now,
			GSI1PK:    fmt.Sprintf("USER#%d", userID),
			GSI1SK:    fmt.Sprintf("SPLIT#%s", expense.ID),
		}

		if split.IsPaid {
			split.PaidAt = now
		}

		item, err := attributevalue.MarshalMap(split)
		if err != nil {
			return fmt.Errorf("failed to marshal split: %w", err)
		}

		_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
			TableName: aws.String(c.tableName),
			Item:      item,
		})
		if err != nil {
			return fmt.Errorf("failed to create split: %w", err)
		}
	}

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: expense.PK},
			"SK": &types.AttributeValueMemberS{Value: expense.SK},
		},
		UpdateExpression: aws.String("SET is_divided = :divided"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":divided": &types.AttributeValueMemberBOOL{Value: true},
		},
	})

	return err
}

// CreateSplitsFromItems crea divisiones desde items individuales
func (c *Client) CreateSplitsFromItems(ctx context.Context, expense *Expense) error {
	if len(expense.Items) == 0 {
		return fmt.Errorf("no items to split")
	}

	splitAmounts := make(map[int64]float64)

	for _, item := range expense.Items {
		if len(item.SharedBy) == 0 {
			continue
		}
		amountPerPerson := item.Amount / float64(len(item.SharedBy))
		for _, userID := range item.SharedBy {
			splitAmounts[userID] += amountPerPerson
		}
	}

	return c.CreateSplitsWithCustomAmounts(ctx, expense, splitAmounts, expense.Payers)
}

// GetExpenseSplits obtiene las divisiones de un gasto
func (c *Client) GetExpenseSplits(ctx context.Context, expenseID string) ([]ExpenseSplit, error) {
	pk := fmt.Sprintf("EXPENSE#%s", expenseID)

	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
			":sk": &types.AttributeValueMemberS{Value: "SPLIT#"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query splits: %w", err)
	}

	var splits []ExpenseSplit
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &splits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal splits: %w", err)
	}

	return splits, nil
}

// GetUserPendingSplits obtiene las deudas pendientes de un usuario
func (c *Client) GetUserPendingSplits(ctx context.Context, userID int64) ([]ExpenseSplit, error) {
	// Necesitamos escanear con filtro ya que el GSI necesita configuración adicional
	// En producción, usar el GSI1PK/GSI1SK
	result, err := c.db.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(c.tableName),
		FilterExpression: aws.String("user_id = :uid AND is_paid = :paid AND begins_with(PK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", userID)},
			":paid":   &types.AttributeValueMemberBOOL{Value: false},
			":prefix": &types.AttributeValueMemberS{Value: "EXPENSE#"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan splits: %w", err)
	}

	var splits []ExpenseSplit
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &splits); err != nil {
		return nil, fmt.Errorf("failed to unmarshal splits: %w", err)
	}

	return splits, nil
}

// MarkSplitAsPaid marca una división como pagada
func (c *Client) MarkSplitAsPaid(ctx context.Context, expenseID string, userID int64) error {
	pk := fmt.Sprintf("EXPENSE#%s", expenseID)
	sk := fmt.Sprintf("SPLIT#%d", userID)

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET is_paid = :paid, paid_at = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":paid": &types.AttributeValueMemberBOOL{Value: true},
			":now":  &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})

	return err
}

// UpdateSplitAmount actualiza el monto de un split específico
func (c *Client) UpdateSplitAmount(ctx context.Context, expenseID string, userID int64, newAmount float64) error {
	pk := fmt.Sprintf("EXPENSE#%s", expenseID)
	sk := fmt.Sprintf("SPLIT#%d", userID)

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET amount = :amount"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":amount": &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", newAmount)},
		},
	})

	return err
}

// RecalculateSplitsForExpense recalcula los montos de todos los splits de un gasto
// cuando el monto total cambia (división equitativa)
func (c *Client) RecalculateSplitsForExpense(ctx context.Context, expense *Expense, newTotalAmount float64) error {
	splits, err := c.GetExpenseSplits(ctx, expense.ID)
	if err != nil {
		return fmt.Errorf("failed to get splits: %w", err)
	}

	if len(splits) == 0 {
		return nil // No hay splits que recalcular
	}

	newAmountPerPerson := newTotalAmount / float64(len(splits))

	for _, split := range splits {
		err := c.UpdateSplitAmount(ctx, expense.ID, split.UserID, newAmountPerPerson)
		if err != nil {
			return fmt.Errorf("failed to update split for user %d: %w", split.UserID, err)
		}
	}

	return nil
}

// DeleteExpenseSplits elimina todos los splits de un gasto
func (c *Client) DeleteExpenseSplits(ctx context.Context, expenseID string) error {
	splits, err := c.GetExpenseSplits(ctx, expenseID)
	if err != nil {
		return fmt.Errorf("failed to get splits for expense %s: %w", expenseID, err)
	}

	// Si no hay splits, no hay nada que eliminar (no es un error)
	if len(splits) == 0 {
		return nil
	}

	for _, split := range splits {
		pk := fmt.Sprintf("EXPENSE#%s", expenseID)
		sk := fmt.Sprintf("SPLIT#%d", split.UserID)

		_, err := c.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to delete split for user %d: %w", split.UserID, err)
		}
	}

	return nil
}

// ResetExpenseDivision elimina los splits y marca el gasto como no dividido
func (c *Client) ResetExpenseDivision(ctx context.Context, expense *Expense) error {
	// Validar que tenemos los datos necesarios
	if expense.ID == "" {
		return fmt.Errorf("expense ID is empty")
	}
	if expense.PK == "" || expense.SK == "" {
		return fmt.Errorf("expense PK or SK is empty (PK: %s, SK: %s)", expense.PK, expense.SK)
	}

	// Eliminar splits
	err := c.DeleteExpenseSplits(ctx, expense.ID)
	if err != nil {
		return fmt.Errorf("failed to delete splits: %w", err)
	}

	// Marcar como no dividido
	_, err = c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: expense.PK},
			"SK": &types.AttributeValueMemberS{Value: expense.SK},
		},
		UpdateExpression: aws.String("SET is_divided = :divided, updated_at = :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":divided": &types.AttributeValueMemberBOOL{Value: false},
			":now":     &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update expense: %w", err)
	}

	return nil
}

// UpdateSplitsPayer actualiza el estado de pago cuando cambia el pagador
// El antiguo pagador pasa a deber, el nuevo pagador se marca como pagado
func (c *Client) UpdateSplitsPayer(ctx context.Context, expenseID string, oldPayerID, newPayerID int64) error {
	splits, err := c.GetExpenseSplits(ctx, expenseID)
	if err != nil {
		return fmt.Errorf("failed to get splits: %w", err)
	}

	now := time.Now()

	for _, split := range splits {
		pk := fmt.Sprintf("EXPENSE#%s", expenseID)
		sk := fmt.Sprintf("SPLIT#%d", split.UserID)

		var updateExpr string
		var exprValues map[string]types.AttributeValue

		if split.UserID == oldPayerID {
			// El antiguo pagador ahora debe
			updateExpr = "SET is_paid = :paid, paid_at = :paidAt REMOVE paid_at"
			exprValues = map[string]types.AttributeValue{
				":paid":   &types.AttributeValueMemberBOOL{Value: false},
				":paidAt": &types.AttributeValueMemberS{Value: ""},
			}
			// Usar REMOVE para paid_at
			updateExpr = "SET is_paid = :paid"
			exprValues = map[string]types.AttributeValue{
				":paid": &types.AttributeValueMemberBOOL{Value: false},
			}
		} else if split.UserID == newPayerID {
			// El nuevo pagador ya pagó
			updateExpr = "SET is_paid = :paid, paid_at = :paidAt"
			exprValues = map[string]types.AttributeValue{
				":paid":   &types.AttributeValueMemberBOOL{Value: true},
				":paidAt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
			}
		} else {
			// Otros usuarios no cambian
			continue
		}

		_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: pk},
				"SK": &types.AttributeValueMemberS{Value: sk},
			},
			UpdateExpression:          aws.String(updateExpr),
			ExpressionAttributeValues: exprValues,
		})
		if err != nil {
			return fmt.Errorf("failed to update split for user %d: %w", split.UserID, err)
		}
	}

	return nil
}

// ============================================
// AUDIT OPERATIONS
// ============================================

// LogCommand registra un comando para auditoría
func (c *Client) LogCommand(ctx context.Context, chatID, userID int64, command string, params map[string]interface{}) error {
	now := time.Now()
	cmdLog := &BotCommand{
		PK:        fmt.Sprintf("GROUP#%d", chatID),
		SK:        fmt.Sprintf("CMD#%s#%s", now.Format("2006-01-02T15:04:05"), uuid.New().String()[:8]),
		ID:        uuid.New().String(),
		ChatID:    chatID,
		UserID:    userID,
		Command:   command,
		Params:    params,
		CreatedAt: now,
	}

	item, err := attributevalue.MarshalMap(cmdLog)
	if err != nil {
		return err
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})

	return err
}

// ============================================
// DEBT OPTIMIZATION
// ============================================

// SimplifyDebts optimiza las deudas del grupo (algoritmo greedy)
func (c *Client) SimplifyDebts(ctx context.Context, chatID int64) (map[int64]map[int64]float64, error) {
	members, err := c.GetGroupMembers(ctx, chatID)
	if err != nil {
		return nil, err
	}

	// Calcular balance de cada persona
	balances := make(map[int64]float64)
	memberNames := make(map[int64]string)

	for _, member := range members {
		memberNames[member.UserID] = member.DisplayName
		balances[member.UserID] = 0
	}

	// Obtener todos los splits pendientes
	for _, member := range members {
		splits, err := c.GetUserPendingSplits(ctx, member.UserID)
		if err != nil {
			continue
		}
		for _, split := range splits {
			balances[split.UserID] -= split.Amount // Lo que deben
		}
	}

	// Agregar lo que pagaron
	expenses, err := c.GetGroupExpenses(ctx, chatID, 100)
	if err != nil {
		return nil, err
	}

	for _, expense := range expenses {
		if !expense.IsDivided || expense.IsDeleted {
			continue
		}
		if len(expense.Payers) > 0 {
			for _, payer := range expense.Payers {
				balances[payer.UserID] += payer.Amount
			}
		} else {
			balances[expense.CreatedBy] += expense.TotalAmount
		}
	}

	// Separar deudores y acreedores
	debtors := make([]struct {
		userID int64
		amount float64
	}, 0)
	creditors := make([]struct {
		userID int64
		amount float64
	}, 0)

	for userID, balance := range balances {
		if balance < -0.01 {
			debtors = append(debtors, struct {
				userID int64
				amount float64
			}{userID, -balance})
		} else if balance > 0.01 {
			creditors = append(creditors, struct {
				userID int64
				amount float64
			}{userID, balance})
		}
	}

	// Optimizar transacciones (greedy)
	transactions := make(map[int64]map[int64]float64)

	i, j := 0, 0
	for i < len(debtors) && j < len(creditors) {
		debtor := debtors[i]
		creditor := creditors[j]

		amount := debtor.amount
		if creditor.amount < amount {
			amount = creditor.amount
		}

		if transactions[debtor.userID] == nil {
			transactions[debtor.userID] = make(map[int64]float64)
		}
		transactions[debtor.userID][creditor.userID] = amount

		debtors[i].amount -= amount
		creditors[j].amount -= amount

		if debtors[i].amount < 0.01 {
			i++
		}
		if creditors[j].amount < 0.01 {
			j++
		}
	}

	return transactions, nil
}

// ============================================
// REMINDER OPERATIONS
// ============================================

// CreateReminder crea un nuevo recordatorio
func (c *Client) CreateReminder(ctx context.Context, userID int64, description string, amount float64, payeeName, payeeUsername string, totalInstallments int, frequency string, startDate time.Time, reminderType string) (*Reminder, error) {
	reminderID := uuid.New().String()
	pk := fmt.Sprintf("USER#%d", userID)
	sk := fmt.Sprintf("REMINDER#%s", reminderID)

	// Calcular fecha de fin si es recurrente
	var endDate time.Time
	isRecurring := totalInstallments > 1

	if isRecurring {
		switch frequency {
		case "monthly":
			endDate = startDate.AddDate(0, totalInstallments, 0)
		case "weekly":
			endDate = startDate.AddDate(0, 0, totalInstallments*7)
		case "daily":
			endDate = startDate.AddDate(0, 0, totalInstallments)
		}
	}

	// Default reminder type
	if reminderType == "" {
		reminderType = "payment"
	}

	reminder := &Reminder{
		PK:                 pk,
		SK:                 sk,
		ID:                 reminderID,
		UserID:             userID,
		Description:        description,
		Amount:             amount,
		PayeeName:          payeeName,
		PayeeUsername:      payeeUsername,
		ReminderType:       reminderType,
		TotalInstallments:  totalInstallments,
		CurrentInstallment: 1,
		Frequency:          frequency,
		NextReminderDate:   startDate,
		StartDate:          startDate,
		EndDate:            endDate,
		IsActive:           true,
		IsRecurring:        isRecurring,
		CreatedAt:          time.Now(),
		GSI2PK:             "REMINDER#PENDING",
		GSI2SK:             fmt.Sprintf("DATE#%s", startDate.Format(time.RFC3339)),
	}

	item, err := attributevalue.MarshalMap(reminder)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal reminder: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create reminder: %w", err)
	}

	return reminder, nil
}

// GetUserReminders obtiene todos los recordatorios de un usuario
func (c *Client) GetUserReminders(ctx context.Context, userID int64) ([]Reminder, error) {
	pk := fmt.Sprintf("USER#%d", userID)

	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :sk)"),
		FilterExpression:       aws.String("is_active = :active"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: pk},
			":sk":     &types.AttributeValueMemberS{Value: "REMINDER#"},
			":active": &types.AttributeValueMemberBOOL{Value: true},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query reminders: %w", err)
	}

	var reminders []Reminder
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &reminders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reminders: %w", err)
	}

	return reminders, nil
}

// UpdateReminderNextDate actualiza la fecha del próximo recordatorio
func (c *Client) UpdateReminderNextDate(ctx context.Context, reminder *Reminder) error {
	// Calcular siguiente fecha según frecuencia
	var nextDate time.Time
	switch reminder.Frequency {
	case "monthly":
		nextDate = reminder.NextReminderDate.AddDate(0, 1, 0)
	case "weekly":
		nextDate = reminder.NextReminderDate.AddDate(0, 0, 7)
	case "daily":
		nextDate = reminder.NextReminderDate.AddDate(0, 0, 1)
	default:
		// Si es "once", desactivar
		return c.DeactivateReminder(ctx, reminder.UserID, reminder.ID)
	}

	// Incrementar cuota actual
	currentInstallment := reminder.CurrentInstallment + 1

	// Si llegamos al final, desactivar
	if currentInstallment > reminder.TotalInstallments {
		return c.DeactivateReminder(ctx, reminder.UserID, reminder.ID)
	}

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: reminder.PK},
			"SK": &types.AttributeValueMemberS{Value: reminder.SK},
		},
		UpdateExpression: aws.String("SET next_reminder_date = :next_date, current_installment = :current, last_notified_at = :now, GSI2SK = :gsi2sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":next_date": &types.AttributeValueMemberS{Value: nextDate.Format(time.RFC3339)},
			":current":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", currentInstallment)},
			":now":       &types.AttributeValueMemberS{Value: time.Now().Format(time.RFC3339)},
			":gsi2sk":    &types.AttributeValueMemberS{Value: fmt.Sprintf("DATE#%s", nextDate.Format(time.RFC3339))},
		},
	})

	return err
}

// DeactivateReminder desactiva un recordatorio
func (c *Client) DeactivateReminder(ctx context.Context, userID int64, reminderID string) error {
	pk := fmt.Sprintf("USER#%d", userID)
	sk := fmt.Sprintf("REMINDER#%s", reminderID)

	_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pk},
			"SK": &types.AttributeValueMemberS{Value: sk},
		},
		UpdateExpression: aws.String("SET is_active = :inactive"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":inactive": &types.AttributeValueMemberBOOL{Value: false},
		},
	})

	return err
}

// GetPendingReminders obtiene recordatorios que deben notificarse
func (c *Client) GetPendingReminders(ctx context.Context) ([]Reminder, error) {
	now := time.Now().Format(time.RFC3339)

	result, err := c.db.Scan(ctx, &dynamodb.ScanInput{
		TableName:        aws.String(c.tableName),
		FilterExpression: aws.String("begins_with(SK, :sk) AND is_active = :active AND next_reminder_date <= :now"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":sk":     &types.AttributeValueMemberS{Value: "REMINDER#"},
			":active": &types.AttributeValueMemberBOOL{Value: true},
			":now":    &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to scan pending reminders: %w", err)
	}

	var reminders []Reminder
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &reminders); err != nil {
		return nil, fmt.Errorf("failed to unmarshal reminders: %w", err)
	}

	return reminders, nil
}

// ============================================
// CHANGELOG OPERATIONS
// ============================================

// ChangelogEntry representa un cambio en cualquier entidad del grupo
type ChangelogEntry struct {
	PK            string            `dynamodbav:"PK"`             // GROUP#<chat_id>
	SK            string            `dynamodbav:"SK"`             // CHANGELOG#<timestamp>#<uuid>
	ID            string            `dynamodbav:"id"`
	ChatID        int64             `dynamodbav:"chat_id"`
	EntityType    string            `dynamodbav:"entity_type"`    // expense | reminder | member | split
	EntityID      string            `dynamodbav:"entity_id"`      // ID del registro afectado
	EntityName    string            `dynamodbav:"entity_name"`    // Descripción legible
	Action        string            `dynamodbav:"action"`         // created | updated | deleted | restored | paid | divided
	ChangedByID   int64             `dynamodbav:"changed_by_id"`
	ChangedByName string            `dynamodbav:"changed_by_name"`
	Changes       map[string]string `dynamodbav:"changes,omitempty"` // {"campo": "antes → después"}
	CreatedAt     time.Time         `dynamodbav:"created_at"`
}

// SaveChangelog guarda una entrada de changelog en DynamoDB
func (c *Client) SaveChangelog(ctx context.Context, entry *ChangelogEntry) error {
	now := time.Now()
	entry.PK = fmt.Sprintf("GROUP#%d", entry.ChatID)
	entry.SK = fmt.Sprintf("CHANGELOG#%s#%s", now.Format("2006-01-02T15:04:05"), uuid.New().String()[:8])
	entry.ID = uuid.New().String()
	entry.CreatedAt = now

	item, err := attributevalue.MarshalMap(entry)
	if err != nil {
		return fmt.Errorf("failed to marshal changelog entry: %w", err)
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})
	return err
}

// GetGroupChangelog obtiene el historial de cambios de un grupo (últimas N entradas)
func (c *Client) GetGroupChangelog(ctx context.Context, chatID int64, limit int32) ([]ChangelogEntry, error) {
	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: fmt.Sprintf("GROUP#%d", chatID)},
			":prefix": &types.AttributeValueMemberS{Value: "CHANGELOG#"},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query changelog: %w", err)
	}

	var entries []ChangelogEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal changelog: %w", err)
	}
	return entries, nil
}

// GetEntityChangelog obtiene el historial de cambios de una entidad específica
func (c *Client) GetEntityChangelog(ctx context.Context, chatID int64, entityID string) ([]ChangelogEntry, error) {
	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk AND begins_with(SK, :prefix)"),
		FilterExpression:       aws.String("entity_id = :eid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk":     &types.AttributeValueMemberS{Value: fmt.Sprintf("GROUP#%d", chatID)},
			":prefix": &types.AttributeValueMemberS{Value: "CHANGELOG#"},
			":eid":    &types.AttributeValueMemberS{Value: entityID},
		},
		ScanIndexForward: aws.Bool(false),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query entity changelog: %w", err)
	}

	var entries []ChangelogEntry
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &entries); err != nil {
		return nil, fmt.Errorf("failed to unmarshal changelog: %w", err)
	}
	return entries, nil
}
