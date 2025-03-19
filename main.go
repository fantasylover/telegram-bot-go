// Part 1 Starting
package main

import (
    "bytes"
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

var BOT_USERNAME = "@Superbv2_bot"

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
// Part 1 Ending

// Part 2 Starting
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
        return User{}, nil
    }
    return user, err
}

func updateUser(db *sql.DB, user User) error {
    query := `INSERT OR REPLACE INTO users (user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state) 
              VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
    _, err := db.Exec(query, user.UserID, user.Username, user.Balance, user.Wallet, user.Referrals, user.ReferredBy, user.Banned, user.ButtonStyle, user.State)
    return err
}

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
// Part 2 Ending


// Part 3 Starting
// Part 3 Starting
// Part 3 Starting
// Part 3 Starting
// Part 3 Starting

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
    // Escape all Telegram MarkdownV2 reserved characters
    charsToEscape := "_*[]()~`>#-+=|{.!"
    for _, char := range charsToEscape {
        text = strings.ReplaceAll(text, string(char), "\\"+string(char))
    }
    return text
}

func formatMarkdownV2(template string, args ...interface{}) string {
    // Format the string with arguments, then escape reserved characters
    formatted := fmt.Sprintf(template, args...)
    return escapeMarkdownV2(formatted)
}

func showMainMenu(bot *tgbotapi.BotAPI, userID int64, buttonStyle string) {
    var markup interface{}
    if buttonStyle == "inline" {
        inlineMarkup := tgbotapi.NewInlineKeyboardMarkup(
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
        markup = inlineMarkup
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
        markup = replyMarkup
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✨ *Main Menu* ✨"))
    msg.ParseMode = "MarkdownV2"
    msg.ReplyMarkup = markup
    bot.Send(msg)
}
// Part 3 Ending

// Part 4 Starting
// Part 4 Starting
// Part 4 Starting
// Part 4 Starting
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

    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user %d: %v", userID, err)
        return
    }

    referredBy := int64(0)
    args := update.Message.CommandArguments()
    if args != "" {
        if refID, err := strconv.ParseInt(args, 10, 64); err == nil && refID != userID {
            referredBy = refID
        }
    }

    if user.UserID == 0 { // New user
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
            log.Printf("Error creating user %d: %v", userID, err)
            return
        }
        // Admin notification for new user
        msg := tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("🔔 *New user joined:* @%s (ID: %d)", username, userID))
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending new user notification to admin %d: %v", ADMIN_ID, err)
        } else {
            log.Printf("New user notification sent to admin for user %d", userID)
        }
        // Referral reward
        if referredBy != 0 {
            referrer, err := getUser(db, referredBy)
            if err != nil {
                log.Printf("Error getting referrer %d: %v", referredBy, err)
            } else if referrer.UserID != 0 {
                rewardStr, err := getSetting(db, "referral_reward")
                if err != nil {
                    log.Printf("Error getting referral_reward: %v", err)
                    rewardStr = "5" // Default fallback
                }
                reward, err := strconv.ParseFloat(rewardStr, 64)
                if err != nil {
                    log.Printf("Error parsing referral reward: %v", err)
                    reward = 5.0 // Default fallback
                }
                referrer.Balance += reward
                referrer.Referrals++
                if err := updateUser(db, referrer); err != nil {
                    log.Printf("Error updating referrer %d: %v", referredBy, err)
                } else {
                    msg := tgbotapi.NewMessage(referredBy, formatMarkdownV2("🎉 *Your friend* @%s *joined!*\n*You earned* %.2f 💰\n*New Balance:* %.2f", username, reward, referrer.Balance))
                    msg.ParseMode = "MarkdownV2"
                    if _, err := bot.Send(msg); err != nil {
                        log.Printf("Error sending referral notification to %d: %v", referredBy, err)
                    } else {
                        log.Printf("Referral notification sent to referrer %d for new user %d", referredBy, userID)
                    }
                }
            }
        }
    }

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
    } else {
        joined, err := checkUserJoinedChannels(bot, userID, db)
        if err != nil {
            log.Printf("Error checking channels for %d: %v", userID, err)
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
                return
            }
        }
        showMainMenu(bot, userID, user.ButtonStyle)
    }
}
// Part 4 Ending


