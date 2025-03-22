// --- Start of Part 1: Imports and Constants ---
// --- Start of Part 1: Imports and Constants ---
// --- Start of Part 1: Imports and Constants ---
package main

import (
    "database/sql"
    "encoding/json"
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

const (
    BOT_TOKEN = "1743577119:AAEiYy_kgUK41RcBxF18NgkR4VehXtZWm_w"
    ADMIN_ID  = 1192041312
)

var (
    BOT_USERNAME = "@Superbv2_bot"
    db           *sql.DB // Declare db at package level
)
// --- End of Part 1: Imports and Constants ---


// --- Start of Part 2: Structs and Database Initialization ---
type User struct {
    UserID      int64
    Username    string
    Balance     float64
    Wallet      string
    Referrals   int
    ReferredBy  sql.NullInt64
    Banned      int
    ButtonStyle string
    State       string
}

type Withdrawal struct {
    ID        int64
    UserID    int64
    Amount    float64
    Wallet    string
    Status    string
    Timestamp int64
}

func initDB() (*sql.DB, error) {
    db, err := sql.Open("sqlite3", "./bot.db")
    if err != nil {
        return nil, err
    }
    queries := []string{
        `CREATE TABLE IF NOT EXISTS users (
            user_id INTEGER PRIMARY KEY,
            username TEXT,
            balance REAL DEFAULT 0,
            wallet TEXT,
            referrals INTEGER DEFAULT 0,
            referred_by INTEGER,
            banned INTEGER DEFAULT 0,
            button_style TEXT,
            state TEXT DEFAULT ''
        )`,
        `CREATE TABLE IF NOT EXISTS settings (
            key TEXT PRIMARY KEY,
            value TEXT
        )`,
        `CREATE TABLE IF NOT EXISTS required_channels (
            channel TEXT PRIMARY KEY
        )`,
        `CREATE TABLE IF NOT EXISTS withdrawals (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER,
            amount REAL,
            wallet TEXT,
            status TEXT DEFAULT 'pending',
            timestamp INTEGER DEFAULT 0
        )`,
    }
    for _, query := range queries {
        if _, err := db.Exec(query); err != nil {
            return nil, err
        }
    }
    defaultSettings := map[string]string{
        "min_withdrawal":    "10",
        "payment_channel":   "@YourPaymentChannel",
        "referral_reward":   "5",
        "start_message":     "🎉 Welcome to the Referral & Earning Bot\\! Join channels to start\\.",
        "qr_enabled":        "0",
    }
    for key, value := range defaultSettings {
        _, err := db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", key, value)
        if err != nil {
            return nil, err
        }
    }
    return db, nil
}
// --- End of Part 2: Structs and Database Initialization ---

// --- Start of Part 3: Database Helper Functions (Part 1) ---
// --- Start of Part 3: Database Helper Functions (Part 1) ---
func createUser(db *sql.DB, user User) error {
    query := `INSERT INTO users (user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
    _, err := db.Exec(query, user.UserID, user.Username, user.Balance, user.Wallet, user.Referrals, user.ReferredBy, user.Banned, user.ButtonStyle, user.State)
    return err
}

func getUser(db *sql.DB, userID int64) (User, error) {
    var user User
    query := `SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state 
              FROM users WHERE user_id = ?`
    err := db.QueryRow(query, userID).Scan(&user.UserID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State)
    if err == sql.ErrNoRows {
        log.Printf("No user found for ID %d", userID)
        return User{}, nil
    } else if err != nil {
        log.Printf("Error retrieving user %d: %v", userID, err)
        return User{}, err
    }
    log.Printf("Retrieved user %d: Balance %.2f, State %s", user.UserID, user.Balance, user.State)
    return user, nil
}

func updateUser(db *sql.DB, user User) error {
    tx, err := db.Begin()
    if err != nil {
        log.Printf("Error starting transaction: %v", err)
        return err
    }
    query := `INSERT OR REPLACE INTO users (user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
    _, err = tx.Exec(query, user.UserID, user.Username, user.Balance, user.Wallet, user.Referrals, user.ReferredBy, user.Banned, user.ButtonStyle, user.State)
    if err != nil {
        tx.Rollback()
        log.Printf("Error executing updateUser for user %d: %v", user.UserID, err)
        return err
    }
    if err := tx.Commit(); err != nil {
        log.Printf("Error committing transaction for user %d: %v", user.UserID, err)
        return err
    }
    log.Printf("User %d updated in database: Balance %.2f, State %s", user.UserID, user.Balance, user.State)
    return nil
}
// --- End of Part 3: Database Helper Functions (Part 1) ---


// --- Start of Part 4: Database Helper Functions (Part 2) ---
func getSetting(db *sql.DB, key string) (string, error) {
    var value string
    err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", nil
    }
    return value, err
}

func updateSetting(db *sql.DB, key, value string) error {
    _, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
    return err
}

func getRequiredChannels(db *sql.DB) ([]string, error) {
    rows, err := db.Query("SELECT channel FROM required_channels")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var channels []string
    for rows.Next() {
        var channel string
        if err := rows.Scan(&channel); err != nil {
            return nil, err
        }
        channels = append(channels, channel)
    }
    return channels, nil
}

func addRequiredChannel(db *sql.DB, channel string) error {
    _, err := db.Exec("INSERT OR IGNORE INTO required_channels (channel) VALUES (?)", channel)
    return err
}

func removeRequiredChannel(db *sql.DB, channel string) error {
    _, err := db.Exec("DELETE FROM required_channels WHERE channel = ?", channel)
    return err
}

func createWithdrawal(db *sql.DB, userID int64, amount float64, wallet string) error {
    _, err := db.Exec("INSERT INTO withdrawals (user_id, amount, wallet, timestamp) VALUES (?, ?, ?, ?)",
        userID, amount, wallet, time.Now().Unix())
    return err
}

func getTotalUsers(db *sql.DB) (int, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
    return count, err
}

func getTotalWithdrawals(db *sql.DB) (int, error) {
    var count int
    err := db.QueryRow("SELECT COUNT(*) FROM withdrawals WHERE status = 'completed'").Scan(&count)
    return count, err
}
// --- End of Part 4: Database Helper Functions (Part 2) ---

// --- Start of Part 5: General Helper Functions ---
// --- Start of Part 5: General Helper Functions ---
// --- Start of Part 5: General Helper Functions ---
// --- Start of Part 5: General Helper Functions ---
// --- Start of Part 5: General Helper Functions ---
func generateReferralLink(userID int64) string {
    return fmt.Sprintf("https://t.me/%s?start=%d", BOT_USERNAME, userID)
}

func checkUserJoinedChannels(bot *tgbotapi.BotAPI, userID int64, db *sql.DB) (bool, error) {
    channels, err := getRequiredChannels(db)
    if err != nil {
        return false, err
    }
    if len(channels) == 0 {
        return true, nil
    }
    for _, channel := range channels {
        params := tgbotapi.Params{"chat_id": channel}
        resp, err := bot.MakeRequest("getChat", params)
        if err != nil {
            return false, err
        }
        var chat tgbotapi.Chat
        if err := json.Unmarshal(resp.Result, &chat); err != nil {
            return false, err
        }
        chatConfig := tgbotapi.GetChatMemberConfig{
            ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
                ChatID: chat.ID,
                UserID: userID,
            },
        }
        member, err := bot.GetChatMember(chatConfig)
        if err != nil {
            return false, err
        }
        if member.Status != "member" && member.Status != "administrator" && member.Status != "creator" {
            return false, nil
        }
    }
    return true, nil
}

