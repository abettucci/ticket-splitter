package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/abettucci/group-split-bot/internal/db"
	"github.com/abettucci/group-split-bot/internal/telegram"
	"github.com/abettucci/group-split-bot/internal/whatsappweb"
	"github.com/aws/aws-lambda-go/lambda"
)

var (
	dbClient    *db.Client
	tgClient    *telegram.Client
	waWebClient *whatsappweb.Client
	logger      *log.Logger
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

	// WhatsApp Web is opt-in. This keeps existing Telegram reminders isolated
	// unless the sidecar and its credentials are explicitly configured.
	if os.Getenv("WHATSAPP_WEB_REMINDERS_ENABLED") == "true" {
		waWebClient = whatsappweb.NewClient()
		logger.Printf("WhatsApp Web reminder delivery enabled (sidecar=%s)", os.Getenv("WAWEB_SIDECAR_URL"))
	}
}

// handler se ejecuta periódicamente (cada hora) vía EventBridge
func handler(ctx context.Context) error {
	logger.Println("🐀 Checking for pending reminders...")

	// Obtener recordatorios pendientes
	reminders, err := dbClient.GetPendingReminders(ctx)
	if err != nil {
		logger.Printf("Error getting pending reminders: %v", err)
		return err
	}

	logger.Printf("Found %d pending reminders", len(reminders))

	// Procesar cada recordatorio
	for _, reminder := range reminders {
		logger.Printf("Processing reminder %s for user %d via %s", reminder.ID[:8], reminder.UserID, reminderChannel(&reminder))

		// Construir mensaje
		progress := fmt.Sprintf("Cuota %d de %d", reminder.CurrentInstallment, reminder.TotalInstallments)
		if reminder.TotalInstallments == 1 {
			progress = "Pago único"
		}

		message := fmt.Sprintf(`🐀 <b>Recordatorio de Pago</b>

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

		// Enviar por el canal elegido cuando el usuario creó el recordatorio.
		// Los records legacy, que no tienen canal, siguen yendo por Telegram.
		err = sendReminder(ctx, &reminder, message)
		if err != nil {
			logger.Printf("Error sending reminder %s: %v", reminder.ID[:8], err)
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

func reminderChannel(reminder *db.Reminder) string {
	if reminder.DeliveryChannel == "" {
		return db.ReminderChannelTelegram
	}
	return reminder.DeliveryChannel
}

func sendReminder(ctx context.Context, reminder *db.Reminder, message string) error {
	destination := reminder.DeliveryAddress
	if destination == 0 {
		destination = reminder.UserID
	}

	switch reminderChannel(reminder) {
	case db.ReminderChannelTelegram:
		return tgClient.SendMessage(ctx, destination, message)
	case db.ReminderChannelWhatsAppWeb:
		if !reminder.NotificationOptIn {
			return fmt.Errorf("WhatsApp Web reminder has no user opt-in")
		}
		if waWebClient == nil {
			return fmt.Errorf("WhatsApp Web reminders are disabled; set WHATSAPP_WEB_REMINDERS_ENABLED=true")
		}
		return waWebClient.SendMessage(ctx, destination, message)
	default:
		return fmt.Errorf("unsupported reminder delivery channel %q", reminder.DeliveryChannel)
	}
}

func main() {
	lambda.Start(handler)
}
