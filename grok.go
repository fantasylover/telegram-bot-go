package main

import (
	"bytes"
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	_ "github.com/mattn/go-sqlite3"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/skip2/go-qrcode"
)

// Constants
const (
	BOT_TOKEN = "1743577119:AAEiYy_kgUK41RcBxF18NgkR4VehXtZWm_w" // BotFather token
	ADMIN_ID  = 1192041312                                  // Admin Telegram ID
)

var BOT_USERNAME string

// Initialize database
func initDB() *sql.DB {
	db, err := sql.Open("sqlite3", "./bot.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create tables
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY,
		username TEXT,
		balance REAL DEFAULT 0,
		wallet TEXT,
		referrals INTEGER DEFAULT 0,
		referred_by INTEGER DEFAULT 0,
		banned INTEGER DEFAULT 0,
		button_style TEXT DEFAULT 'inline',
		state TEXT DEFAULT ''
	)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT
	)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS required_channels (
		channel_id TEXT PRIMARY KEY
	)`)
	if err != nil {
		log.Fatal(err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS withdrawals (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER,
		amount REAL,
		wallet TEXT,
		status TEXT DEFAULT 'pending',
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)
	if err != nil {
		log.Fatal(err)
	}

	// Default settings
	setSetting(db, "min_withdrawal", "10")
	setSetting(db, "referral_reward", "5")
	setSetting(db, "start_message", "Welcome to the bot! Use /start to begin.")
	setSetting(db, "payment_channel", "@YourPaymentChannel")
	setSetting(db, "qr_enabled", "1")

	return db
}

// User struct for database
type User struct {
	ID          int64
	Username    string
	Balance     float64
	Wallet      string
	Referrals   int
	ReferredBy  int64
	Banned      bool
	ButtonStyle string
	State       string
}

// Setting struct for database
type Setting struct {
	Key   string
	Value string
}

// Withdrawal struct for database
type Withdrawal struct {
	ID        int
	UserID    int64
	Amount    float64
	Wallet    string
	Status    string
	Timestamp time.Time
}

// Database helper functions
func getUser(db *sql.DB, userID int64) (User, error) {
	var user User
	err := db.QueryRow("SELECT id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE id = ?", userID).Scan(
		&user.ID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State)
	if err != nil {
		if err == sql.ErrNoRows {
			user = User{ID: userID, Balance: 0, Referrals: 0, Banned: false, ButtonStyle: "inline", State: ""}
			updateUser(db, user)
		} else {
			return user, err
		}
	}
	return user, nil
}

func getUserByUsername(db *sql.DB, username string) (User, error) {
	var user User
	err := db.QueryRow("SELECT id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE username = ?", username).Scan(
		&user.ID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State)
	return user, err
}

func updateUser(db *sql.DB, user User) error {
	_, err := db.Exec("INSERT OR REPLACE INTO users (id, username, balance, wallet, referrals, referred_by, banned, button_style, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		user.ID, user.Username, user.Balance, user.Wallet, user.Referrals, user.ReferredBy, user.Banned, user.ButtonStyle, user.State)
	return err
}

func setSetting(db *sql.DB, key, value string) error {
	_, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
	return err
}

func getSetting(db *sql.DB, key string) (string, error) {
	var value string
	err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func addRequiredChannel(db *sql.DB, channel string) error {
	_, err := db.Exec("INSERT OR IGNORE INTO required_channels (channel_id) VALUES (?)", channel)
	return err
}

func removeRequiredChannel(db *sql.DB, channel string) error {
	_, err := db.Exec("DELETE FROM required_channels WHERE channel_id = ?", channel)
	return err
}

func createWithdrawal(db *sql.DB, userID int64, amount float64, wallet string) error {
	_, err := db.Exec("INSERT INTO withdrawals (user_id, amount, wallet) VALUES (?, ?, ?)", userID, amount, wallet)
	return err
}

func getTotalUsers(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}

func getCompletedWithdrawals(db *sql.DB) (int, error) {
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM withdrawals WHERE status = 'completed'").Scan(&count)
	return count, err
}

// Utility functions
func escapeMarkdownV2(text string) string {
	chars := []string{"_", "*", "`", "[", "]", "(", ")", ".", "!", "-", "+"}
	for _, c := range chars {
		text = strings.ReplaceAll(text, c, "\\"+c)
	}
	return text
}

