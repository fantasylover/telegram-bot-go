// Part 1 Starting
package main

import (
    "bytes"
    "database/sql"
    "encoding/json"
    "fmt"
    "log"
    "math/rand"
    "os"
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
    charsToEscape := []string{"_", "*", "`", "[", "]", "(", ")", ".", "!", "-", "+"}
    for _, char := range charsToEscape {
        text = strings.ReplaceAll(text, char, "\\"+char)
    }
    return text
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
    msg := tgbotapi.NewMessage(userID, "✨ *Main Menu* ✨")
    msg.ParseMode = "MarkdownV2"
    msg.ReplyMarkup = markup
    bot.Send(msg)
}
// Part 3 Ending


// Part 4 Starting
func handleStart(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    userID := update.Message.From.ID
    username := update.Message.From.Username
    if username == "" {
        username = update.Message.From.FirstName
        if username == "" {
            username = fmt.Sprintf("User_%d", userID)
        }
    }

    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
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
            log.Printf("Error creating user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("🔔 *New user:* @%s", escapeMarkdownV2(username))))
        if referredBy != 0 {
            referrer, err := getUser(db, referredBy)
            if err == nil && referrer.UserID != 0 {
                rewardStr, _ := getSetting(db, "referral_reward")
                reward, _ := strconv.ParseFloat(rewardStr, 64)
                referrer.Balance += reward
                referrer.Referrals++
                if err := updateUser(db, referrer); err != nil {
                    log.Printf("Error updating referrer: %v", err)
                }
                bot.Send(tgbotapi.NewMessage(referredBy, fmt.Sprintf("🎉 *Your friend* @%s *joined\\!*\n*You earned* %.2f 💰", escapeMarkdownV2(username), reward)))
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
        if !checkUserJoinedChannels(bot, userID, db) {
            channels, _ := getRequiredChannels(db)
            if len(channels) > 0 {
                var buttons []tgbotapi.InlineKeyboardButton
                for _, channel := range channels {
                    buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
                }
                markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
                msg := tgbotapi.NewMessage(userID, escapeMarkdownV2("📢 *Please join:*\n"+strings.Join(channels, "\n")))
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
        if update.Message != nil {
            user, err := getUser(db, update.Message.From.ID)
            if err != nil {
                log.Printf("Error getting user: %v", err)
                continue
            }
            if user.Banned == 1 {
                bot.Send(tgbotapi.NewMessage(update.Message.From.ID, "🚫 *You are banned\\!*"))
                continue
            }
            if update.Message.IsCommand() {
                switch update.Message.Command() {
                case "start":
                    handleStart(bot, db, update)
                case "admin":
                    if update.Message.From.ID == ADMIN_ID {
                        showAdminPanel(bot, db, update.Message.From.ID)
                    }
                }
            }
        }
    }
}

func showAdminPanel(bot *tgbotapi.BotAPI, db *sql.DB, userID int64) {
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
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
            bot.Send(tgbotapi.NewMessage(userID, "🚫 *You are banned\\!*"))
            return
        }

        if user.UserID == 0 { // New user registration
            username := update.Message.From.Username
            if username == "" {
                username = update.Message.From.FirstName
                if username == "" {
                    username = fmt.Sprintf("User_%d", userID)
                }
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
    joined, err := checkUserJoinedChannels(bot, userID, db)
    if err != nil {
        log.Printf("Error checking channels: %v", err)
        return
    }
    if !joined {
        channels, err := getRequiredChannels(db)
        if err != nil {
            log.Printf("Error getting required channels: %v", err)
            return
        }
        if len(channels) > 0 {
            var buttons []tgbotapi.InlineKeyboardButton
            for _, channel := range channels {
                buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
            msg := tgbotapi.NewMessage(userID, escapeMarkdownV2("📢 *Please join:*\n"+strings.Join(channels, "\n")))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        }
        return
    }

    switch update.Message.Text {
    case "💰 Balance":
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "💳 Set Wallet":
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Your wallet:* `%s`", escapeMarkdownV2(user.Wallet)))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
                ),
            )
            bot.Send(msg)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Enter your wallet address:*"))
            user.State = "setting_wallet"
            updateUser(db, user)
        }
    case "📞 Support":
        bot.Send(tgbotapi.NewMessage(userID, "📞 *Send your message for support:*"))
        user.State = "support_message"
        updateUser(db, user)
    case "🔗 Referral":
        referralLink := generateReferralLink(userID)
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("🔗 *Referral Link:* `%s`\n🤝 *Referrals:* %d", escapeMarkdownV2(referralLink), user.Referrals))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("📄 View Referrals", "view_referrals"),
                tgbotapi.NewInlineKeyboardButtonURL("📤 Share Link", fmt.Sprintf("https://t.me/share/url?url=%s", referralLink)),
            ),
        )
        bot.Send(msg)
    case "📈 Stats":
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
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "💸 Withdraw":
        if user.Wallet == "" {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Set your wallet first\\.*"))
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
            } else {
                bot.Send(tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*"))
                user.State = "withdraw_amount"
                updateUser(db, user)
            }
        }
    }
}
// Part 7 Ending