// Part 5 Starting
func main() {
    db, err := initDB()
    if err != nil {
        log.Panic(err)
    }
    defer db.Close()

    bot, err := tgbotapi.NewBotAPI(BOT_TOKEN)
    if err != nil {
        log.Panic(err)
    }
    bot.Debug = true
    BOT_USERNAME = bot.Self.UserName
    log.Printf("Authorized on account @%s", BOT_USERNAME)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        handleUpdate(bot, db, update)
    }
}

func showAdminPanel(bot *tgbotapi.BotAPI, db *sql.DB, userID int64) {
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, "🚫 *Unauthorized*")
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
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
            tgbotapi.NewInlineKeyboardButtonData("📌 Add Channel", "admin_add_channel"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("➖ Remove Channel", "admin_remove_channel"),
            tgbotapi.NewInlineKeyboardButtonData("🚀 Start Settings", "admin_start_settings"),
        ),
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonData("🔳 QR Settings", "admin_qr_settings"),
        ),
    )
    msg := tgbotapi.NewMessage(userID, "🔧 *Admin Panel* 🔧")
    msg.ParseMode = "MarkdownV2"
    msg.ReplyMarkup = markup
    bot.Send(msg)
}
// Part 5 Ending


// Part 6 Starting
func handleUpdate(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    if update.Message != nil {
        userID := update.Message.From.ID
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        if user.Banned == 1 {
            msg := tgbotapi.NewMessage(userID, "🚫 *You are banned!*")
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }

        if user.UserID == 0 { // New user registration
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
                if user.ButtonStyle == "normal" && user.State == "" {
                    handleMenuOptions(bot, db, update, user)
                }
            }
        } else if user.State != "" {
            handleStateMessages(bot, db, update, user)
        } else if user.ButtonStyle == "normal" {
            handleMenuOptions(bot, db, update, user)
        }
    } else if update.CallbackQuery != nil {
        handleCallbackQuery(bot, db, update.CallbackQuery)
    }
}
// Part 6 Ending

// Part 7 Starting
func handleMenuOptions(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    log.Printf("Handling menu option for user %d, received text: %s", userID, update.Message.Text)

    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels for user %d: %v", userID, err)
        msg := tgbotapi.NewMessage(userID, "❌ *Error checking channel status. Try again later.*")
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    log.Printf("User %d joined status: %v", userID, joined)
    if !joined {
        channels, err := getRequiredChannels(db)
        if err != nil {
            log.Printf("Error getting required channels for user %d: %v", userID, err)
            msg := tgbotapi.NewMessage(userID, "❌ *Error fetching required channels.*")
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        log.Printf("Required channels for user %d: %v", userID, channels)
        if len(channels) > 0 {
            var buttons []tgbotapi.InlineKeyboardButton
            for _, channel := range channels {
                buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Please join:*\n%s", strings.Join(channels, "\n")))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            if _, err := bot.Send(msg); err != nil {
                log.Printf("Error sending join message to user %d: %v", userID, err)
            }
        } else {
            log.Printf("No required channels, but joined is false for user %d - proceeding anyway", userID)
        }
        return
    }

    switch strings.TrimSpace(update.Message.Text) {
    case "💰 Balance":
        log.Printf("Normal button 'Balance' triggered for user %d", userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending balance message to user %d: %v", userID, err)
            msg := tgbotapi.NewMessage(userID, "❌ *Error displaying balance. Try again.*")
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
    case "💳 Set Wallet":
        log.Printf("Normal button 'Set Wallet' triggered for user %d", userID)
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Your wallet:* `%s`", user.Wallet))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
                ),
            )
            if _, err := bot.Send(msg); err != nil {
                log.Printf("Error sending wallet message to user %d: %v", userID, err)
            }
        } else {
            msg := tgbotapi.NewMessage(userID, "💳 *Enter your wallet address:*")
            msg.ParseMode = "MarkdownV2"
            if _, err := bot.Send(msg); err != nil {
                log.Printf("Error sending wallet prompt to user %d: %v", userID, err)
            }
            user.State = "setting_wallet"
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user state for %d: %v", userID, err)
            }
        }
    case "📞 Support":
        log.Printf("Normal button 'Support' triggered for user %d", userID)
        msg := tgbotapi.NewMessage(userID, "📞 *Send your message for support:*")
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending support prompt to user %d: %v", userID, err)
        }
        user.State = "support_message"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user state for %d: %v", userID, err)
        }
    case "🔗 Referral":
        log.Printf("Normal button 'Referral' triggered for user %d", userID)
        referralLink := generateReferralLink(userID)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🔗 *Referral Link:* `%s`\n🤝 *Referrals:* %d", referralLink, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📄 View Referrals", "view_referrals"),
                tgbotapi.NewInlineKeyboardButtonURL("📤 Share Link", fmt.Sprintf("https://t.me/share/url?url=%s", referralLink)),
            ),
        )
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending referral message to user %d: %v", userID, err)
        }
    case "📈 Stats":
        log.Printf("Normal button 'Stats' triggered for user %d", userID)
        totalUsers, err := getTotalUsers(db)
        if err != nil {
            log.Printf("Error getting total users for user %d: %v", userID, err)
            return
        }
        totalWithdrawals, err := getTotalWithdrawals(db)
        if err != nil {
            log.Printf("Error getting total withdrawals for user %d: %v", userID, err)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        if _, err := bot.Send(msg); err != nil {
            log.Printf("Error sending stats message to user %d: %v", userID, err)
        }
    case "💸 Withdraw":
        log.Printf("Normal button 'Withdraw' triggered for user %d", userID)
        if user.Wallet == "" {
            msg := tgbotapi.NewMessage(userID, "💳 *Set your wallet first!*")
            msg.ParseMode = "MarkdownV2"
            if _, err := bot.Send(msg); err != nil {
                log.Printf("Error sending wallet prompt to user %d: %v", userID, err)
            }
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
                msg.ParseMode = "MarkdownV2"
                if _, err := bot.Send(msg); err != nil {
                    log.Printf("Error sending min withdrawal message to user %d: %v", userID, err)
                }
            } else {
                msg := tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*")
                msg.ParseMode = "MarkdownV2"
                if _, err := bot.Send(msg); err != nil {
                    log.Printf("Error sending withdraw prompt to user %d: %v", userID, err)
                }
                user.State = "withdraw_amount"
                if err := updateUser(db, user); err != nil {
                    log.Printf("Error updating user state for %d: %v", userID, err)
                }
            }
        }
    default:
        log.Printf("Unhandled menu option for user %d: %s", userID, update.Message.Text)
    }
}
// Part 7 Ending



