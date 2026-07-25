package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	dbClient *db.Client
	tgClient *telegram.Client
	logger   *log.Logger
)

func init() {
	logger = log.New(os.Stdout, "[REMINDER-WORKER] ", log.LstdFlags)

	// Initialize DynamoDB client
	var err error
	dbClient, err = db.NewClient(context.Background())
	if err != nil {
		logger.Fatalf("Failed to initialize DynamoDB client: %v", err)
	}

	// Initialize Telegram client
	tgClient = telegram.NewClient()
}

// handler se ejecuta periódicamente (cada hora) vía EventBridge
func handler(ctx context.Context) error {
	logger.Println("🔔 Checking for pending reminders...")

	// Obtener recordatorios pendientes
	reminders, err := dbClient.GetPendingReminders(ctx)
	if err != nil {
		logger.Printf("Error getting pending reminders: %v", err)
		return err
	}

	logger.Printf("Found %d pending reminders", len(reminders))

	// Procesar cada recordatorio
	for _, reminder := range reminders {
		logger.Printf("Processing reminder %s for user %d", reminder.ID[:8], reminder.UserID)

		// Construir mensaje
		progress := fmt.Sprintf("Cuota %d de %d", reminder.CurrentInstallment, reminder.TotalInstallments)
		if reminder.TotalInstallments == 1 {
			progress = "Pago único"
		}

		message := fmt.Sprintf(`🔔 <b>Recordatorio de Pago</b>

📝 <b>%s</b>
💰 Monto: $%.2f
👤 Para: %s
📊 %s

<i>No olvides realizar este pago.</i>

Usa /mis_recordatorios para ver todos tus recordatorios.`,
			telegram.EscapeHTML(reminder.Description),
			reminder.Amount,
			telegram.EscapeHTML(reminder.PayeeName),
			progress)

		// Enviar mensaje al usuario
		err = tgClient.SendMessage(ctx, reminder.UserID, message)
		if err != nil {
			logger.Printf("Error sending reminder to user %d: %v", reminder.UserID, err)
			continue
		}

		// Actualizar próxima fecha de recordatorio
		err = dbClient.UpdateReminderNextDate(ctx, &reminder)
		if err != nil {
			logger.Printf("Error updating reminder %s: %v", reminder.ID, err)
			continue
		}

		logger.Printf("✅ Reminder %s sent successfully", reminder.ID[:8])
	}

	logger.Printf("✅ Processed %d reminders", len(reminders))
	return nil
}

func main() {
	lambda.Start(handler)
}