func escapeMarkdownV2(text string) string {
    specialChars := []string{"[", "]", "(", ")", "~", ">", "#", "+", "-", "=", "|", "{", "}", ".", "!"}
    for _, char := range specialChars {
        text = strings.ReplaceAll(text, char, "\\"+char)
    }
    return text
}

func formatMarkdownV2(template string, args ...interface{}) string {
    formatted := template
    if len(args) > 0 {
        formatted = fmt.Sprintf(template, args...)
    }
    return escapeMarkdownV2(formatted)
}

func getChatIDFromUsername(bot *tgbotapi.BotAPI, username string) (int64, error) {
    chatConfig := tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: 0, SuperGroupUsername: username}}
    chat, err := bot.GetChat(chatConfig)
    if err != nil {
        return 0, err
    }
    return chat.ID, nil
}

func showMainMenu(bot *tgbotapi.BotAPI, userID int64, buttonStyle string) {
    log.Printf("Attempting to show main menu for user %d with style %s", userID, buttonStyle)
    if buttonStyle == "inline" {
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("💰 Balance", "balance"),
                tgbotapi.NewInlineKeyboardButtonData("💳 Set Wallet", "set_wallet"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📞 Support", "support"),
                tgbotapi.NewInlineKeyboardButtonData("🔗 Referral", "referral"),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📈 Stats", "stats"),
                tgbotapi.NewInlineKeyboardButtonData("💸 Withdraw", "withdraw"),
            ),
        )
        // Use plain text to avoid MarkdownV2 issues
        msg := tgbotapi.NewMessage(userID, "Welcome back!\nChoose an option below:\n\n💰 Balance\n💳 Set Wallet\n📞 Support\n🔗 Referral\n📈 Stats\n💸 Withdraw")
        msg.ReplyMarkup = markup
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending inline main menu to user %d: %v", userID, err)
            sendError(bot, userID, ErrSendMessageFailed, "Failed to send inline menu")
        } else {
            log.Printf("Inline main menu sent successfully to user %d", userID)
        }
    } else {
        replyMarkup := tgbotapi.NewReplyKeyboard(
            tgbotapi.NewKeyboardButtonRow(
                tgbotapi.NewKeyboardButton("💰 Balance"),
                tgbotapi.NewKeyboardButton("💳 Set Wallet"),
            ),
            tgbotapi.NewKeyboardButtonRow(
                tgbotapi.NewKeyboardButton("📞 Support"),
                tgbotapi.NewKeyboardButton("🔗 Referral"),
            ),
            tgbotapi.NewKeyboardButtonRow(
                tgbotapi.NewKeyboardButton("📈 Stats"),
                tgbotapi.NewKeyboardButton("💸 Withdraw"),
            ),
        )
        replyMarkup.ResizeKeyboard = true
        replyMarkup.OneTimeKeyboard = false
        // Use plain text to avoid MarkdownV2 issues
        msg := tgbotapi.NewMessage(userID, "Welcome back!\nType an option:\n💰 Balance\n💳 Set Wallet\n📞 Support\n🔗 Referral\n📈 Stats\n💸 Withdraw")
        msg.ReplyMarkup = replyMarkup
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending normal main menu to user %d: %v", userID, err)
            sendError(bot, userID, ErrSendMessageFailed, "Failed to send normal menu")
        } else {
            log.Printf("Normal main menu sent successfully to user %d", userID)
        }
    }
}
// --- End of Part 5: General Helper Functions ---

// --- Start of Part 6: Handle Start Command ---
// --- Start of Part 6: Handle Start Command ---
// --- Start of Part 6: Handle Start Command ---
// --- Start of Part 6: Handle Start Command ---
// --- Start of Part 6: Handle Start Command ---
func handleStart(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    userID := update.Message.From.ID
    username := ""
    if update.Message.From != nil && update.Message.From.UserName != "" {
        username = update.Message.From.UserName
    } else if update.Message.From != nil && update.Message.From.FirstName != "" {
        username = update.Message.From.FirstName
    } else {
        username = fmt.Sprintf("User_%d", userID)
    }
    log.Printf("Start command received for user %d with username %s", userID, username)

    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user %d: %v", userID, err)
        sendError(bot, userID, ErrUserNotFound)
        return
    }

    // Handle new user
    if user.UserID == 0 {
        referredBy := int64(0)
        args := update.Message.CommandArguments()
        if args != "" {
            if refID, err := strconv.ParseInt(args, 10, 64); err == nil && refID != userID {
                referredBy = refID
            }
        }
        user = User{
            UserID:      userID,
            Username:    username,
            Balance:     0.0,
            Wallet:      "",
            Referrals:   0,
            ReferredBy:  sql.NullInt64{Int64: referredBy, Valid: referredBy != 0},
            Banned:      0,
            ButtonStyle: "",
            State:       "",
        }
        if err := createUser(db, user); err != nil {
            log.Printf("Error creating user %d with username %s: %v", userID, username, err)
            sendError(bot, userID, ErrUserNotFound, "Failed to create user")
            return
        }
        log.Printf("New user %d (%s) created successfully", userID, username)
        msg := tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("🔔 *New user joined:* @%s (ID: %d)", username, userID))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)

        if referredBy != 0 {
            referrer, err := getUser(db, referredBy)
            if err != nil || referrer.UserID == 0 {
                log.Printf("Referrer %d not found or error: %v", referredBy, err)
            } else {
                rewardStr, _ := getSetting(db, "referral_reward")
                reward, _ := strconv.ParseFloat(rewardStr, 64)
                if reward == 0 {
                    reward = 5.0
                }
                referrer.Balance += reward
                referrer.Referrals++
                if err := updateUser(db, referrer); err != nil {
                    log.Printf("Error updating referrer %d: %v", referredBy, err)
                } else {
                    log.Printf("Referrer %d updated: +%.2f, Referrals: %d", referredBy, reward, referrer.Referrals)
                    msg := tgbotapi.NewMessage(referredBy, formatMarkdownV2("🎉 *Your friend* @%s *joined!*\n*You earned* %.2f 💰\n*New Balance:* %.2f", username, reward, referrer.Balance))
                    msg.ParseMode = "MarkdownV2"
                    bot.Send(msg)
                }
            }
        }
    }

    // Prompt for button style if not set
    if user.ButtonStyle == "" {
        startMessage, _ := getSetting(db, "start_message")
        if startMessage == "" {
            startMessage = "🎉 Welcome to the Referral & Earning Bot\\! Join channels to start\\."
        }
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Inline Buttons", "set_inline"),
                tgbotapi.NewInlineKeyboardButtonData("Normal Buttons", "set_normal"),
            ),
        )
        msg := tgbotapi.NewMessage(userID, startMessage)
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = markup
        bot.Send(msg)
        log.Printf("Prompted user %d for button style", userID)
        return
    }

    // Check channels and show menu
    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels for user %d: %v", userID, err)
        sendError(bot, userID, ErrFetchChannelsFailed)
        return
    }
    if !joined {
        channels, err := getRequiredChannels(db)
        if err != nil {
            log.Printf("Error fetching channels for user %d: %v", userID, err)
            sendError(bot, userID, ErrFetchChannelsFailed)
            return
        }
        if len(channels) > 0 {
            var buttons []tgbotapi.InlineKeyboardButton
            for _, channel := range channels {
                buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Please join:*\n%s", strings.Join(channels, "\n")))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
            log.Printf("User %d prompted to join channels", userID)
            return
        }
    }

    // Show main menu
    log.Printf("Showing main menu for user %d with button style %s", userID, user.ButtonStyle)
    showMainMenu(bot, userID, user.ButtonStyle)
}
// --- End of Part 6: Handle Start Command ---

