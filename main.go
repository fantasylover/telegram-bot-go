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
    "os"
)

const (
    BOT_TOKEN = "1743577119:AAEiYy_kgUK41RcBxF18NgkR4VehXtZWm_w"
    ADMIN_ID  = 1192041312
)

var BOT_USERNAME string

func initDB() (*sql.DB, error) {
    db, err := sql.Open("sqlite3", "bot.db")
    if err != nil {
        return nil, err
    }

    // Users table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            user_id INTEGER PRIMARY KEY,
            username TEXT,
            balance REAL DEFAULT 0,
            wallet TEXT,
            referrals INTEGER DEFAULT 0,
            referred_by INTEGER,
            banned INTEGER DEFAULT 0,
            button_style TEXT,
            state TEXT DEFAULT ''
        )
    `)
    if err != nil {
        return nil, err
    }

    // Settings table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS settings (
            key TEXT PRIMARY KEY,
            value TEXT
        )
    `)
    if err != nil {
        return nil, err
    }

    // Required channels table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS required_channels (
            channel_id TEXT PRIMARY KEY
        )
    `)
    if err != nil {
        return nil, err
    }

    // Withdrawals table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS withdrawals (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER,
            amount REAL,
            wallet TEXT,
            status TEXT DEFAULT 'pending',
            timestamp INTEGER DEFAULT 0
        )
    `)
    if err != nil {
        return nil, err
    }

    // Insert default settings
    defaultSettings := [][]string{
        {"min_withdrawal", "10"},
        {"payment_channel", "@YourPaymentChannel"},
        {"referral_reward", "5"},
        {"start_message", "🎉 Welcome to the Referral & Earning Bot! Join channels to start."},
        {"qr_enabled", "0"},
    }
    for _, setting := range defaultSettings {
        _, err = db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES (?, ?)", setting[0], setting[1])
        if err != nil {
            return nil, err
        }
    }

    return db, nil
}



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

func getUser(db *sql.DB, userID int64) (User, error) {
    var user User
    err := db.QueryRow("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE user_id = ?", userID).Scan(
        &user.UserID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State,
    )
    if err == sql.ErrNoRows {
        return user, nil
    }
    return user, err
}

func updateUser(db *sql.DB, user User) error {
    _, err := db.Exec(
        "INSERT OR REPLACE INTO users (user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
        user.UserID, user.Username, user.Balance, user.Wallet, user.Referrals, user.ReferredBy, user.Banned, user.ButtonStyle, user.State,
    )
    return err
}

func getSetting(db *sql.DB, key string) (string, error) {
    var value string
    err := db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
    if err == sql.ErrNoRows {
        return "", fmt.Errorf("setting %s not found", key)
    }
    return value, err
}

func updateSetting(db *sql.DB, key, value string) error {
    _, err := db.Exec("INSERT OR REPLACE INTO settings (key, value) VALUES (?, ?)", key, value)
    return err
}