// Part 8 Starting
// Part 8 Starting
// Part 8 Starting
// Part 8 Starting
// Part 8 Starting
// Part 8 Starting
func handleCallbackQuery(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user %d: %v", userID, err)
        return
    }

    if user.Banned == 1 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *You are banned\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Request(tgbotapi.NewCallback(callback.ID, "🚫 Banned"))
        bot.Send(msg)
        return
    }

    if callback.Data == "set_inline" || callback.Data == "set_normal" {
        buttonStyle := "inline"
        if callback.Data == "set_normal" {
            buttonStyle = "normal"
        }
        user.ButtonStyle = buttonStyle
        updateUser(db, user)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Button style set to* %s\\!", buttonStyle))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        joined, err := checkUserJoinedChannels(bot, userID, db)
        if err != nil {
            log.Printf("Error checking channels for %d: %v", userID, err)
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
        } else {
            showMainMenu(bot, userID, buttonStyle)
        }
        bot.Request(tgbotapi.NewCallback(callback.ID, "✅ Style set"))
        return
    }

    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels for %d: %v", userID, err)
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

    switch callback.Data {
    case "balance":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "set_wallet":
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Your wallet:* `%s`", user.Wallet))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
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
        )
        bot.Send(msg)
    case "view_referrals":
        rows, err := db.Query("SELECT username FROM users WHERE referred_by = ?", userID)
        if err != nil {
            log.Printf("Error getting referrals for user %d: %v", userID, err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error fetching referrals\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
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
        msgText := formatMarkdownV2("📄 *Your referrals:* %d\n*Balance:* %.2f 💰\n*Referral List:*\n%s", user.Referrals, user.Balance, strings.Join(referrals, "\n"))
        if len(referrals) == 0 {
            msgText = formatMarkdownV2("📄 *Your referrals:* %d\n*Balance:* %.2f 💰\n*No referrals yet\\!*")
        }
        msg := tgbotapi.NewMessage(userID, msgText)
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "stats":
        totalUsers, err := getTotalUsers(db)
        if err != nil {
            log.Printf("Error getting total users: %v", err)
            return
        }
        totalWithdrawals, err := getTotalWithdrawals(db)
        if err != nil {
            log.Printf("Error getting total withdrawals: %v", err)
            return
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "withdraw":
        if user.Wallet == "" {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💳 *Set your wallet first\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Minimum withdrawal:* %.2f"))
                msg.ParseMode = "MarkdownV2"
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
// Part 8 Ending

// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9 Starting
// Part 9A Starting
// Part 9 Starting
// Part 9A Starting
// Part 9 Starting
// Part 9A Starting
func updateBroadcastProgress(bot *tgbotapi.BotAPI, userID int64, messageID int, sent, total int) {
    progress := int(float64(sent) / float64(total) * 10)
    bar := strings.Repeat("█", progress) + strings.Repeat("□", 10-progress)
    percentage := int(float64(sent) / float64(total) * 100)
    editMsg := tgbotapi.NewEditMessageText(userID, messageID, formatMarkdownV2("📢 *Broadcasting:* [%s] %d%%", bar, percentage))
    editMsg.ParseMode = "MarkdownV2"
    bot.Send(editMsg)
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

func getAllUsers(db *sql.DB) ([]User, error) {
    rows, err := db.Query("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users")
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    var users []User
    for rows.Next() {
        var user User
        err := rows.Scan(&user.UserID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State)
        if err != nil {
            log.Printf("Error scanning user: %v", err)
            continue
        }
        users = append(users, user)
    }
    return users, nil
}
// Part 9A Ending

// Part 9B Starting
func handleStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    state := user.State

    switch state {
    case "setting_wallet":
        wallet := strings.TrimSpace(update.Message.Text)
        if len(wallet) < 5 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Wallet address too short\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        user.Wallet = wallet
        user.State = ""
        updateUser(db, user)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Wallet set to:* `%s`", wallet))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        showMainMenu(bot, userID, user.ButtonStyle)
    case "support_message":
        supportMsg := update.Message.Text
        user.State = ""
        updateUser(db, user)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Message sent to support\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        adminMsg := tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("📩 *Support message from* @%s \\(ID: %d\\):\n%s", user.Username, userID, supportMsg))
        adminMsg.ParseMode = "MarkdownV2"
        adminMsg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Reply", fmt.Sprintf("contact_%d", userID)),
            ),
        )
        bot.Send(adminMsg)
        showMainMenu(bot, userID, user.ButtonStyle)
    case "withdraw_amount":
        amountStr := strings.TrimSpace(update.Message.Text)
        amount, err := strconv.ParseFloat(amountStr, 64)
        if err != nil || amount <= 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
        minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
        if amount < minWithdrawal {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        if user.Balance < amount {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Insufficient balance\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        user.Balance -= amount
        user.State = ""
        updateUser(db, user)
        paymentChannel, _ := getSetting(db, "payment_channel")
        if paymentChannel == "" {
            paymentChannel = "Admin"
        }
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💸 *Withdrawal request sent\\!*\n*Amount:* %.2f\n*Wallet:* `%s`", amount, user.Wallet))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        adminMsg := tgbotapi.NewMessage(ADMIN_ID, formatMarkdownV2("💸 *Withdrawal Request*\n*User:* @%s \\(ID: %d\\)\n*Amount:* %.2f\n*Wallet:* `%s`\n*Channel:* %s", user.Username, userID, amount, user.Wallet, paymentChannel))
        adminMsg.ParseMode = "MarkdownV2"
        bot.Send(adminMsg)
        showMainMenu(bot, userID, user.ButtonStyle)
    case "broadcast_message":
        if userID != ADMIN_ID {
            return
        }
        broadcastMsg := update.Message.Text
        users, err := getAllUsers(db)
        if err != nil {
            log.Printf("Error getting users for broadcast: %v", err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error fetching users\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        successCount := 0
        for _, u := range users {
            if u.Banned == 0 {
                msg := tgbotapi.NewMessage(u.UserID, broadcastMsg)
                msg.ParseMode = "MarkdownV2"
                if _, err := bot.Send(msg); err == nil {
                    successCount++
                }
                time.Sleep(50 * time.Millisecond)
            }
        }
        user.State = ""
        updateUser(db, user)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Broadcast sent to* %d *users\\!*", successCount))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "getting_user_info":
        if userID != ADMIN_ID {
            return
        }
        input := strings.TrimSpace(update.Message.Text)
        var targetUser User
        if strings.HasPrefix(input, "@") {
            targetUser, err = getUserByUsername(db, strings.TrimPrefix(input, "@"))
        } else {
            targetUserID, err := strconv.ParseInt(input, 10, 64)
            if err != nil {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID or username\\!*"))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
                return
            }
            targetUser, err = getUser(db, targetUserID)
        }
        if err != nil || targetUser.UserID == 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        status := "Active"
        if targetUser.Banned == 1 {
            status = "Banned"
        }
        msgText := formatMarkdownV2("📊 *User Info*\n*ID:* %d\n*Username:* @%s\n*Balance:* %.2f\n*Referrals:* %d\n*Wallet:* `%s`\n*Status:* %s", targetUser.UserID, targetUser.Username, targetUser.Balance, targetUser.Referrals, targetUser.Wallet, status)
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
        updateUser(db, user)
    case strings.HasPrefix(state, "adjusting_balance_"):
        if userID != ADMIN_ID {
            return
        }
        targetUserIDStr := strings.TrimPrefix(state, "adjusting_balance_")
        targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
        if err != nil {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        targetUser, err := getUser(db, targetUserID)
        if err != nil || targetUser.UserID == 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            user.State = ""
            updateUser(db, user)
            return
        }
        adjustment := strings.TrimSpace(update.Message.Text)
        if len(adjustment) == 0 || (adjustment[0] != '+' && adjustment[0] != '-') {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Enter a valid amount \\(e.g., +10 or -5\\)\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        amount, err := strconv.ParseFloat(adjustment[1:], 64)
        if err != nil || amount < 0 {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        if adjustment[0] == '+' {
            targetUser.Balance += amount
        } else {
            if targetUser.Balance < amount {
                msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Insufficient balance to deduct\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        targetUser.Balance -= amount
    }
    if err := updateUser(db, targetUser); err != nil {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to adjust balance\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Balance adjusted for user* %d\\!\n*New Balance:* %.2f", targetUserID, targetUser.Balance))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("💰 *Your balance has been updated\\!*\n*New Balance:* %.2f", targetUser.Balance))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case strings.HasPrefix(state, "contacting_"):
    if userID != ADMIN_ID {
        return
    }
    targetUserIDStr := strings.TrimPrefix(state, "contacting_")
    targetUserID, err := strconv.ParseInt(targetUserIDStr, 10, 64)
    if err != nil {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid user ID\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)
        return
    }
    targetUser, err := getUser(db, targetUserID)
    if err != nil || targetUser.UserID == 0 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        user.State = ""
        updateUser(db, user)
        return
    }
    message := update.Message.Text
    msg := tgbotapi.NewMessage(targetUserID, formatMarkdownV2("📩 *Message from Admin:*\n%s", message))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    msg = tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Message sent to user* %d\\!", targetUserID))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "setting_min_withdrawal":
    if userID != ADMIN_ID {
        return
    }
    amountStr := strings.TrimSpace(update.Message.Text)
    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil || amount < 0 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    if err := updateSetting(db, "min_withdrawal", fmt.Sprintf("%.2f", amount)); err != nil {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set minimum withdrawal\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Minimum withdrawal set to:* %.2f", amount))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "setting_payment_channel":
    if userID != ADMIN_ID {
        return
    }
    channel := update.Message.Text
    if !strings.HasPrefix(channel, "@") {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Use '@' \\(e.g., @ChannelName\\)\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    params := tgbotapi.Params{"chat_id": channel}
    resp, err := bot.MakeRequest("getChat", params)
    if err != nil {
        log.Printf("Error verifying payment channel %s: %v", channel, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid channel or bot lacks access\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    if err := updateSetting(db, "payment_channel", channel); err != nil {
        log.Printf("Error updating payment channel to %s: %v", channel, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set payment channel\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Payment Channel set to:* %s", channel))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "setting_referral_reward":
    if userID != ADMIN_ID {
        return
    }
    amountStr := strings.TrimSpace(update.Message.Text)
    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil || amount < 0 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid amount\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    if err := updateSetting(db, "referral_reward", fmt.Sprintf("%.2f", amount)); err != nil {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set referral reward\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Referral reward set to:* %.2f", amount))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "add_channel":
    if userID != ADMIN_ID {
        return
    }
    channel := update.Message.Text
    if !strings.HasPrefix(channel, "@") {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Use '@' \\(e.g., @ChannelName\\)\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    params := tgbotapi.Params{"chat_id": channel}
    resp, err := bot.MakeRequest("getChat", params)
    if err != nil {
        log.Printf("Error verifying channel %s: %v", channel, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Invalid channel or bot lacks access\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    if err := addRequiredChannel(db, channel); err != nil {
        log.Printf("Error adding channel %s: %v", channel, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to add channel\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➕ *Channel* %s *added\\!*"))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "remove_channel":
    if userID != ADMIN_ID {
        return
    }
    channel := update.Message.Text
    if !strings.HasPrefix(channel, "@") {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Use '@' \\(e.g., @ChannelName\\)\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    if err := removeRequiredChannel(db, channel); err != nil {
        log.Printf("Error removing channel %s: %v", channel, err)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to remove channel\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➖ *Channel* %s *removed\\!*"))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
case "setting_start_message":
    if userID != ADMIN_ID {
        return
    }
    startMessage := update.Message.Text
    if err := updateSetting(db, "start_message", startMessage); err != nil {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Failed to set start message\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *Start message updated\\!*"))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    user.State = ""
    updateUser(db, user)
}
}
// Part 9B Ending
// Part 10 Starting
// Part 10 Starting
// Part 10 Starting
func handleAdminActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    action := callback.Data
    switch action {
    case "admin_broadcast":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📢 *Send message to broadcast:*"))
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
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📡 *Enter payment channel \\(e.g., @Channel\\):*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_payment_channel"})
    case "admin_set_referral_reward":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🎁 *Enter referral reward amount:*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "setting_referral_reward"})
    case "admin_add_channel":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📌 *Enter channel username \\(e.g., @Channel\\):*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: "add_channel"})
    case "admin_remove_channel":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("➖ *Enter channel username \\(e.g., @Channel\\):*"))
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

func handleQRSettings(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    qrAction := "Disabled"
    if callback.Data == "qr_enable" {
        updateSetting(db, "qr_enabled", "1")
        qrAction = "Enabled"
    } else {
        updateSetting(db, "qr_enabled", "0")
    }
    msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🔳 *QR* %s", qrAction))
    msg.ParseMode = "MarkdownV2"
    bot.Send(msg)
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func handleAdminUserActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("🚫 *Unauthorized\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    parts := strings.Split(callback.Data, "_")
    action, targetUserIDStr := parts[0], parts[1]
    targetUserID, _ := strconv.ParseInt(targetUserIDStr, 10, 64)
    targetUser, err := getUser(db, targetUserID)
    if err != nil || targetUser.UserID == 0 {
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *User not found\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    switch action {
    case "adjust":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("💰 *Enter amount to adjust for user* %d *\\(e.g., +10 or -5\\):*", targetUserID))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: fmt.Sprintf("adjusting_balance_%d", targetUserID)})
    case "ban":
        targetUser.Banned = 1
        updateUser(db, targetUser)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *User* %d *banned\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("🚫 *You have been banned\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "unban":
        targetUser.Banned = 0
        updateUser(db, targetUser)
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("✅ *User* %d *unbanned\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        msg = tgbotapi.NewMessage(targetUserID, formatMarkdownV2("✅ *You have been unbanned\\!*"))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "viewrefs":
        rows, err := db.Query("SELECT username FROM users WHERE referred_by = ?", targetUserID)
        if err != nil {
            log.Printf("Error getting referrals: %v", err)
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("❌ *Error fetching referrals\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        defer rows.Close()
        var referrals []string
        for rows.Next() {
            var username string
            rows.Scan(&username)
            referrals = append(referrals, "@"+username)
        }
        if len(referrals) > 0 {
            var buf bytes.Buffer
            buf.WriteString(strings.Join(referrals, "\n"))
            doc := tgbotapi.NewDocument(userID, tgbotapi.FileBytes{
                Name:  fmt.Sprintf("referrals_%d.txt", targetUserID),
                Bytes: buf.Bytes(),
            })
            doc.Caption = formatMarkdownV2("📄 *Referrals for user* %d", targetUserID)
            doc.ParseMode = "MarkdownV2"
            bot.Send(doc)
        } else {
            msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📄 *No referrals yet\\!*"))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        }
    case "contact":
        msg := tgbotapi.NewMessage(userID, formatMarkdownV2("📩 *Enter message for user* %d:", targetUserID))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        updateUser(db, User{UserID: userID, State: fmt.Sprintf("contacting_%d", targetUserID)})
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}
// Part 10 Ending