// --- Part 7: Admin Panel ---
func showAdminPanel(bot *tgbotapi.BotAPI, db *sql.DB, userID int64) {
    markup := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📢 Broadcast", "admin_broadcast"),
            tgbotapi.NewInlineKeyboardButtonData("📊 User Info", "admin_user_info"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("💰 Set Min Withdrawal", "admin_set_min_withdrawal"),
            tgbotapi.NewInlineKeyboardButtonData("📡 Set Payment Channel", "admin_set_payment_channel"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🎁 Set Referral Reward", "admin_set_referral_reward"),
            tgbotapi.NewInlineKeyboardButtonData("🔳 QR Settings", "admin_qr_settings"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("📌 Add Channel", "admin_add_channel"),
            tgbotapi.NewInlineKeyboardButtonData("➖ Remove Channel", "admin_remove_channel"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🚀 Start Message", "admin_start_settings"),
        ),
    )
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🛠 *Admin Panel* 🛠"))
    msg.ParseMode = "MarkdownV2"
    msg.ReplyMarkup = markup
    bot.Send(msg)
}
// --- End of Part 7: Admin Panel ---

// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
// --- Start of Part 8: Handle Update and Menu Options ---
func handleUpdate(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    if update.Message != nil {
        userID := update.Message.From.ID
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        if user.Banned == 1 {
            msg := tgbotapi.NewMessage(userID, "*You are banned!*")
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }

        if user.UserID == 0 {
            username := ""
            if update.Message.From != nil && update.Message.From.UserName != "" {
                username = update.Message.From.UserName
            } else if update.Message.From != nil && update.Message.From.FirstName != "" {
                username = update.Message.From.FirstName
            } else {
                username = fmt.Sprintf("User_%d", userID)
            }
            referredBy := int64(0)
            if update.Message.IsCommand() && update.Message.Command() == "start" {
                args := update.Message.CommandArguments()
                if args != "" {
                    if refID, err := strconv.ParseInt(args, 10, 64); err == nil && refID != userID {
                        referredBy = refID
                    }
                }
            }
            user = User{
                UserID:      userID,
                Username:    username,
                Balance:     0.0,
                Wallet:      "",
                Referrals:   0,
                ReferredBy:  sql.NullInt64{Int64: referredBy, Valid: referredBy != 0},
                Banned:      0,
                ButtonStyle: "",
                State:       "",
            }
            if err := createUser(db, user); err != nil {
                log.Printf("Error creating user: %v", err)
                return
            }
        }

        if update.Message.IsCommand() {
            switch update.Message.Command() {
            case "start":
                handleStart(bot, db, update)
            case "admin":
                if userID == ADMIN_ID {
                    showAdminPanel(bot, db, userID)
                }
            default:
                if user.State == "" {
                    handleMenuOptions(bot, db, update, user)
                }
            }
        } else if user.State != "" {
            handleStateMessages(bot, db, update, user)
        } else {
            handleMenuOptions(bot, db, update, user) // Process text inputs for both inline and normal users
        }
    } else if update.CallbackQuery != nil {
        handleCallbackQuery(bot, db, update.CallbackQuery)
    }
}

func handleMenuOptions(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels for user %d: %v", userID, err)
        return
    }
    if !joined {
        channels, _ := getRequiredChannels(db)
        if len(channels) > 0 {
            var buttons []tgbotapi.InlineKeyboardButton
            for _, channel := range channels {
                buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
            msg := tgbotapi.NewMessage(userID, "*Please join:*\n"+strings.Join(channels, "\n"))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        }
        return
    }

    switch strings.TrimSpace(update.Message.Text) {
    case "💰 Balance":
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error re-fetching user %d for balance check: %v", userID, err)
            return
        }
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("*Balance:* %.2f\n*Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        if user.ButtonStyle == "inline" {
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                ),
            )
        }
        bot.Send(msg)
    case "💳 Set Wallet":
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("*Your wallet:* `%s`", user.Wallet))
            msg.ParseMode = "MarkdownV2"
            if user.ButtonStyle == "inline" {
                msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                    tgbotapi.NewInlineKeyboardRow(
                        tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
                        tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                    ),
                )
            }
            bot.Send(msg)
        } else {
            msg := tgbotapi.NewMessage(userID, "*Enter your wallet address:*")
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = "setting_wallet"
            updateUser(db, user)
        }
    case "📞 Support":
        msg := tgbotapi.NewMessage(userID, "*Send your message for support:*")
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = "support_message"
        updateUser(db, user)
    case "🔗 Referral":
        referralLink := generateReferralLink(userID)
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("*Referral Link:* `%s`\n*Referrals:* %d", referralLink, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        if user.ButtonStyle == "inline" {
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("📄 View Referrals", "view_referrals"),
                    tgbotapi.NewInlineKeyboardButtonURL("📤 Share Link", fmt.Sprintf("https://t.me/share/url?url=%s", referralLink)),
                ),
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                ),
            )
        }
        bot.Send(msg)
    case "📈 Stats":
        totalUsers, _ := getTotalUsers(db)
        totalWithdrawals, _ := getTotalWithdrawals(db)
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("*Stats:*\n*Total Users:* %d\n*Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        if user.ButtonStyle == "inline" {
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                ),
            )
        }
        bot.Send(msg)
    case "💸 Withdraw":
        if user.Wallet == "" {
            msg := tgbotapi.NewMessage(userID, "*Set your wallet first!*")
            msg.ParseMode = "MarkdownV2"
            if user.ButtonStyle == "inline" {
                msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                    tgbotapi.NewInlineKeyboardRow(
                        tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                    ),
                )
            }
            bot.Send(msg)
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, fmt.Sprintf("*Minimum withdrawal:* %.2f", minWithdrawal))
                msg.ParseMode = "MarkdownV2"
                if user.ButtonStyle == "inline" {
                    msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                        tgbotapi.NewInlineKeyboardRow(
                            tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                        ),
                    )
                }
                bot.Send(msg)
            } else {
                msg := tgbotapi.NewMessage(userID, "*Enter amount to withdraw:*")
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
                user.State = "withdraw_amount"
                updateUser(db, user)
            }
        }
    }
}
// --- End of Part 8: Handle Update and Menu Options ---


// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
// --- Start of Part 9: Handle Callback Query ---
func handleCallbackQuery(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user %d: %v", userID, err)
        bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Error"))
        return
    }

    if user.Banned == 1 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *You are banned!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, "🚫 Banned"))
        return
    }

    if callback.Data == "set_inline" || callback.Data == "set_normal" {
        buttonStyle := "inline"
        if callback.Data == "set_normal" {
            buttonStyle = "normal"
        }
        user.ButtonStyle = buttonStyle
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user %d button style to %s: %v", userID, buttonStyle, err)
            bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Error"))
            return
        }
        log.Printf("Button style set to %s for user %d", buttonStyle, userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Button style set to* %s\\!", buttonStyle))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Style set"))
        // Immediately show main menu after setting style
        log.Printf("Showing main menu after style set for user %d", userID)
        showMainMenu(bot, userID, buttonStyle)
        return
    }

    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels for %d: %v", userID, err)
        bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Error"))
        return
    }
    if !joined {
        channels, _ := getRequiredChannels(db)
        if len(channels) > 0 {
            var buttons []tgbotapi.InlineKeyboardButton
            for _, channel := range channels {
                buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Please join:*\n%s", strings.Join(channels, "\n")))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        }
        bot.Request(tgbotapi.NewCallback(callback.ID, "📢 Join channels"))
        return
    }

    backButton := tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
        ),
    )

    switch callback.Data {
    case "back_to_menu":
        showMainMenu(bot, userID, user.ButtonStyle)
        bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Menu"))
    case "change_style":
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Inline Buttons", "set_inline"),
                tgbotapi.NewInlineKeyboardButtonData("Normal Buttons", "set_normal"),
            ),
        )
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🎨 *Choose your button style:*"))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = markup
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Style selection"))
    case "balance":
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error re-fetching user %d for balance callback: %v", userID, err)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = backButton
        bot.Send(msg)
    case "set_wallet":
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Your wallet:* `%s`", user.Wallet))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
                    tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
                ),
            )
            bot.Send(msg)
        } else {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Enter your wallet address:*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = "setting_wallet"
            updateUser(db, user)
        }
    case "change_wallet":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Enter new wallet address:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = "setting_wallet"
        updateUser(db, user)
    case "support":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📞 *Send your message for support:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = "support_message"
        updateUser(db, user)
    case "referral":
        referralLink := generateReferralLink(userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🔗 *Referral Link:* `%s`\n🤝 *Referrals:* %d", referralLink, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📄 View Referrals", "view_referrals"),
                tgbotapi.NewInlineKeyboardButtonURL("📤 Share Link", fmt.Sprintf("https://t.me/share/url?url=%s", referralLink)),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("⬅️ Back to Menu", "back_to_menu"),
            ),
        )
        bot.Send(msg)
    case "view_referrals":
        rows, err := db.Query("SELECT username FROM users WHERE referred_by = ?", userID)
        if err != nil {
            log.Printf("Error getting referrals for user %d: %v", userID, err)
            bot.Request(tgbotapi.NewCallback(callback.ID, "❌ Error"))
            return
        }
        defer rows.Close()
        var referrals []string
        for rows.Next() {
            var username string
            if err := rows.Scan(&username); err != nil {
                log.Printf("Error scanning referral for user %d: %v", userID, err)
                continue
            }
            referrals = append(referrals, "@"+username)
        }
        msgText := formatMarkdownV2("📄 *Your referrals:*\n%s", strings.Join(referrals, "\n"))
        if len(referrals) == 0 {
            msgText = formatMarkdownV2("📄 *No referrals yet!*")
        }
        msg := tgbotapi.NewMessage(userID, msgText)
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = backButton
        bot.Send(msg)
    case "stats":
        totalUsers, _ := getTotalUsers(db)
        totalWithdrawals, _ := getTotalWithdrawals(db)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = backButton
        bot.Send(msg)
    case "withdraw":
        if user.Wallet == "" {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Set your wallet first!*"))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = backButton
            bot.Send(msg)
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
                msg.ParseMode = "MarkdownV2"
                msg.ReplyMarkup = backButton
                bot.Send(msg)
            } else {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Enter amount to withdraw:*"))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
                user.State = "withdraw_amount"
                updateUser(db, user)
            }
        }
    }

    if strings.HasPrefix(callback.Data, "admin_") {
        handleAdminActions(bot, db, callback)
    } else if callback.Data == "qr_enable" || callback.Data == "qr_disable" {
        handleQRSettings(bot, db, callback)
    } else if strings.HasPrefix(callback.Data, "adjust_") || strings.HasPrefix(callback.Data, "ban_") || strings.HasPrefix(callback.Data, "unban_") || strings.HasPrefix(callback.Data, "viewrefs_") || strings.HasPrefix(callback.Data, "contact_") {
        handleAdminUserActions(bot, db, callback)
    }

    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}
// --- End of Part 9: Handle Callback Query ---

// --- Start of Part 9B: Additional Helper Functions ---
// --- Start of Part 9B: Additional Helper Functions ---
// --- Start of Part 9B: Additional Helper Functions ---
// --- Start of Part 9B: Additional Helper Functions ---
func getAllUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var users []User
    for rows.Next() {
        var u User
        err := rows.Scan(&u.UserID, &u.Username, &u.Balance, &u.Wallet, &u.Referrals, &u.ReferredBy, &u.Banned, &u.ButtonStyle, &u.State)
        if err != nil {
            return nil, err
        }
        users = append(users, u)
    }
    return users, nil
}

func getUserByUsername(db *sql.DB, username string) (User, error) {
    var user User
    query := `SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state 
              FROM users WHERE username = ?`
    err := db.QueryRow(query, username).Scan(&user.UserID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State)
    if err == sql.ErrNoRows {
        return User{}, nil
    }
    return user, err
}