func getRequiredChannels(db *sql.DB) ([]string, error) {
    rows, err := db.Query("SELECT channel_id FROM required_channels")
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

func addRequiredChannel(db *sql.DB, channelID string) error {
    if !strings.HasPrefix(channelID, "@") {
        channelID = "@" + channelID
    }
    _, err := db.Exec("INSERT OR IGNORE INTO required_channels (channel_id) VALUES (?)", channelID)
    return err
}

func removeRequiredChannel(db *sql.DB, channelID string) error {
    if !strings.HasPrefix(channelID, "@") {
        channelID = "@" + channelID
    }
    _, err := db.Exec("DELETE FROM required_channels WHERE channel_id = ?", channelID)
    return err
}

func createWithdrawal(db *sql.DB, userID int64, amount float64, wallet string) error {
    _, err := db.Exec(
        "INSERT INTO withdrawals (user_id, amount, wallet, timestamp) VALUES (?, ?, ?, ?)",
        userID, amount, wallet, int64(time.Now().Unix()),
    )
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

func getReferrals(db *sql.DB, userID int64) ([]User, error) {
    rows, err := db.Query("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE referred_by = ?", userID)
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var referrals []User
    for rows.Next() {
        var user User
        if err := rows.Scan(&user.UserID, &user.Username, &user.Balance, &user.Wallet, &user.Referrals, &user.ReferredBy, &user.Banned, &user.ButtonStyle, &user.State); err != nil {
            return nil, err
        }
        referrals = append(referrals, user)
    }
    return referrals, nil
}



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
        chatConfig := tgbotapi.GetChatMemberConfig{
            ChatConfigWithUser: tgbotapi.ChatConfigWithUser{
                ChatID: channel,
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

func main() {
    db, err := initDB()
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    bot, err := tgbotapi.NewBotAPI(os.Getenv("BOT_TOKEN"))
    if err != nil {
        log.Fatal(err)
    }

    BOT_USERNAME = bot.Self.UserName
    bot.Debug = true
    log.Printf("Authorized on account @%s", BOT_USERNAME)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        go handleUpdate(bot, db, update)
    }
}



func handleUpdate(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    if update.Message != nil {
        userID := update.Message.From.ID
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        if user.UserID == 0 {
            username := update.Message.From.UserName
            if username == "" {
                username = fmt.Sprintf("User_%d", userID)
            }
            user = User{
                UserID:   userID,
                Username: username,
            }
            if update.Message.Text == "/start" {
                user.State = ""
            } else if strings.HasPrefix(update.Message.Text, "/start ") {
                refIDStr := strings.TrimPrefix(update.Message.Text, "/start ")
                refID, _ := strconv.ParseInt(refIDStr, 10, 64)
                user.ReferredBy = sql.NullInt64{Int64: refID, Valid: refID != 0 && refID != userID}
            }
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user: %v", err)
                return
            }
        }

        if user.Banned == 1 {
            bot.Send(tgbotapi.NewMessage(userID, "🚫 *You are banned or not registered\\!*"))
            return
        }

        if update.Message.IsCommand() {
            switch update.Message.Command() {
            case "start":
                handleStart(bot, db, update, user)
            case "admin":
                showAdminPanel(bot, db, userID)
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

func handleStart(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    username := update.Message.From.UserName
    if username == "" {
        username = fmt.Sprintf("User_%d", userID)
    }

    if user.ReferredBy.Valid {
        referrer, err := getUser(db, user.ReferredBy.Int64)
        if err != nil {
            log.Printf("Error getting referrer: %v", err)
        } else if referrer.UserID != 0 {
            reward, err := getSetting(db, "referral_reward")
            if err != nil {
                log.Printf("Error getting referral_reward: %v", err)
            } else {
                rewardFloat, err := strconv.ParseFloat(reward, 64)
                if err != nil {
                    log.Printf("Error parsing referral_reward: %v", err)
                } else {
                    referrer.Balance += rewardFloat
                    referrer.Referrals += 1
                    if err := updateUser(db, referrer); err != nil {
                        log.Printf("Error updating referrer: %v", err)
                    } else {
                        escapedUsername := escapeMarkdownV2(username)
                        bot.Send(tgbotapi.NewMessage(referrer.UserID, fmt.Sprintf("🎉 *Your friend* @%s *joined\\!*\n*You earned* %.2f 💰", escapedUsername, rewardFloat)))
                    }
                }
            }
        }
    }

    escapedUsername := escapeMarkdownV2(username)
    bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("🔔 *New user:* @%s", escapedUsername)))

    user.Username = username
    if err := updateUser(db, user); err != nil {
        log.Printf("Error updating user: %v", err)
        return
    }

    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
        return
    }

    if user.ButtonStyle == "" {
        msg := tgbotapi.NewMessage(userID, getSetting(db, "start_message"))
        msg.ParseMode = "MarkdownV2"
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Inline Buttons", "set_inline"),
                tgbotapi.NewInlineKeyboardButtonData("Normal Buttons", "set_normal"),
            ),
        )
        bot.Send(msg)
    } else {
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
            escapedChannels := strings.Join(channels, "\n")
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📢 *Please join:*\n%s", escapeMarkdownV2(escapedChannels)))
            msg.ParseMode = "MarkdownV2"
            bot.Send(msg)
            return
        }
        showMainMenu(bot, userID, user.ButtonStyle)
    }
}

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
        escapedChannels := strings.Join(channels, "\n")
        msg := tgbotapi.NewMessage(userID, fmt.Sprintf("📢 *Please join:*\n%s", escapeMarkdownV2(escapedChannels)))
        msg.ParseMode = "MarkdownV2"
        bot.Send(msg)
        return
    }

    switch update.Message.Text {
    case "💰 Balance":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals)))
    case "💳 Set Wallet":
        if user.Wallet != "" {
            escapedWallet := escapeMarkdownV2(user.Wallet)
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Your wallet:* `%s`", escapedWallet))
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
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user: %v", err)
            }
        }
    case "📞 Support":
        bot.Send(tgbotapi.NewMessage(userID, "📞 *Send your message for support:*"))
        user.State = "support_message"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
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
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals)))
    case "💸 Withdraw":
        if user.Wallet == "" {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Set your wallet first\\.*"))
        } else {
            minWithdrawal, err := getSetting(db, "min_withdrawal")
            if err != nil {
                log.Printf("Error getting min_withdrawal: %v", err)
                return
            }
            minWithdrawalFloat, err := strconv.ParseFloat(minWithdrawal, 64)
            if err != nil {
                log.Printf("Error parsing min_withdrawal: %v", err)
                return
            }
            if user.Balance < minWithdrawalFloat {
                bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💸 *Minimum withdrawal:* %.2f", minWithdrawalFloat)))
            } else {
                bot.Send(tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*"))
                user.State = "withdraw_amount"
                if err := updateUser(db, user); err != nil {
                    log.Printf("Error updating user: %v", err)
                }
            }
        }
    }
}