func createQRCode(data string) ([]byte, error) {
	var buf bytes.Buffer
	err := qrcode.WriteFile(data, qrcode.Medium, 256, "qrcode.png")
	if err != nil {
		return nil, err
	}
	file, err := os.ReadFile("qrcode.png")
	if err != nil {
		return nil, err
	}
	return file, nil
}


func main() {
	// Initialize bot
	bot, err := tgbotapi.NewBotAPI(BOT_TOKEN)
	if err != nil {
		log.Fatal(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)
	BOT_USERNAME = bot.Self.UserName

	// Initialize database
	db := initDB()
	defer db.Close()

	// Set up update channel
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	// Main loop to handle updates
	for update := range updates {
		if update.Message == nil && update.CallbackQuery == nil {
			continue
		}

		// Extract user info
		userID := int64(0)
		username := ""
		firstName := ""
		if update.Message != nil {
			userID = update.Message.From.ID
			username = update.Message.From.UserName
			if update.Message.From.FirstName != "" {
				firstName = update.Message.From.FirstName
			} else {
				firstName = username
			}
		} else if update.CallbackQuery != nil {
			userID = update.CallbackQuery.From.ID
			username = update.CallbackQuery.From.UserName
			if update.CallbackQuery.From.FirstName != "" {
				firstName = update.CallbackQuery.From.FirstName
			} else {
				firstName = username
			}
		}

		// Get or create user
		user, err := getUser(db, userID)
		if err != nil {
			log.Println("Failed to get user:", err)
			continue
		}
		user.Username = username
		updateUser(db, user)

		// Handle commands
		if update.Message != nil && update.Message.IsCommand() {
			handleCommands(bot, db, update.Message, user)
		}

		// Handle callback queries
		if update.CallbackQuery != nil {
			handleCallbackQueries(bot, db, update.CallbackQuery, user, firstName)
		}

		// Handle state messages
		if update.Message != nil && user.State != "" {
			handleStateMessages(bot, db, update.Message, user, firstName)
		}
	}
}

func handleCommands(bot *tgbotapi.BotAPI, db *sql.DB, message *tgbotapi.Message, user User) {
	userID := message.From.ID
	switch message.Command() {
	case "start":
		// Handle referral
		referredBy := int64(0)
		args := message.CommandArguments()
		if args != "" {
			referredBy, _ = strconv.ParseInt(args, 10, 64)
			if referredBy != 0 && referredBy != userID {
				reward, _ := strconv.ParseFloat(getSetting(db, "referral_reward"))
				referrer, _ := getUser(db, referredBy)
				referrer.Balance += reward
				referrer.Referrals++
				updateUser(db, referrer)
			}
		}
		user.ReferredBy = referredBy
		updateUser(db, user)

		// Send welcome message
		startMsg, _ := getSetting(db, "start_message")
		msg := tgbotapi.NewMessage(userID, escapeMarkdownV2(startMsg))
		msg.ParseMode = "MarkdownV2"
		if user.ButtonStyle == "inline" {
			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("💰 Balance", "balance"),
					tgbotapi.NewInlineKeyboardButtonData("💳 Set Wallet", "set_wallet"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("📞 Support", "support"),
					tgbotapi.NewInlineKeyboardButtonData("👥 Referral", "referral"),
				),
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonData("📊 Stats", "stats"),
					tgbotapi.NewInlineKeyboardButtonData("💸 Withdraw", "withdraw"),
				),
			)
			msg.ReplyMarkup = keyboard
		} else {
			keyboard := tgbotapi.NewReplyKeyboard(
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("💰 Balance"),
					tgbotapi.NewKeyboardButton("💳 Set Wallet"),
				),
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("📞 Support"),
					tgbotapi.NewKeyboardButton("👥 Referral"),
				),
				tgbotapi.NewKeyboardButtonRow(
					tgbotapi.NewKeyboardButton("📊 Stats"),
					tgbotapi.NewKeyboardButton("💸 Withdraw"),
				),
			)
			msg.ReplyMarkup = keyboard
		}
		bot.Send(msg)

	case "admin":
		if userID != ADMIN_ID {
			msg := tgbotapi.NewMessage(userID, "🚫 *Unauthorized.*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📢 Broadcast", "broadcast"),
				tgbotapi.NewInlineKeyboardButtonData("📊 User Info", "user_info"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💵 Set Min Withdrawal", "set_min_withdrawal"),
				tgbotapi.NewInlineKeyboardButtonData("📡 Set Payment Channel", "set_payment_channel"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("🎁 Set Referral Reward", "set_referral_reward"),
				tgbotapi.NewInlineKeyboardButtonData("➕ Add Channel", "add_channel"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("➖ Remove Channel", "remove_channel"),
				tgbotapi.NewInlineKeyboardButtonData("✍️ Start Settings", "start_settings"),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("📷 QR Settings", "qr_settings"),
			),
		)
		msg := tgbotapi.NewMessage(userID, "📊 *Admin Panel*")
		msg.ParseMode = "MarkdownV2"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	}
}