func handleQRSettings(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized!* [E001]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    if callback.Data == "qr_enable" {
        updateSetting(db, "qr_enabled", "1")
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *QR Codes Enabled!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    } else if callback.Data == "qr_disable" {
        updateSetting(db, "qr_enabled", "0")
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *QR Codes Disabled!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func handleAdminUserActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized!* [E001]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }

    data := callback.Data
    log.Printf("Admin action callback received: %s", data)
    action := strings.Split(data, "_")[0]
    targetUserIDStr := strings.TrimPrefix(data, action+"_")
    targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
    if err != nil {
        log.Printf("Error parsing target user ID from %s: %v", targetUserIDStr, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID in callback!* [E002]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }

    targetUser, err := getUser(db, targetUserID)
    if err != nil || targetUser.UserID == 0 {
        log.Printf("Error fetching target user %d: %v", targetUserID, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found!* [E003]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }

    switch action {
    case "adjust":
        log.Printf("Adjust balance requested for user %d", targetUserID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Enter amount to adjust for user %d (+ for add, - for deduct):*", targetUserID))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        adminUser := User{UserID: userID, State: "adjusting_balance_" + targetUserIDStr}
        if err := updateUser(db, adminUser); err != nil {
            log.Printf("Error setting state for admin %d: %v", userID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set state!* [E004]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
    case "ban":
        log.Printf("Ban requested for user %d", targetUserID)
        targetUser.Banned = 1
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error banning user %d: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to ban user %d!* [E005]", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *User* %d *banned!*", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("🚫 *You have been banned!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
    case "unban":
        log.Printf("Unban requested for user %d", targetUserID)
        targetUser.Banned = 0
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error unbanning user %d: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to unban user %d!* [E006]", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *User* %d *unbanned!*", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("✅ *You have been unbanned!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
    case "viewrefs":
        log.Printf("View referrals requested for user %d", targetUserID)
        rows, err := db.Query("SELECT username FROM users WHERE referred_by = ?", targetUserID)
        if err != nil {
            log.Printf("Error getting referrals for user %d: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error fetching referrals!* [E007]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        defer rows.Close()
        var referrals []string
        for rows.Next() {
            var username string
            if err := rows.Scan(&username); err != nil {
                log.Printf("Error scanning referral for user %d: %v", targetUserID, err)
                continue
            }
            referrals = append(referrals, "@"+username)
        }
        msgText := formatMarkdownV2("📄 *Referrals for* @%s:\n%s", targetUser.Username, strings.Join(referrals, "\n"))
        if len(referrals) == 0 {
            msgText = formatMarkdownV2("📄 *Referrals for* @%s:\n*No referrals yet!*", targetUser.Username)
        }
        msg := tgbotapi.NewMessage(userID, msgText)
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "contact":
        log.Printf("Contact requested for user %d", targetUserID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📩 *Enter message to send to user* %d:", targetUserID))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        adminUser := User{UserID: userID, State: "contacting_" + targetUserIDStr}
        if err := updateUser(db, adminUser); err != nil {
            log.Printf("Error setting state for admin %d: %v", userID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set contact state!* [E008]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            log.Printf("State set to contacting_%s for admin %d", targetUserIDStr, userID)
        }
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}
// --- End of Part 9B: Additional Helper Functions ---

// --- Start of Part 10: Handle State Messages and Admin Actions ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
// --- Start of Part 10A: Handle State Messages (User States) and QR Code ---
func createQRCode(data string) ([]byte, error) {
    qr, err := qrcode.New(data, qrcode.Medium)
    if err != nil {
        return nil, err
    }
    return qr.PNG(256)
}

func handleStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    state := user.State

    switch state {
    case "setting_wallet":
        wallet := strings.TrimSpace(update.Message.Text)
        if strings.HasPrefix(wallet, "💰") || strings.HasPrefix(wallet, "💳") || strings.HasPrefix(wallet, "📞") ||
           strings.HasPrefix(wallet, "🔗") || strings.HasPrefix(wallet, "📈") || strings.HasPrefix(wallet, "💸") {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Please enter a valid wallet address, not a menu option!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        if len(wallet) < 5 {
            sendError(bot, userID, ErrWalletTooShort)
            return
        }
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error re-fetching user %d for wallet update: %v", userID, err)
            sendError(bot, userID, ErrUserNotFound)
            return
        }
        user.Wallet = wallet
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating wallet for user %d: %v", userID, err)
            sendError(bot, userID, ErrWalletUpdateFailed)
            return
        }
        log.Printf("Wallet set to %s for user %d", wallet, userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Wallet set to:* `%s`", wallet))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        showMainMenu(bot, userID, user.ButtonStyle)

    case "support_message":
        supportMsg := strings.TrimSpace(update.Message.Text)
        if supportMsg == "" {
            sendError(bot, userID, ErrEmptyMessage)
            return
        }
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error re-fetching user %d for support: %v", userID, err)
            sendError(bot, userID, ErrUserNotFound)
            return
        }
        // Clear state first to avoid getting stuck
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error clearing state for user %d after support: %v", userID, err)
            sendError(bot, userID, ErrStateNotCleared)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Message sent to support\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)

        adminMsg := tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("📩 *Support message from* @%s \\(ID: %d\\):\n%s", user.Username, userID, supportMsg))
        adminMsg.ParseMode = "MarkdownV2"
        adminMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Ban User", fmt.Sprintf("ban_%d", userID)),
            ),
        )
        if _, err := bot.Send(adminMsg); err != nil {
            log.Printf("Error sending support message to admin %d from user %d: %v", ADMIN_ID, userID, err)
            errorMsg := tgbotapi.NewMessage(userID, formatMarkdownV2("⚠️ *Support message submitted, but admin notification failed. Please wait for a response!*"))
            errorMsg.ParseMode = "MarkdownV2"
            bot.Send(errorMsg)
        } else {
            log.Printf("Support message from user %d sent to admin %d", userID, ADMIN_ID)
        }
        showMainMenu(bot, userID, user.ButtonStyle)

    case "withdraw_amount":
        amountStr := strings.TrimSpace(update.Message.Text)
        amount, err := strconv.ParseFloat(amountStr, 64)
        if err != nil || amount <= 0 {
            sendError(bot, userID, ErrInvalidAmount)
            return
        }
        minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
        minWithdrawal, err := strconv.ParseFloat(minWithdrawalStr, 64)
        if err != nil {
            minWithdrawal = 10.0
        }
        if amount < minWithdrawal {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error re-fetching user %d for withdrawal: %v", userID, err)
            sendError(bot, userID, ErrUserNotFound)
            return
        }
        if user.Balance < amount {
            sendError(bot, userID, ErrInsufficientBalance)
            return
        }
        user.Balance -= amount
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user %d balance: %v", userID, err)
            sendError(bot, userID, ErrAdjustBalanceFailed)
            return
        }
        if err := createWithdrawal(db, userID, amount, user.Wallet); err != nil {
            log.Printf("Error creating withdrawal for user %d: %v", userID, err)
            sendError(bot, userID, "E999")
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Withdrawal request sent\\! Admin will review soon!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        showMainMenu(bot, userID, user.ButtonStyle)

        paymentChannel, _ := getSetting(db, "payment_channel")
        if paymentChannel == "" {
            paymentChannel = "@DefaultChannel"
        }
        paymentChatID, err := getChatIDFromUsername(bot, paymentChannel)
        if err != nil {
            log.Printf("Error resolving payment channel %s: %v", paymentChannel, err)
            paymentChatID = ADMIN_ID
            bot.Send(tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("⚠️ *Failed to resolve payment channel %s!*", paymentChannel)))
        }

        firstName := user.Username
        if update.Message.From != nil && update.Message.From.FirstName != "" {
            firstName = update.Message.From.FirstName
        }
        txID := fmt.Sprintf("2025%07d", rand.Intn(10000000))
        channels, _ := getRequiredChannels(db)
        channelURL := "@DefaultChannel"
        if len(channels) > 0 {
            channelURL = channels[0]
        }
        notification := formatMarkdownV2(
            "🔥 *NEW WITHDRAWAL SENT* 🔥\n\n"+
                "👤 *USER:* [%s](tg://user?id=%d)\n"+
                "💎 *USER ID:* `%d`\n"+
                "💰 *AMOUNT:* %.2f FREE COIN\n"+
                "📞 *REFERRER:* %d\n"+
                "🔗 *ADDRESS:* `%s`\n"+
                "⏰ *TRANSACTION ID:* `%s`",
            firstName, userID, userID, amount, user.Referrals, user.Wallet, txID,
        )
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonURL("🔍CHANN", "https://t.me/"+strings.TrimPrefix(channelURL, "@")),
                tgbotapi.NewInlineKeyboardButtonURL("JOIN", "https://t.me/"+strings.TrimPrefix(BOT_USERNAME, "@")),
            ),
        )
        qrEnabled, _ := getSetting(db, "qr_enabled")
        if qrEnabled == "1" {
            qrBytes, err := createQRCode(user.Wallet)
            if err != nil {
                log.Printf("Error generating QR code for user %d: %v", userID, err)
                msg := tgbotapi.NewMessage(paymentChatID, notification+"\n⚠️ *QR code generation failed!*")
                msg.ParseMode = "MarkdownV2"
                msg.ReplyMarkup = markup
                bot.Send(msg)
                bot.Send(tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("⚠️ *QR code generation failed for user %d!*", userID)))
            } else {
                photo := tgbotapi.NewPhoto(paymentChatID, tgbotapi.FileBytes{Name: "qr_withdrawal.png", Bytes: qrBytes})
                photo.Caption = notification
                photo.ParseMode = "MarkdownV2"
                photo.ReplyMarkup = markup
                bot.Send(photo)
            }
        } else {
            msg := tgbotapi.NewMessage(paymentChatID, notification)
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        }

    default:
        handleAdminStateMessages(bot, db, update, user)
    }
}
// --- End of Part 10A: Handle State Messages (User States) and QR Code ---


// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// --- Start of Part 10B: Handle State Messages (Admin States) and Admin Actions ---
// Note: Imports are assumed to be in Part 1 (e.g., "database/sql", "log", "strconv", etc.)

func handleAdminStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    state := user.State
    log.Printf("Admin state received: %s for user %d", state, userID)

    if userID != ADMIN_ID {
        log.Printf("Unauthorized access attempt by user %d", userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized!* [E001]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }

    switch {
    case strings.HasPrefix(state, "broadcast_message"):
        users, err := getAllUsers(db)
        if err != nil {
            log.Printf("Error getting users for broadcast: %v", err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to fetch users for broadcast!* [E018]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        totalUsers := 0
        for _, u := range users {
            if u.Banned == 0 {
                totalUsers++
            }
        }
        successCount := 0
        statusMsg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Broadcasting:* [□□□□□□□□□□] 0%%"))
        statusMsg.ParseMode = "MarkdownV2"
        sentStatus, _ := bot.Send(statusMsg)

        for _, u := range users {
            if u.Banned == 0 {
                var sent bool
                if update.Message.Photo != nil && len(update.Message.Photo) > 0 {
                    photo := tgbotapi.NewPhoto(u.UserID, tgbotapi.FileID(update.Message.Photo[len(update.Message.Photo)-1].FileID))
                    photo.Caption = update.Message.Caption
                    photo.ParseMode = "MarkdownV2"
                    if _, err := bot.Send(photo); err == nil {
                        sent = true
                    }
                } else if update.Message.Video != nil {
                    video := tgbotapi.NewVideo(u.UserID, tgbotapi.FileID(update.Message.Video.FileID))
                    video.Caption = update.Message.Caption
                    video.ParseMode = "MarkdownV2"
                    if _, err := bot.Send(video); err == nil {
                        sent = true
                    }
                } else if update.Message.Document != nil {
                    doc := tgbotapi.NewDocument(u.UserID, tgbotapi.FileID(update.Message.Document.FileID))
                    doc.Caption = update.Message.Caption
                    doc.ParseMode = "MarkdownV2"
                    if _, err := bot.Send(doc); err == nil {
                        sent = true
                    }
                } else {
                    msg := tgbotapi.NewMessage(u.UserID, update.Message.Text)
                    msg.ParseMode = "MarkdownV2"
                    if _, err := bot.Send(msg); err == nil {
                        sent = true
                    }
                }
                if sent {
                    successCount++
                }
                progress := int((float64(successCount) / float64(totalUsers)) * 10)
                bar := strings.Repeat("█", progress) + strings.Repeat("□", 10-progress)
                percentage := int((float64(successCount) / float64(totalUsers)) * 100)
                bot.Send(tgbotapi.NewEditMessageText(userID, sentStatus.MessageID, formatMarkdownV2("📢 *Broadcasting:* [%s] %d%% (%d/%d)", bar, percentage, successCount, totalUsers)))
                time.Sleep(100 * time.Millisecond)
            }
        }
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error clearing broadcast state for admin %d: %v", userID, err)
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Broadcast completed! Sent to* %d/%d *users!*", successCount, totalUsers))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)

    case strings.HasPrefix(state, "getting_user_info"):
        input := strings.TrimSpace(update.Message.Text)
        var targetUser User
        var err error
        if strings.HasPrefix(input, "@") {
            targetUser, err = getUserByUsername(db, strings.TrimPrefix(input, "@"))
        } else {
            targetUserID, err := strconv.ParseInt(input, 10, 64)
            if err != nil {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID or username!* [E009]"))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
                return
            }
            targetUser, err = getUser(db, targetUserID)
        }
        if err != nil || targetUser.UserID == 0 {
            log.Printf("User not found for input %s: %v", input, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found!* [E003]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        status := "Active"
        if targetUser.Banned == 1 {
            status = "Banned"
        }
        msgText := formatMarkdownV2("📊 *User Info*\n*ID:* %d\n*Username:* @%s\n*Balance:* %.2f\n*Referrals:* %d\n*Wallet:* `%s`\n*Status:* %s",
            targetUser.UserID, targetUser.Username, targetUser.Balance, targetUser.Referrals, targetUser.Wallet, status)
        msg := tgbotapi.NewMessage(userID, msgText)
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("💰 Adjust Balance", fmt.Sprintf("adjust_%d", targetUser.UserID)),
                tgbotapi.NewInlineKeyboardButtonData("📄 View Referrals", fmt.Sprintf("viewrefs_%d", targetUser.UserID)),
            ),
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📩 Contact", fmt.Sprintf("contact_%d", targetUser.UserID)),
                tgbotapi.NewInlineKeyboardButtonData("🚫 Ban", fmt.Sprintf("ban_%d", targetUser.UserID)),
                tgbotapi.NewInlineKeyboardButtonData("✅ Unban", fmt.Sprintf("unban_%d", targetUser.UserID)),
            ),
        )
        bot.Send(msg)
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error clearing getting_user_info state for admin %d: %v", userID, err)
        }

    case strings.HasPrefix(state, "adjusting_balance_"):
        targetUserIDStr := strings.TrimPrefix(state, "adjusting_balance_")
        targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
        if err != nil {
            log.Printf("Invalid target user ID in state %s: %v", state, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID!* [E002]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        targetUser, err := getUser(db, targetUserID)
        if err != nil || targetUser.UserID == 0 {
            log.Printf("User %d not found for balance adjustment: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found!* [E003]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        adjustment := strings.TrimSpace(update.Message.Text)
        log.Printf("Received adjustment input: %s for user %d", adjustment, targetUserID)
        if len(adjustment) == 0 || (adjustment[0] != '+' && adjustment[0] != '-') {
            log.Printf("Invalid adjustment format: %s", adjustment)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Enter a valid amount (e.g., +10 or -5)!* [E010]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        amount, err := strconv.ParseFloat(adjustment[1:], 64)
        if err != nil || amount < 0 {
            log.Printf("Invalid amount parsing %s: %v", adjustment[1:], err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount!* [E011]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        if adjustment[0] == '-' && targetUser.Balance < amount {
            log.Printf("Insufficient balance for user %d: Current %.2f, Requested %.2f", targetUserID, targetUser.Balance, amount)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Insufficient balance to deduct!* [E012]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        oldBalance := targetUser.Balance
        if adjustment[0] == '+' {
            targetUser.Balance += amount
        } else {
            targetUser.Balance -= amount
        }
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error updating balance for user %d in database: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to adjust balance in database!* [E013]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        // Verify balance update
        updatedUser, err := getUser(db, targetUserID)
        if err != nil || updatedUser.Balance != targetUser.Balance {
            log.Printf("Balance verification failed for user %d: Expected %.2f, Got %.2f, Error: %v", targetUserID, targetUser.Balance, updatedUser.Balance, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Balance update verification failed!* [E013]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        log.Printf("Balance adjusted for user %d: Old balance %.2f, New balance %.2f", targetUserID, oldBalance, targetUser.Balance)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Balance adjusted for user* %d!\n*Old Balance:* %.2f\n*New Balance:* %.2f", targetUserID, oldBalance, targetUser.Balance))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("💰 *Your balance has been updated!*\n*New Balance:* %.2f", targetUser.Balance))
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error notifying user %d of balance update: %v", targetUserID, err)
        }
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error clearing adjusting_balance state for admin %d: %v", userID, err)
        } else {
            log.Printf("State cleared for admin %d after balance adjustment", userID)
        }

    case strings.HasPrefix(state, "contacting_"):
        targetUserIDStr := strings.TrimPrefix(state, "contacting_")
        targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
        if err != nil {
            log.Printf("Invalid target user ID in state %s: %v", state, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID!* [E002]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        targetUser, err := getUser(db, targetUserID)
        if err != nil || targetUser.UserID == 0 {
            log.Printf("User %d not found for contact: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found!* [E003]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        message := strings.TrimSpace(update.Message.Text)
        log.Printf("Received contact message: %s for user %d", message, targetUserID)
        if message == "" {
            log.Printf("Empty message received for user %d", targetUserID)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Message cannot be empty!* [E014]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        msg := tgbotapi.NewMessage(targetUserID, formatMarkdownV2("📩 *Message from Admin:*\n%s", message))
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending message to user %d: %v", targetUserID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to send message to user* %d! [E015]", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            log.Printf("Message successfully sent to user %d", targetUserID)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Message sent to user* %d!", targetUserID))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error clearing contacting state for admin %d: %v", userID, err)
        } else {
            log.Printf("State cleared for admin %d after contacting", userID)
        }

    case strings.HasPrefix(state, "setting_min_withdrawal"):
        amountStr := strings.TrimSpace(update.Message.Text)
        amount, err := strconv.ParseFloat(amountStr, 64)
        if err != nil || amount < 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount!* [E011]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        updateSetting(db, "min_withdrawal", fmt.Sprintf("%.2f", amount))
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Minimum withdrawal set to:* %.2f", amount))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)

    case strings.HasPrefix(state, "setting_payment_channel"):
        channel := strings.TrimSpace(update.Message.Text)
        if !strings.HasPrefix(channel, "@") {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Use '@' (e.g., @ChannelName)!* [E016]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        _, err := bot.MakeRequest("getChat", tgbotapi.Params{"chat_id": channel})
        if err != nil {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid channel or bot lacks access!* [E017]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        updateSetting(db, "payment_channel", channel)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Payment Channel set to:* %s", channel))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)

    case strings.HasPrefix(state, "setting_referral_reward"):
        amountStr := strings.TrimSpace(update.Message.Text)
        amount, err := strconv.ParseFloat(amountStr, 64)
        if err != nil || amount < 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount!* [E011]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        updateSetting(db, "referral_reward", fmt.Sprintf("%.2f", amount))
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Referral reward set to:* %.2f", amount))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)

    case strings.HasPrefix(state, "add_channel"):
        channel := strings.TrimSpace(update.Message.Text)
        if !strings.HasPrefix(channel, "@") {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Channel must start with '@' (e.g., @ChannelName)!* [E016]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        channels, err := getRequiredChannels(db)
        if err != nil {
            log.Printf("Error fetching channels: %v", err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error checking channels!* [E018]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        for _, ch := range channels {
            if ch == channel {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Channel* %s *already added!* [E019]", channel))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
                user.State = ""
                updateUser(db, user)
                return
            }
        }
        _, err = bot.MakeRequest("getChat", tgbotapi.Params{"chat_id": channel})
        if err != nil {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid channel or bot lacks access!* [E017]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        err = addRequiredChannel(db, channel)
        if err != nil {
            log.Printf("Error adding channel %s: %v", channel, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to add channel!* [E020]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➕ *Channel* %s *added successfully!*", channel))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)

    case strings.HasPrefix(state, "remove_channel"):
        channel := strings.TrimSpace(update.Message.Text)
        if !strings.HasPrefix(channel, "@") {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Channel must start with '@' (e.g., @ChannelName)!* [E016]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        channels, err := getRequiredChannels(db)
        if err != nil {
            log.Printf("Error fetching channels: %v", err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error checking channels!* [E018]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        found := false
        for _, ch := range channels {
            if ch == channel {
                found = true
                break
            }
        }
        if !found {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Channel* %s *not found!* [E021]", channel))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        err = removeRequiredChannel(db, channel)
        if err != nil {
            log.Printf("Error removing channel %s: %v", channel, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to remove channel!* [E022]"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➖ *Channel* %s *removed successfully!*", channel))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)

    case strings.HasPrefix(state, "setting_start_message"):
        startMessage := update.Message.Text
        updateSetting(db, "start_message", startMessage)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Start message updated!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)
    }
}

func handleAdminActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized!* [E001]"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    action := callback.Data
    switch action {
    case "admin_broadcast":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Send message or media to broadcast:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "broadcast_message"})
    case "admin_user_info":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📊 *Enter user ID or username:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "getting_user_info"})
    case "admin_set_min_withdrawal":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Enter new minimum withdrawal:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_min_withdrawal"})
    case "admin_set_payment_channel":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📡 *Enter payment channel (e.g., @Channel):*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_payment_channel"})
    case "admin_set_referral_reward":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🎁 *Enter referral reward amount:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_referral_reward"})
    case "admin_add_channel":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📌 *Enter channel username (e.g., @Channel):*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "add_channel"})
    case "admin_remove_channel":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➖ *Enter channel username (e.g., @Channel):*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "remove_channel"})
    case "admin_start_settings":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚀 *Enter new start message:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_start_message"})
    case "admin_qr_settings":
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Enable QR", "qr_enable"),
                tgbotapi.NewInlineKeyboardButtonData("Disable QR", "qr_disable"),
            ),
        )
        qrStatus := "Disabled"
        if qrEnabled, _ := getSetting(db, "qr_enabled"); qrEnabled == "1" {
            qrStatus = "Enabled"
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🔳 *QR Status:* %s", qrStatus))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = markup
        bot.Send(msg)
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}
// --- End of Part 10B: Handle



// --- Start of Part 12: Error Codes ---
// --- Start of Part 12: Error Codes ---

// Error codes for consistent error handling and reporting
const (
    ErrUnauthorized         = "E001" // Unauthorized access attempt
    ErrInvalidUserID        = "E002" // Invalid user ID in callback or state
    ErrUserNotFound         = "E003" // User not found in database
    ErrStateSetFailed       = "E004" // Failed to set user state
    ErrBanFailed            = "E005" // Failed to ban user
    ErrUnbanFailed          = "E006" // Failed to unban user
    ErrFetchReferralsFailed = "E007" // Failed to fetch referrals
    ErrContactStateFailed   = "E008" // Failed to set contact state
    ErrInvalidInput         = "E009" // Invalid user ID or username input
    ErrInvalidAmountFormat  = "E010" // Invalid amount format (e.g., missing + or -)
    ErrInvalidAmount        = "E011" // Invalid amount (parsing error or negative)
    ErrInsufficientBalance  = "E012" // Insufficient balance to deduct
    ErrAdjustBalanceFailed  = "E013" // Failed to adjust balance in database or verification
    ErrEmptyMessage         = "E014" // Message cannot be empty for contact or broadcast
    ErrSendMessageFailed    = "E015" // Failed to send message to user
    ErrInvalidChannelFormat = "E016" // Channel must start with @
    ErrInvalidChannel       = "E017" // Invalid channel or bot lacks access
    ErrFetchChannelsFailed  = "E018" // Failed to fetch required channels
    ErrChannelAlreadyAdded  = "E019" // Channel already added
    ErrAddChannelFailed     = "E020" // Failed to add channel to database
    ErrChannelNotFound      = "E021" // Channel not found for removal
    ErrRemoveChannelFailed  = "E022" // Failed to remove channel from database
    ErrWalletTooShort       = "E023" // Wallet address too short (less than 5 characters)
    ErrWalletUpdateFailed   = "E024" // Failed to update wallet in database
    ErrStateNotCleared      = "E025" // Failed to clear user state after action
    ErrKeyboardPersistFailed= "E026" // Failed to persist normal keyboard
)

// ErrorMessages maps error codes to user-friendly messages
var ErrorMessages = map[string]string{
    ErrUnauthorized:         "🚫 *Unauthorized access attempt!*",
    ErrInvalidUserID:        "❌ *Invalid user ID!*",
    ErrUserNotFound:         "❌ *User not found!*",
    ErrStateSetFailed:       "❌ *Failed to set state!*",
    ErrBanFailed:            "❌ *Failed to ban user!*",
    ErrUnbanFailed:          "❌ *Failed to unban user!*",
    ErrFetchReferralsFailed: "❌ *Failed to fetch referrals!*",
    ErrContactStateFailed:   "❌ *Failed to set contact state!*",
    ErrInvalidInput:         "❌ *Invalid user ID or username!*",
    ErrInvalidAmountFormat:  "❌ *Enter a valid amount (e.g., +10 or -5)!*",
    ErrInvalidAmount:        "❌ *Invalid amount!*",
    ErrInsufficientBalance:  "❌ *Insufficient balance!*",
    ErrAdjustBalanceFailed:  "❌ *Failed to adjust balance!*",
    ErrEmptyMessage:         "❌ *Message cannot be empty!*",
    ErrSendMessageFailed:    "❌ *Failed to send message!*",
    ErrInvalidChannelFormat: "❌ *Channel must start with '@' (e.g., @ChannelName)!*",
    ErrInvalidChannel:       "❌ *Invalid channel or bot lacks access!*",
    ErrFetchChannelsFailed:  "❌ *Error fetching channels!*",
    ErrChannelAlreadyAdded:  "❌ *Channel already added!*",
    ErrAddChannelFailed:     "❌ *Failed to add channel!*",
    ErrChannelNotFound:      "❌ *Channel not found!*",
    ErrRemoveChannelFailed:  "❌ *Failed to remove channel!*",
    ErrWalletTooShort:       "❌ *Wallet address too short! Minimum 5 characters!*",
    ErrWalletUpdateFailed:   "❌ *Failed to save wallet!*",
    ErrStateNotCleared:      "❌ *Failed to clear state!*",
    ErrKeyboardPersistFailed:"❌ *Failed to keep keyboard visible!*",
}

// sendError sends an error message to the user with the specified error code
func sendError(bot *tgbotapi.BotAPI, userID int64, errCode string, additionalInfo ...interface{}) {
    msgText := ErrorMessages[errCode]
    if len(additionalInfo) > 0 {
        msgText = formatMarkdownV2(msgText+" "+fmt.Sprint(additionalInfo...), nil)
    } else {
        msgText = formatMarkdownV2(msgText)
    }
    msg := tgbotapi.NewMessage(userID, msgText+" ["+errCode+"]")
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    log.Printf("Error sent to user %d: %s [%s]", userID, ErrorMessages[errCode], errCode)
}
// --- End of Part 12: Error Codes ---

// --- Start of Part 11: Main Function ---
// --- Start of Part 11: Main Function ---
func main() {
    var err error // Since db is package-level, we only need err locally
    db, err = initDB() // Assign to the global db variable
    if err != nil {
        log.Fatalf("Failed to initialize database: %v", err)
    }
    defer db.Close()

    bot, err := tgbotapi.NewBotAPI(BOT_TOKEN)
    if err != nil {
        log.Fatalf("Failed to initialize bot: %v", err)
    }
    bot.Debug = false
    log.Printf("Authorized on account %s", bot.Self.UserName)
    BOT_USERNAME = "@" + bot.Self.UserName

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        handleUpdate(bot, db, update)
    }
}
// --- End of Part 11: Main Function ---