// Part 8 Starting
func handleCallbackQuery(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
        return
    }

    if user.Banned == 1 {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *You are banned\\!*"))
        bot.Request(tgbotapi.NewCallback(callback.ID, "🚫 Banned"))
        return
    }

    if callback.Data == "set_inline" || callback.Data == "set_normal" {
        buttonStyle := "inline"
        if callback.Data == "set_normal" {
            buttonStyle = "normal"
        }
        user.ButtonStyle = buttonStyle
        updateUser(db, user)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *Button style set to* %s\\.", escapeMarkdownV2(buttonStyle))))
        if !checkUserJoinedChannels(bot, userID, db) {
            channels, _ := getRequiredChannels(db)
            if len(channels) > 0 {
                var buttons []tgbotapi.InlineKeyboardButton
                for _, channel := range channels {
                    buttons = append(buttons, tgbotapi.NewInlineKeyboardButtonURL("Join Channel", "https://t.me/"+strings.TrimPrefix(channel, "@")))
                }
                markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(buttons...))
                msg := tgbotapi.NewMessage(userID, escapeMarkdownV2("📢 *Please join:*\n"+strings.Join(channels, "\n")))
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
        log.Printf("Error checking channels: %v", err)
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
            msg := tgbotapi.NewMessage(userID, escapeMarkdownV2("📢 *Please join:*\n"+strings.Join(channels, "\n")))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        }
        bot.Request(tgbotapi.NewCallback(callback.ID, "📢 Join channels"))
        return
    }

    switch callback.Data {
    case "balance":
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "set_wallet":
        if user.Wallet != "" {
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Your wallet:* `%s`", escapeMarkdownV2(user.Wallet)))
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("Change Wallet", "change_wallet"),
                ),
            )
            bot.Send(msg)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Enter your wallet address:*"))
            user.State = "setting_wallet"
            updateUser(db, user)
        }
    case "change_wallet":
        bot.Send(tgbotapi.NewMessage(userID, "💳 *Enter new wallet address:*"))
        user.State = "setting_wallet"
        updateUser(db, user)
    case "support":
        bot.Send(tgbotapi.NewMessage(userID, "📞 *Send your message for support:*"))
        user.State = "support_message"
        updateUser(db, user)
    case "referral":
        referralLink := generateReferralLink(userID)
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("🔗 *Referral Link:* `%s`\n🤝 *Referrals:* %d", escapeMarkdownV2(referralLink), user.Referrals))
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
            log.Printf("Error getting referrals: %v", err)
            return
        }
        defer rows.Close()
        var referrals []string
        for rows.Next() {
            var username string
            rows.Scan(&username)
            referrals = append(referrals, username)
        }
        if len(referrals) > 0 {
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📄 *Your referrals:*\n%s", escapeMarkdownV2(strings.Join(referrals, "\n"))))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "📄 *No referrals yet\\.*"))
        }
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
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
    case "withdraw":
        if user.Wallet == "" {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Set your wallet first\\.*"))
        } else {
            minWithdrawalStr, _ := getSetting(db, "min_withdrawal")
            minWithdrawal, _ := strconv.ParseFloat(minWithdrawalStr, 64)
            if user.Balance < minWithdrawal {
                msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💸 *Minimum withdrawal:* %.2f", minWithdrawal))
                msg.ParseMode = "MarkdownV2"
                bot.Send(msg)
            } else {
                bot.Send(tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*"))
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
func handleStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    state := user.State

    switch state {
    case "setting_wallet":
        wallet := strings.TrimSpace(update.Message.Text)
        user.Wallet = wallet
        user.State = ""
        updateUser(db, user)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Wallet set to:* `%s`", escapeMarkdownV2(wallet))))
    case "withdraw_amount":
        amount, err := strconv.ParseFloat(update.Message.Text, 64)
        if err != nil || amount <= 0 || amount > user.Balance {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Enter a valid amount\\.*"))
            return
        }
        user.Balance -= amount
        user.State = ""
        updateUser(db, user)
        createWithdrawal(db, userID, amount, user.Wallet)
        bot.Send(tgbotapi.NewMessage(userID, "✅ *Withdrawal request sent\\! Admin will review soon\\.*"))
        paymentChannel, _ := getSetting(db, "payment_channel")
        if paymentChannel != "" {
            params := tgbotapi.Params{"chat_id": paymentChannel}
            resp, err := bot.MakeRequest("getChat", params)
            if err != nil {
                log.Printf("Error fetching payment channel: %v", err)
                return
            }
            var chat tgbotapi.Chat
            json.Unmarshal(resp.Result, &chat)
            paymentChannelID := chat.ID

            randomSuffix := rand.Intn(10000000)
            txID := fmt.Sprintf("2025%07d", randomSuffix)
            channels, _ := getRequiredChannels(db)
            channelURL := "@DefaultChannel"
            if len(channels) > 0 {
                channelURL = channels[0]
            }
            markup := tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonURL("🔍CHANN", fmt.Sprintf("https://t.me/%s", strings.TrimPrefix(channelURL, "@"))),
                    tgbotapi.NewInlineKeyboardButtonURL("JOIN", fmt.Sprintf("https://t.me/%s", BOT_USERNAME)),
                ),
            )
            firstName := update.Message.From.FirstName
            if firstName == "" {
                firstName = user.Username
            }
            msgText := fmt.Sprintf(
                "🔥 *NEW WITHDRAWAL SENT* 🔥\n\n👤 *USER:* [%s](tg://user?id=%d)\n💎 *USER ID:* `%d`\n💰 *AMOUNT:* %.2f FREE COIN\n📞 *REFERRER:* %d\n🔗 *ADDRESS:* `%s`\n⏰ *TRANSACTION ID:* `%s`",
                escapeMarkdownV2(firstName), userID, userID, amount, user.Referrals, escapeMarkdownV2(user.Wallet), txID,
            )
            qrEnabled, _ := getSetting(db, "qr_enabled")
            if qrEnabled == "1" {
                qr, err := qrcode.New(user.Wallet, qrcode.Medium)
                if err != nil {
                    log.Printf("Error generating QR: %v", err)
                    bot.Send(tgbotapi.NewMessage(paymentChannelID, fmt.Sprintf("%s\n⚠️ *QR code generation failed\\.*", msgText)))
                    return
                }
                var buf bytes.Buffer
                qr.Write(256, &buf)
                photo := tgbotapi.NewPhoto(paymentChannelID, tgbotapi.FileBytes{
                    Name:  "qr.png",
                    Bytes: buf.Bytes(),
                })
                photo.Caption = msgText
                photo.ParseMode = "MarkdownV2"
                photo.ReplyMarkup = markup
                bot.Send(photo)
            } else {
                msg := tgbotapi.NewMessage(paymentChannelID, msgText)
                msg.ParseMode = "MarkdownV2"
                msg.ReplyMarkup = markup
                bot.Send(msg)
            }
        }
    case "support_message":
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Ban User", fmt.Sprintf("ban_%d", userID)),
            ),
        )
        bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("📞 *Support from* @%s\n%s", escapeMarkdownV2(user.Username), escapeMarkdownV2(update.Message.Text)), markup))
        bot.Send(tgbotapi.NewMessage(userID, "✅ *Your message has been sent to support\\!*"))
        user.State = ""
        updateUser(db, user)
    case "broadcast_message":
        if userID != ADMIN_ID {
            return
        }
        rows, err := db.Query("SELECT user_id FROM users WHERE banned = 0")
        if err != nil {
            log.Printf("Error getting users: %v", err)
            return
        }
        defer rows.Close()
        var users []int64
        for rows.Next() {
            var uid int64
            rows.Scan(&uid)
            users = append(users, uid)
        }
        totalUsers := len(users)
        sentCount := 0
        statusMsg, _ := bot.Send(tgbotapi.NewMessage(userID, "📢 *Broadcasting:* [□□□□□□□□□□] 0%"))
        for _, uid := range users {
            bot.Send(tgbotapi.NewMessage(uid, escapeMarkdownV2(update.Message.Text)))
            sentCount++
            progress := int(float64(sentCount) / float64(totalUsers) * 10)
            bar := strings.Repeat("█", progress) + strings.Repeat("□", 10-progress)
            percentage := int(float64(sentCount) / float64(totalUsers) * 100)
            bot.Send(tgbotapi.NewEditMessageText(userID, statusMsg.MessageID, fmt.Sprintf("📢 *Broadcasting:* [%s] %d%% (%d/%d)", bar, percentage, sentCount, totalUsers)))
            time.Sleep(100 * time.Millisecond)
        }
        bot.Send(tgbotapi.NewEditMessageText(userID, statusMsg.MessageID, fmt.Sprintf("✅ *Broadcast completed\\!* Sent to %d/%d users\\.", sentCount, totalUsers)))
        user.State = ""
        updateUser(db, user)
    case "getting_user_info":
        if userID != ADMIN_ID {
            return
        }
        target := update.Message.Text
        var targetUser User
        if targetID, err := strconv.ParseInt(target, 10, 64); err == nil {
            targetUser, err = getUser(db, targetID)
        } else {
            rows, err := db.Query("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE username = ?", strings.TrimPrefix(target, "@"))
            if err == nil && rows.Next() {
                rows.Scan(&targetUser.UserID, &targetUser.Username, &targetUser.Balance, &targetUser.Wallet, &targetUser.Referrals, &targetUser.ReferredBy, &targetUser.Banned, &targetUser.ButtonStyle, &targetUser.State)
                rows.Close()
            }
        }
        if targetUser.UserID != 0 {
            info := fmt.Sprintf(
                "👤 *User Info*\n*ID:* %d\n*Username:* @%s\n*Balance:* %.2f 💰\n*Wallet:* `%s`\n*Referrals:* %d\n*Banned:* %s",
                targetUser.UserID, escapeMarkdownV2(targetUser.Username), targetUser.Balance, escapeMarkdownV2(targetUser.Wallet), targetUser.Referrals, map[int]string{0: "No", 1: "Yes"}[targetUser.Banned],
            )
            markup := tgbotapi.NewInlineKeyboardMarkup(
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("💰 Adjust Balance", fmt.Sprintf("adjust_%d", targetUser.UserID)),
                    tgbotapi.NewInlineKeyboardButtonData(map[int]string{0: "Ban User", 1: "Unban User"}[targetUser.Banned], fmt.Sprintf("%s_%d", map[int]string{0: "ban", 1: "unban"}[targetUser.Banned], targetUser.UserID)),
                ),
                tgbotapi.NewInlineKeyboardRow(
                    tgbotapi.NewInlineKeyboardButtonData("View Referrals", fmt.Sprintf("viewrefs_%d", targetUser.UserID)),
                    tgbotapi.NewInlineKeyboardButtonData("Contact User", fmt.Sprintf("contact_%d", targetUser.UserID)),
                ),
            )
            msg := tgbotapi.NewMessage(userID, info)
            msg.ParseMode = "MarkdownV2"
            msg.ReplyMarkup = markup
            bot.Send(msg)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *User not found\\.*"))
        }
        user.State = ""
        updateUser(db, user)
    case "setting_min_withdrawal", "setting_referral_reward", "setting_start_message", "setting_payment_channel":
        if userID != ADMIN_ID {
            return
        }
        key := strings.TrimPrefix(state, "setting_")
        value := update.Message.Text
        updateSetting(db, key, value)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *%s set to:* %s", strings.Title(strings.ReplaceAll(key, "_", " ")), escapeMarkdownV2(value))))
        user.State = ""
        updateUser(db, user)
    case "add_channel":
        if userID != ADMIN_ID {
            return
        }
        channel := update.Message.Text
        if !strings.HasPrefix(channel, "@") {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Use '@' \\(e\\.g\\., @ChannelName\\)\\.*"))
            return
        }
        params := tgbotapi.Params{"chat_id": channel}
        resp, err := bot.MakeRequest("getChat", params)
        if err != nil {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Invalid channel or bot lacks access\\.*"))
            return
        }
        json.Unmarshal(resp.Result, &tgbotapi.Chat{})
        addRequiredChannel(db, channel)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("➕ *Channel* %s *added\\!*", escapeMarkdownV2(channel))))
        user.State = ""
        updateUser(db, user)
    case "remove_channel":
        if userID != ADMIN_ID {
            return
        }
        channel := update.Message.Text
        if !strings.HasPrefix(channel, "@") {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Use '@' \\(e\\.g\\., @ChannelName\\)\\.*"))
            return
        }
        removeRequiredChannel(db, channel)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("➖ *Channel* %s *removed\\!*", escapeMarkdownV2(channel))))
        user.State = ""
        updateUser(db, user)
    }
}
// Part 9 Ending