func handleCallbackQueries(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery, user User, firstName string) {
	userID := callback.From.ID
	data := callback.Data

	switch data {
	case "balance":
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Your Balance:* %s\n👥 *Referrals:* %d", escapeMarkdownV2(fmt.Sprintf("%.1f", user.Balance)), user.Referrals))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "set_wallet":
		user.State = "setting_wallet"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "💳 *Enter your wallet address:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "support":
		user.State = "support_message"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "📞 *Enter your message for support:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "referral":
		referralLink := fmt.Sprintf("https://t.me/%s?start=%d", BOT_USERNAME, userID)
		rows, _ := db.Query("SELECT username FROM users WHERE referred_by = ?", userID)
		referrals := []string{}
		for rows.Next() {
			var refUsername string
			rows.Scan(&refUsername)
			referrals = append(referrals, refUsername)
		}
		rows.Close()
		msgText := fmt.Sprintf("👥 *Referral Link:* [%s](%s)\n*Total Referrals:* %d", escapeMarkdownV2(referralLink), referralLink, len(referrals))
		if len(referrals) > 0 {
			msgText += "\n*Referred Users:*\n" + strings.Join(referrals, "\n")
		}
		msg := tgbotapi.NewMessage(userID, msgText)
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "stats":
		totalUsers, _ := getTotalUsers(db)
		completedWithdrawals, _ := getCompletedWithdrawals(db)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📊 *Stats*\n*Total Users:* %d\n*Completed Withdrawals:* %d", totalUsers, completedWithdrawals))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "withdraw":
		minWithdrawal, _ := strconv.ParseFloat(getSetting(db, "min_withdrawal"))
		if user.Balance < minWithdrawal {
			msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💸 *Minimum withdrawal is* %s *FREE COIN.*", escapeMarkdownV2(fmt.Sprintf("%.1f", minWithdrawal))))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		if user.Wallet == "" {
			msg := tgbotapi.NewMessage(userID, "💳 *Please set your wallet address first.*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		user.State = "withdraw_amount"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "broadcast":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "broadcast_message"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "📢 *Enter message to broadcast (text/photo/video/document):*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "user_info":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "getting_user_info"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "📊 *Enter user ID or @username:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "set_min_withdrawal":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "setting_min_withdrawal"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "💵 *Enter minimum withdrawal amount:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "set_payment_channel":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "setting_payment_channel"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "📡 *Enter payment channel (@username):*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "set_referral_reward":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "setting_referral_reward"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "🎁 *Enter referral reward amount:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "add_channel":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "add_channel"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "➕ *Enter channel to add (@username):*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "remove_channel":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "remove_channel"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "➖ *Enter channel to remove (@username):*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "start_settings":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		user.State = "setting_start_message"
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, "✍️ *Enter new start message:*")
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "qr_settings":
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		qrEnabled, _ := getSetting(db, "qr_enabled")
		newValue := "0"
		if qrEnabled == "0" {
			newValue = "1"
		}
		setSetting(db, "qr_enabled", newValue)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📷 *QR Code generation is now* %s", escapeMarkdownV2(newValue)))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)
	}

	// Admin actions (ban, unban, adjust balance, etc.)
	if strings.HasPrefix(data, "adjust_") || strings.HasPrefix(data, "ban_") || strings.HasPrefix(data, "unban_") || strings.HasPrefix(data, "viewrefs_") || strings.HasPrefix(data, "contact_") {
		if userID != ADMIN_ID {
			bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized"))
			return
		}
		parts := strings.Split(data, "_")
		if len(parts) < 2 {
			return
		}
		action, target := parts[0], parts[1]
		targetID, _ := strconv.ParseInt(target, 10, 64)

		switch action {
		case "adjust":
			user.State = fmt.Sprintf("adjusting_balance_%d", targetID)
			updateUser(db, user)
			msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Enter amount to adjust for user* %d *(e.g., +10 or -5):*", targetID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)

		case "ban":
			targetUser, _ := getUser(db, targetID)
			targetUser.Banned = true
			updateUser(db, targetUser)
			bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *banned.*", targetID)))
			bot.Send(tgbotapi.NewMessage(targetID, "🚫 *You have been banned from the bot.*"))

		case "unban":
			targetUser, _ := getUser(db, targetID)
			targetUser.Banned = false
			updateUser(db, targetUser)
			bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *unbanned.*", targetID)))
			bot.Send(tgbotapi.NewMessage(targetID, "✅ *You have been unbanned!*"))

		case "viewrefs":
			rows, _ := db.Query("SELECT username FROM users WHERE referred_by = ?", targetID)
			referrals := []string{}
			for rows.Next() {
				var refUsername string
				rows.Scan(&refUsername)
				referrals = append(referrals, refUsername)
			}
			rows.Close()
			if len(referrals) > 0 {
				var bio bytes.Buffer
				bio.WriteString(strings.Join(referrals, "\n"))
				file := tgbotapi.FileBytes{Name: fmt.Sprintf("referrals_%d.txt", targetID), Bytes: bio.Bytes()}
				bot.Send(tgbotapi.NewDocumentUpload(userID, file))
			} else {
				msg := tgbotapi.NewMessage(userID, "📄 *No referrals yet.*")
				msg.ParseMode = "MarkdownV2"
				bot.Send(msg)
			}

		case "contact":
			user.State = fmt.Sprintf("contacting_%d", targetID)
			updateUser(db, user)
			msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📩 *Enter message for user* %d:", targetID))
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
		}
		bot.AnswerCallbackQuery(tgbotapi.NewCallback(callback.ID, ""))
	}
}

func handleStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, message *tgbotapi.Message, user User, firstName string) {
	userID := message.From.ID
	username := message.From.UserName

	switch user.State {
	case "setting_wallet":
		wallet := message.Text
		user.Wallet = wallet
		user.State = ""
		updateUser(db, user)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Wallet set to:* `%s`", escapeMarkdownV2(wallet)))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)

	case "withdraw_amount":
		amount, err := strconv.ParseFloat(message.Text, 64)
		if err != nil || amount <= 0 || amount > user.Balance {
			msg := tgbotapi.NewMessage(userID, "❌ *Enter a valid amount.*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		user.Balance -= amount
		user.State = ""
		updateUser(db, user)
		createWithdrawal(db, userID, amount, user.Wallet)
		bot.Send(tgbotapi.NewMessage(userID, "✅ *Withdrawal request sent! Admin will review soon.*"))

		paymentChannel, _ := getSetting(db, "payment_channel")
		if paymentChannel != "" {
			escapedUsername := escapeMarkdownV2(username)
			escapedWallet := escapeMarkdownV2(user.Wallet)
			amountStr := escapeMarkdownV2(fmt.Sprintf("%.1f", amount))
			rand.Seed(time.Now().UnixNano())
			txID := fmt.Sprintf("2025%d", rand.Intn(9000000)+1000000)
			var channelID string
			err := db.QueryRow("SELECT channel_id FROM required_channels LIMIT 1").Scan(&channelID)
			if err != nil {
				channelID = "@DefaultChannel"
			}

			msgText := fmt.Sprintf(
				"🔥 *NEW WITHDRAWAL SENT* 🔥\n\n"+
					"👤 *USER:* [%s](tg://user?id=%d)\n"+
					"💎 *USER ID:* `%d`\n"+
					"💰 *AMOUNT:* `%s` FREE COIN\n"+
					"📞 *REFERRER:* `%d`\n"+
					"🔗 *ADDRESS:* `%s`\n"+
					"⏰ *TRANSACTION ID:* `%s`",
				escapeMarkdownV2(firstName), userID, userID, amountStr, user.Referrals, escapedWallet, txID,
			)

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(
					tgbotapi.NewInlineKeyboardButtonURL("🔍CHANN", fmt.Sprintf("https://t.me/%s", strings.TrimPrefix(channelID, "@"))),
					tgbotapi.NewInlineKeyboardButtonURL("JOIN", fmt.Sprintf("https://t.me/%s", BOT_USERNAME)),
				),
			)

			if qrEnabled, _ := getSetting(db, "qr_enabled"); qrEnabled == "1" {
				qr, err := createQRCode(user.Wallet)
				if err == nil {
					photo := tgbotapi.NewPhotoUpload(paymentChannel, tgbotapi.FileBytes{Name: "qrcode.png", Bytes: qr})
					photo.Caption = msgText
					photo.ParseMode = "MarkdownV2"
					photo.ReplyMarkup = keyboard
					bot.Send(photo)
				} else {
					msg := tgbotapi.NewMessageToChannel(paymentChannel, msgText+"\n⚠️ *QR code generation failed.*")
					msg.ParseMode = "MarkdownV2"
					msg.ReplyMarkup = keyboard
					bot.Send(msg)
				}
			} else {
				msg := tgbotapi.NewMessageToChannel(paymentChannel, msgText)
				msg.ParseMode = "MarkdownV2"
				msg.ReplyMarkup = keyboard
				bot.Send(msg)
			}
		}

	case "support_message":
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("Ban User", fmt.Sprintf("ban_%d", userID)),
			),
		)
		msgText := fmt.Sprintf("📞 *Support from* @%s\n%s", escapeMarkdownV2(username), escapeMarkdownV2(message.Text))
		msg := tgbotapi.NewMessage(int64(ADMIN_ID), msgText)
		msg.ParseMode = "MarkdownV2"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		user.State = ""
		updateUser(db, user)
		bot.Send(tgbotapi.NewMessage(userID, "✅ *Your message has been sent to support!*"))

	case "broadcast_message":
		if userID != ADMIN_ID {
			return
		}
		rows, _ := db.Query("SELECT id FROM users WHERE banned = 0")
		userIDs := []int64{}
		for rows.Next() {
			var uid int64
			rows.Scan(&uid)
			userIDs = append(userIDs, uid)
		}
		rows.Close()

		totalUsers := len(userIDs)
		sentCount := 0
		statusMsg := tgbotapi.NewMessage(userID, "📢 *Broadcasting:* [□□□□□□□□□□] 0%")
		statusMsg.ParseMode = "MarkdownV2"
		sentMsg, _ := bot.Send(statusMsg)

		for i, uid := range userIDs {
			var content tgbotapi.Chattable
			switch {
			case message.Text != "":
				msg := tgbotapi.NewMessage(uid, escapeMarkdownV2(message.Text))
				msg.ParseMode = "MarkdownV2"
				content = msg
			case message.Photo != nil:
				photo := tgbotapi.NewPhotoUpload(uid, message.Photo[len(message.Photo)-1].FileID)
				photo.Caption = escapeMarkdownV2(message.Caption)
				photo.ParseMode = "MarkdownV2"
				content = photo
			case message.Video != nil:
				video := tgbotapi.NewVideoUpload(uid, message.Video.FileID)
				video.Caption = escapeMarkdownV2(message.Caption)
				video.ParseMode = "MarkdownV2"
				content = video
			case message.Document != nil:
				doc := tgbotapi.NewDocumentUpload(uid, message.Document.FileID)
				doc.Caption = escapeMarkdownV2(message.Caption)
				doc.ParseMode = "MarkdownV2"
				content = doc
			}
			if _, err := bot.Send(content); err == nil {
				sentCount++
			}
			progress := (sentCount * 10) / totalUsers
			bar := strings.Repeat("█", progress) + strings.Repeat("□", 10-progress)
			percentage := (sentCount * 100) / totalUsers
			bot.Send(tgbotapi.NewEditMessageText(userID, sentMsg.MessageID, fmt.Sprintf("📢 *Broadcasting:* [%s] %d%% (%d/%d)", bar, percentage, sentCount, totalUsers)))
			time.Sleep(100 * time.Millisecond)
		}
		bot.Send(tgbotapi.NewEditMessageText(userID, sentMsg.MessageID, fmt.Sprintf("✅ *Broadcast completed!* Sent to %d/%d users.", sentCount, totalUsers)))
		user.State = ""
		updateUser(db, user)

	case "getting_user_info":
		if userID != ADMIN_ID {
			return
		}
		target := message.Text
		var targetUser User
		if strings.HasPrefix(target, "@") {
			targetUser, err = getUserByUsername(db, strings.TrimPrefix(target, "@"))
		} else {
			targetID, _ := strconv.ParseInt(target, 10, 64)
			targetUser, err = getUser(db, targetID)
		}
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "❌ *User not found.*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			user.State = ""
			updateUser(db, user)
			return
		}
		escapedWallet := escapeMarkdownV2(targetUser.Wallet)
		if targetUser.Wallet == "" {
			escapedWallet = "Not set"
		}
		msgText := fmt.Sprintf(
			"👤 *User Info*\n*ID:* %d\n*Username:* @%s\n*Balance:* %s 💰\n*Wallet:* `%s`\n*Referrals:* %d\n*Banned:* %v",
			targetUser.ID, escapeMarkdownV2(targetUser.Username), escapeMarkdownV2(fmt.Sprintf("%.1f", targetUser.Balance)), escapedWallet, targetUser.Referrals, targetUser.Banned,
		)
		keyboard := tgbotapi.NewInlineKeyboardMarkup(
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("💰 Adjust Balance", fmt.Sprintf("adjust_%d", targetUser.ID)),
				tgbotapi.NewInlineKeyboardButtonData("Ban User", fmt.Sprintf("ban_%d", targetUser.ID)),
			),
			tgbotapi.NewInlineKeyboardRow(
				tgbotapi.NewInlineKeyboardButtonData("View Referrals", fmt.Sprintf("viewrefs_%d", targetUser.ID)),
				tgbotapi.NewInlineKeyboardButtonData("Contact User", fmt.Sprintf("contact_%d", targetUser.ID)),
			),
		)
		msg := tgbotapi.NewMessage(userID, msgText)
		msg.ParseMode = "MarkdownV2"
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
		user.State = ""
		updateUser(db, user)

	case "setting_min_withdrawal", "setting_referral_reward", "setting_start_message", "setting_payment_channel":
		if userID != ADMIN_ID {
			return
		}
		key := strings.TrimPrefix(user.State, "setting_")
		value := message.Text
		setSetting(db, key, value)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *%s set to:* %s", escapeMarkdownV2(strings.ReplaceAll(key, "_", " ")), escapeMarkdownV2(value)))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)
		user.State = ""
		updateUser(db, user)

	case "add_channel":
		if userID != ADMIN_ID {
			return
		}
		channel := message.Text
		if !strings.HasPrefix(channel, "@") {
			msg := tgbotapi.NewMessage(userID, "❌ *Use '@' (e.g., @ChannelName).*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		addRequiredChannel(db, channel)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("➕ *Channel* %s *added!*", escapeMarkdownV2(channel)))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)
		user.State = ""
		updateUser(db, user)

	case "remove_channel":
		if userID != ADMIN_ID {
			return
		}
		channel := message.Text
		if !strings.HasPrefix(channel, "@") {
			msg := tgbotapi.NewMessage(userID, "❌ *Use '@' (e.g., @ChannelName).*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		removeRequiredChannel(db, channel)
		msg := tgbotapi.NewMessage(userID, fmt.Sprintf("➖ *Channel* %s *removed!*", escapeMarkdownV2(channel)))
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)
		user.State = ""
		updateUser(db, user)

	case strings.HasPrefix(user.State, "adjusting_balance_"):
		if userID != ADMIN_ID {
			return
		}
		parts := strings.Split(user.State, "_")
		targetUserID, _ := strconv.ParseInt(parts[2], 10, 64)
		amount, err := strconv.ParseFloat(message.Text, 64)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "❌ *Enter a valid number (e.g., +10 or -5).*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		targetUser, _ := getUser(db, targetUserID)
		newBalance := targetUser.Balance + amount
		if newBalance < 0 {
			msg := tgbotapi.NewMessage(userID, "❌ *Balance cannot be negative.*")
			msg.ParseMode = "MarkdownV2"
			bot.Send(msg)
			return
		}
		targetUser.Balance = newBalance
		updateUser(db, targetUser)
		bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *Balance updated to* %s *for user* %d.", escapeMarkdownV2(fmt.Sprintf("%.1f", newBalance)), targetUserID)))
		bot.Send(tgbotapi.NewMessage(targetUserID, fmt.Sprintf("💰 *Your balance updated to* %s.", escapeMarkdownV2(fmt.Sprintf("%.1f", newBalance)))))
		user.State = ""
		updateUser(db, user)

	case strings.HasPrefix(user.State, "contacting_"):
		if userID != ADMIN_ID {
			return
		}
		parts := strings.Split(user.State, "_")
		targetUserID, _ := strconv.ParseInt(parts[1], 10, 64)
		msgText := fmt.Sprintf("📩 *Message from Admin:*\n%s", escapeMarkdownV2(message.Text))
		msg := tgbotapi.NewMessage(targetUserID, msgText)
		msg.ParseMode = "MarkdownV2"
		bot.Send(msg)
		bot.Send(tgbotapi.NewMessage(userID, "✅ *Message sent to user!*"))
		user.State = ""
		updateUser(db, user)
	}
}