func handleCallbackQuery(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    user, err := getUser(db, userID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
        return
    }

    if user.Banned == 1 {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *You are banned or not registered\\!*"))
        callbackConfig := tgbotapi.NewCallback(callback.ID, "🚫 Banned")
        bot.Request(callbackConfig)
        return
    }

    if callback.Data == "set_inline" || callback.Data == "set_normal" {
        buttonStyle := "inline"
        if callback.Data == "set_normal" {
            buttonStyle = "normal"
        }
        user.ButtonStyle = buttonStyle
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *Button style set to* %s\\.", escapeMarkdownV2(buttonStyle))))
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
            escapedChannels := strings.Join(channels, "\n")
            bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📢 *Please join:*\n%s", escapeMarkdownV2(escapedChannels))))
        } else {
            showMainMenu(bot, userID, buttonStyle)
        }
        callbackConfig := tgbotapi.NewCallback(callback.ID, "✅ Style set")
        bot.Request(callbackConfig)
        return
    }

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
        escapedChannels := strings.Join(channels, "\n")
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📢 *Please join:*\n%s", escapeMarkdownV2(escapedChannels))))
        callbackConfig := tgbotapi.NewCallback(callback.ID, "📢 Join channels")
        bot.Request(callbackConfig)
        return
    }

    switch callback.Data {
    case "balance":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Balance:* %.2f\n🤝 *Referrals:* %d", user.Balance, user.Referrals)))
    case "set_wallet":
        if user.Wallet != "" {
            escapedWallet := escapeMarkdownV2(user.Wallet)
            msg := tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Your wallet:* `%s`", escapedWallet))
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
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user: %v", err)
            }
        }
    case "change_wallet":
        bot.Send(tgbotapi.NewMessage(userID, "💳 *Enter new wallet address:*"))
        user.State = "setting_wallet"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "support":
        bot.Send(tgbotapi.NewMessage(userID, "📞 *Send your message for support:*"))
        user.State = "support_message"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
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
        referrals, err := getReferrals(db, userID)
        if err != nil {
            log.Printf("Error getting referrals: %v", err)
            return
        }
        if len(referrals) > 0 {
            var usernames []string
            for _, ref := range referrals {
                usernames = append(usernames, ref.Username)
            }
            escapedReferrals := strings.Join(usernames, "\n")
            bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📄 *Your referrals:*\n%s", escapeMarkdownV2(escapedReferrals))))
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
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📈 *Stats:*\n📊 *Total Users:* %d\n💸 *Total Withdrawals:* %d", totalUsers, totalWithdrawals)))
    case "withdraw":
        if user.Wallet == "" {
            bot.Send(tgbotapi.NewMessage(userID, "💳 *Set your wallet first\\.*"))
        } else {
            minWithdrawal, err := getSetting(db, "min_withdrawal")
            if err != nil {
                log.Printf("Error getting min_withdrawal: %v", err)
                return
            }
            minWithdrawalFloat, err := strconv.ParseFloat(minWithdrawal, 64)
            if err != nil {
                log.Printf("Error parsing min_withdrawal: %v", err)
                return
            }
            if user.Balance < minWithdrawalFloat {
                bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💸 *Minimum withdrawal:* %.2f", minWithdrawalFloat)))
            } else {
                bot.Send(tgbotapi.NewMessage(userID, "💸 *Enter amount to withdraw:*"))
                user.State = "withdraw_amount"
                if err := updateUser(db, user); err != nil {
                    log.Printf("Error updating user: %v", err)
                }
            }
        }
    }

    if strings.HasPrefix(callback.Data, "admin_") || callback.Data == "admin_add_channel" || callback.Data == "admin_remove_channel" {
        handleAdminActions(bot, db, callback)
    } else if callback.Data == "qr_enable" || callback.Data == "qr_disable" {
        handleQRSettings(bot, db, callback)
    } else if strings.HasPrefix(callback.Data, "adjust_") || strings.HasPrefix(callback.Data, "ban_") || strings.HasPrefix(callback.Data, "unban_") || strings.HasPrefix(callback.Data, "viewrefs_") || strings.HasPrefix(callback.Data, "contact_") {
        handleAdminUserActions(bot, db, callback)
    }

    callbackConfig := tgbotapi.NewCallback(callback.ID, "")
    bot.Request(callbackConfig)
}