// Part 10 Starting
func handleAdminActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    action := callback.Data
    switch action {
    case "admin_broadcast":
        bot.Send(tgbotapi.NewMessage(userID, "📢 *Send message to broadcast:*"))
        updateUser(db, User{UserID: userID, State: "broadcast_message"})
    case "admin_user_info":
        bot.Send(tgbotapi.NewMessage(userID, "📊 *Enter user ID or username:*"))
        updateUser(db, User{UserID: userID, State: "getting_user_info"})
    case "admin_set_min_withdrawal":
        bot.Send(tgbotapi.NewMessage(userID, "💰 *Enter new minimum withdrawal:*"))
        updateUser(db, User{UserID: userID, State: "setting_min_withdrawal"})
    case "admin_set_payment_channel":
        bot.Send(tgbotapi.NewMessage(userID, "📡 *Enter payment channel \\(e\\.g\\., @Channel\\):*"))
        updateUser(db, User{UserID: userID, State: "setting_payment_channel"})
    case "admin_set_referral_reward":
        bot.Send(tgbotapi.NewMessage(userID, "🎁 *Enter referral reward amount:*"))
        updateUser(db, User{UserID: userID, State: "setting_referral_reward"})
    case "admin_add_channel":
        bot.Send(tgbotapi.NewMessage(userID, "📌 *Enter channel username \\(e\\.g\\., @Channel\\):*"))
        updateUser(db, User{UserID: userID, State: "add_channel"})
    case "admin_remove_channel":
        bot.Send(tgbotapi.NewMessage(userID, "➖ *Enter channel username \\(e\\.g\\., @Channel\\):*"))
        updateUser(db, User{UserID: userID, State: "remove_channel"})
    case "admin_start_settings":
        bot.Send(tgbotapi.NewMessage(userID, "🚀 *Enter new start message:*"))
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
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("🔳 *QR Status:* %s", qrStatus), markup))
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func handleQRSettings(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
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
    bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("🔳 *QR* %s", qrAction)))
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}

func handleAdminUserActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    parts := strings.Split(callback.Data, "_")
    action, targetUserIDStr := parts[0], parts[1]
    targetUserID, _ := strconv.ParseInt(targetUserIDStr, 10, 64)
    targetUser, err := getUser(db, targetUserID)
    if err != nil || targetUser.UserID == 0 {
        bot.Send(tgbotapi.NewMessage(userID, "❌ *User not found\\.*"))
        bot.Request(tgbotapi.NewCallback(callback.ID, ""))
        return
    }
    switch action {
    case "adjust":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Enter amount to adjust for user* %d *\\(e\\.g\\., +10 or -5\\):*", targetUserID)))
        updateUser(db, User{UserID: userID, State: fmt.Sprintf("adjusting_balance_%d", targetUserID)})
    case "ban":
        targetUser.Banned = 1
        updateUser(db, targetUser)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *banned\\.*", targetUserID)))
        bot.Send(tgbotapi.NewMessage(targetUserID, "🚫 *You have been banned\\!*"))
    case "unban":
        targetUser.Banned = 0
        updateUser(db, targetUser)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *unbanned\\.*", targetUserID)))
        bot.Send(tgbotapi.NewMessage(targetUserID, "✅ *You have been unbanned\\!*"))
    case "viewrefs":
        rows, err := db.Query("SELECT username FROM users WHERE referred_by = ?", targetUserID)
        if err != nil {
            log.Printf("Error getting referrals: %v", err)
            return
        }
        defer rows.Close()
        var referrals []string
        for rows.Next() {
            var username string
            rows.Scan(&username)
            referrals = append(referrals, username)
        }
        if len(referrals) > 0 {
            var buf bytes.Buffer
            buf.WriteString(strings.Join(referrals, "\n"))
            doc := tgbotapi.NewDocument(userID, tgbotapi.FileBytes{
                Name:  fmt.Sprintf("referrals_%d.txt", targetUserID),
                Bytes: buf.Bytes(),
            })
            doc.Caption = fmt.Sprintf("📄 *Referrals for user* %d", targetUserID)
            doc.ParseMode = "MarkdownV2"
            bot.Send(doc)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "📄 *No referrals yet\\.*"))
        }
    case "contact":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📩 *Enter message for user* %d:", targetUserID)))
        updateUser(db, User{UserID: userID, State: fmt.Sprintf("contacting_%d", targetUserID)})
    }
    bot.Request(tgbotapi.NewCallback(callback.ID, ""))
}
// Part 10 Ending