func handleStateMessages(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    userID := update.Message.From.ID
    state := user.State

    switch state {
    case "setting_wallet":
        wallet := strings.TrimSpace(update.Message.Text)
        user.Wallet = wallet
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
            return
        }
        escapedWallet := escapeMarkdownV2(wallet)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💳 *Wallet set to:* `%s`", escapedWallet)))
    case "withdraw_amount":
        amount, err := strconv.ParseFloat(update.Message.Text, 64)
        if err != nil || amount <= 0 || amount > user.Balance {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Enter a valid amount\\.*"))
            return
        }
        user.Balance -= amount
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
            return
        }
        if err := createWithdrawal(db, userID, amount, user.Wallet); err != nil {
            log.Printf("Error creating withdrawal: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(userID, "✅ *Withdrawal request sent\\! Admin will review soon\\.*"))
        paymentChannel, err := getSetting(db, "payment_channel")
        if err != nil {
            log.Printf("Error getting payment_channel: %v", err)
            return
        }
        if paymentChannel != "" {
            escapedUsername := escapeMarkdownV2(user.Username)
            escapedWallet := escapeMarkdownV2(user.Wallet)
            amountStr := fmt.Sprintf("%.2f", amount)
            randomSuffix := rand.Intn(10000000) // 7 digits
            txID := fmt.Sprintf("2025%07d", randomSuffix)
            channels, err := getRequiredChannels(db)
            if err != nil {
                log.Printf("Error getting required channels: %v", err)
                return
            }
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
            firstName := user.Username
            if update.Message.From.FirstName != "" {
                firstName = update.Message.From.FirstName
            }
            escapedFirstName := escapeMarkdownV2(firstName)
            msgText := fmt.Sprintf(
                "🔥 *NEW WITHDRAWAL SENT* 🔥\n\n👤 *USER:* [%s](tg://user?id=%d)\n💎 *USER ID:* `%d`\n💰 *AMOUNT:* %s FREE COIN\n📞 *REFERRER:* %d\n🔗 *ADDRESS:* `%s`\n⏰ *TRANSACTION ID:* `%s`",
                escapedFirstName, userID, userID, escapeMarkdownV2(amountStr), user.Referrals, escapedWallet, txID,
            )
            qrEnabled, err := getSetting(db, "qr_enabled")
            if err != nil {
                log.Printf("Error getting qr_enabled: %v", err)
                return
            }
            if qrEnabled == "1" {
                qr, err := qrcode.New(user.Wallet, qrcode.Medium)
                if err != nil {
                    log.Printf("Error generating QR code: %v", err)
                    bot.Send(tgbotapi.NewMessage(paymentChannel, fmt.Sprintf("%s\n⚠️ *QR code generation failed\\.*", msgText)))
                    return
                }
                var buf bytes.Buffer
                if err := qr.Write(256, &buf); err != nil {
                    log.Printf("Error writing QR code: %v", err)
                    return
                }
                photo := tgbotapi.NewPhoto(paymentChannel, tgbotapi.FileBytes{
                    Name:  "qr.png",
                    Bytes: buf.Bytes(),
                })
                photo.Caption = msgText
                photo.ParseMode = "MarkdownV2"
                photo.ReplyMarkup = markup
                bot.Send(photo)
            } else {
                msg := tgbotapi.NewMessage(paymentChannel, msgText)
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
        escapedUsername := escapeMarkdownV2(user.Username)
        escapedText := escapeMarkdownV2(update.Message.Text)
        bot.Send(tgbotapi.NewMessage(ADMIN_ID, fmt.Sprintf("📞 *Support from* @%s\n%s", escapedUsername, escapedText)))
        bot.Send(tgbotapi.NewMessage(userID, "✅ *Your message has been sent to support\\!*"))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
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
            if err := rows.Scan(&uid); err != nil {
                log.Printf("Error scanning user: %v", err)
                continue
            }
            users = append(users, uid)
        }
        totalUsers := len(users)
        sentCount := 0
        statusMsg := bot.Send(tgbotapi.NewMessage(userID, "📢 *Broadcasting:* [□□□□□□□□□□] 0%"))
        for i, uid := range users {
            if update.Message.Text != "" {
                escapedText := escapeMarkdownV2(update.Message.Text)
                bot.Send(tgbotapi.NewMessage(uid, escapedText))
            }
            sentCount++
            progress := int(float64(sentCount) / float64(totalUsers) * 10)
            bar := strings.Repeat("█", progress) + strings.Repeat("□", 10-progress)
            percentage := int(float64(sentCount) / float64(totalUsers) * 100)
            bot.Send(tgbotapi.NewEditMessageText(userID, statusMsg.MessageID, fmt.Sprintf("📢 *Broadcasting:* [%s] %d%% (%d/%d)", bar, percentage, sentCount, totalUsers)))
            time.Sleep(100 * time.Millisecond)
        }
        bot.Send(tgbotapi.NewEditMessageText(userID, statusMsg.MessageID, fmt.Sprintf("✅ *Broadcast completed\\!* Sent to %d/%d users\\.", sentCount, totalUsers)))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "getting_user_info":
        if userID != ADMIN_ID {
            return
        }
        target := strings.TrimSpace(update.Message.Text)
        var targetUser User
        if _, err := strconv.ParseInt(target, 10, 64); err == nil {
            targetUser, err = getUser(db, int64(atoi(target)))
            if err != nil {
                log.Printf("Error getting user: %v", err)
                return
            }
        } else {
            rows, err := db.Query("SELECT user_id, username, balance, wallet, referrals, referred_by, banned, button_style, state FROM users WHERE username = ?", strings.TrimPrefix(target, "@"))
            if err != nil {
                log.Printf("Error querying user: %v", err)
                return
            }
            defer rows.Close()
            if rows.Next() {
                err := rows.Scan(&targetUser.UserID, &targetUser.Username, &targetUser.Balance, &targetUser.Wallet, &targetUser.Referrals, &targetUser.ReferredBy, &targetUser.Banned, &targetUser.ButtonStyle, &targetUser.State)
                if err != nil {
                    log.Printf("Error scanning user: %v", err)
                    return
                }
            }
        }
        if targetUser.UserID == 0 {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *User not found\\.*"))
        } else {
            escapedUsername := escapeMarkdownV2(targetUser.Username)
            escapedWallet := "Not set"
            if targetUser.Wallet != "" {
                escapedWallet = escapeMarkdownV2(targetUser.Wallet)
            }
            info := fmt.Sprintf(
                "👤 *User Info*\n*ID:* %d\n*Username:* @%s\n*Balance:* %.2f 💰\n*Wallet:* `%s`\n*Referrals:* %d\n*Banned:* %s",
                targetUser.UserID, escapedUsername, targetUser.Balance, escapedWallet, targetUser.Referrals, map[int]string{0: "No", 1: "Yes"}[targetUser.Banned],
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
            bot.Send(tgbotapi.NewMessage(userID, info))
            bot.Send(tgbotapi.NewMessage(userID, "Choose an action:", markup))
        }
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }




    case "setting_min_withdrawal", "setting_referral_reward", "setting_start_message", "setting_payment_channel":
        if userID != ADMIN_ID {
            return
        }
        value := strings.TrimSpace(update.Message.Text)
        key := strings.TrimPrefix(state, "setting_")
        if err := updateSetting(db, key, value); err != nil {
            log.Printf("Error updating setting: %v", err)
            return
        }
        escapedValue := escapeMarkdownV2(value)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *%s set to:* %s", strings.Title(strings.ReplaceAll(key, "_", " ")), escapedValue)))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "add_channel":
        if userID != ADMIN_ID {
            return
        }
        channel := strings.TrimSpace(update.Message.Text)
        if !strings.HasPrefix(channel, "@") {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Use '@' \\(e\\.g\\., @ChannelName\\)\\.*"))
            return
        }
        _, err := bot.GetChat(tgbotapi.ChatInfoConfig{ChatConfig: tgbotapi.ChatConfig{ChatID: channel}})
        if err != nil {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Invalid channel or bot lacks access\\.*"))
            return
        }
        if err := addRequiredChannel(db, channel); err != nil {
            log.Printf("Error adding channel: %v", err)
            return
        }
        escapedChannel := escapeMarkdownV2(channel)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("➕ *Channel* %s *added\\!*", escapedChannel)))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "remove_channel":
        if userID != ADMIN_ID {
            return
        }
        channel := strings.TrimSpace(update.Message.Text)
        if !strings.HasPrefix(channel, "@") {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Use '@' \\(e\\.g\\., @ChannelName\\)\\.*"))
            return
        }
        if err := removeRequiredChannel(db, channel); err != nil {
            log.Printf("Error removing channel: %v", err)
            return
        }
        escapedChannel := escapeMarkdownV2(channel)
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("➖ *Channel* %s *removed\\!*", escapedChannel)))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case strings.HasPrefix(state, "adjusting_balance_"):
        if userID != ADMIN_ID {
            return
        }
        targetUserID, err := strconv.ParseInt(strings.Split(state, "_")[2], 10, 64)
        if err != nil {
            log.Printf("Error parsing target user ID: %v", err)
            return
        }
        amount, err := strconv.ParseFloat(update.Message.Text, 64)
        if err != nil {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Enter a valid number \\(e\\.g\\., \\+10 or \\-5\\)\\.*"))
            return
        }
        targetUser, err := getUser(db, targetUserID)
        if err != nil {
            log.Printf("Error getting target user: %v", err)
            return
        }
        if targetUser.UserID == 0 {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *User not found\\.*"))
            user.State = ""
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user: %v", err)
            }
            return
        }
        newBalance := targetUser.Balance + amount
        if newBalance < 0 {
            bot.Send(tgbotapi.NewMessage(userID, "❌ *Balance cannot be negative\\.*"))
            return
        }
        targetUser.Balance = newBalance
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error updating target user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *Balance updated to* %.2f *for user* %d\\.", newBalance, targetUserID)))
        bot.Send(tgbotapi.NewMessage(targetUserID, fmt.Sprintf("💰 *Your balance updated to* %.2f\\.", newBalance)))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case strings.HasPrefix(state, "contacting_"):
        if userID != ADMIN_ID {
            return
        }
        targetUserID, err := strconv.ParseInt(strings.Split(state, "_")[1], 10, 64)
        if err != nil {
            log.Printf("Error parsing target user ID: %v", err)
            return
        }
        escapedText := escapeMarkdownV2(update.Message.Text)
        bot.Send(tgbotapi.NewMessage(targetUserID, fmt.Sprintf("📩 *Message from Admin:*\n%s", escapedText)))
        bot.Send(tgbotapi.NewMessage(userID, "✅ *Message sent to user\\!*"))
        user.State = ""
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
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
    bot.Send(tgbotapi.NewMessage(userID, "🔧 *Admin Panel* 🔧", markup))
}

func handleAdminActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
        return
    }
    action := callback.Data
    switch action {
    case "admin_broadcast":
        bot.Send(tgbotapi.NewMessage(userID, "📢 *Send message or media to broadcast:*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "broadcast_message"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_user_info":
        bot.Send(tgbotapi.NewMessage(userID, "📊 *Enter user ID or username:*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "getting_user_info"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_set_min_withdrawal":
        bot.Send(tgbotapi.NewMessage(userID, "💰 *Enter new minimum withdrawal:*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "setting_min_withdrawal"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_set_payment_channel":
        bot.Send(tgbotapi.NewMessage(userID, "📡 *Enter payment channel \\(e\\.g\\., @Channel\\):*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "setting_payment_channel"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_set_referral_reward":
        bot.Send(tgbotapi.NewMessage(userID, "🎁 *Enter referral reward amount:*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "setting_referral_reward"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_add_channel":
        bot.Send(tgbotapi.NewMessage(userID, "📌 *Enter channel username \\(e\\.g\\., @Channel\\):*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "add_channel"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_remove_channel":
        bot.Send(tgbotapi.NewMessage(userID, "➖ *Enter channel username \\(e\\.g\\., @Channel\\):*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "remove_channel"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_start_settings":
        bot.Send(tgbotapi.NewMessage(userID, "🚀 *Enter new start message:*"))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = "setting_start_message"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "admin_qr_settings":
        qrEnabled, err := getSetting(db, "qr_enabled")
        if err != nil {
            log.Printf("Error getting qr_enabled: %v", err)
            return
        }
        qrStatus := "Enabled"
        if qrEnabled != "1" {
            qrStatus = "Disabled"
        }
        markup := tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Enable QR", "qr_enable"),
                tgbotapi.NewInlineKeyboardButtonData("Disable QR", "qr_disable"),
            ),
        )
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("🔳 *QR Status:* %s", qrStatus), markup))
    }
}

func handleQRSettings(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
        return
    }
    qrEnabled := "1"
    qrAction := "Enabled"
    if callback.Data == "qr_disable" {
        qrEnabled = "0"
        qrAction = "Disabled"
    }
    if err := updateSetting(db, "qr_enabled", qrEnabled); err != nil {
        log.Printf("Error updating qr_enabled: %v", err)
        return
    }
    bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("🔳 *QR* %s", qrAction)))
}

func handleAdminUserActions(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    userID := callback.From.ID
    if userID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(userID, "🚫 *Unauthorized\\.*"))
        return
    }
    parts := strings.Split(callback.Data, "_")
    action := parts[0]
    targetUserID, err := strconv.ParseInt(parts[1], 10, 64)
    if err != nil {
        log.Printf("Error parsing target user ID: %v", err)
        return
    }

    switch action {
    case "adjust":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("💰 *Enter amount to adjust for user* %d *\\(e\\.g\\., \\+10 or \\-5\\):*", targetUserID)))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = fmt.Sprintf("adjusting_balance_%d", targetUserID)
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    case "ban":
        targetUser, err := getUser(db, targetUserID)
        if err != nil {
            log.Printf("Error getting target user: %v", err)
            return
        }
        targetUser.Banned = 1
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error updating target user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *banned\\.*", targetUserID)))
        bot.Send(tgbotapi.NewMessage(targetUserID, "🚫 *You have been banned from the bot\\.*"))
    case "unban":
        targetUser, err := getUser(db, targetUserID)
        if err != nil {
            log.Printf("Error getting target user: %v", err)
            return
        }
        targetUser.Banned = 0
        if err := updateUser(db, targetUser); err != nil {
            log.Printf("Error updating target user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("✅ *User* %d *unbanned\\.*", targetUserID)))
        bot.Send(tgbotapi.NewMessage(targetUserID, "✅ *You have been unbanned\\!*"))
    case "viewrefs":
        referrals, err := getReferrals(db, targetUserID)
        if err != nil {
            log.Printf("Error getting referrals: %v", err)
            return
        }
        if len(referrals) > 0 {
            var usernames []string
            for _, ref := range referrals {
                usernames = append(usernames, ref.Username)
            }
            bio := bytes.NewBufferString(strings.Join(usernames, "\n"))
            doc := tgbotapi.NewDocument(userID, tgbotapi.FileBytes{
                Name:  fmt.Sprintf("referrals_%d.txt", targetUserID),
                Bytes: bio.Bytes(),
            })
            doc.Caption = fmt.Sprintf("📄 *Referrals for user* %d", targetUserID)
            bot.Send(doc)
        } else {
            bot.Send(tgbotapi.NewMessage(userID, "📄 *No referrals yet\\.*"))
        }
    case "contact":
        bot.Send(tgbotapi.NewMessage(userID, fmt.Sprintf("📩 *Enter message for user* %d:", targetUserID)))
        user, err := getUser(db, userID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }
        user.State = fmt.Sprintf("contacting_%d", targetUserID)
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
        }
    }
}

