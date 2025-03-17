package main

import (
    "bytes"
    "database/sql"
    "fmt"
    "log"
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

    // Create users table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY,
            username TEXT,
            balance REAL DEFAULT 0,
            referrer_id INTEGER,
            state TEXT,
            withdrawal_address TEXT
        )
    `)
    if err != nil {
        return nil, err
    }

    // Create settings table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS settings (
            key TEXT PRIMARY KEY,
            value TEXT
        )
    `)
    if err != nil {
        return nil, err
    }

    // Create withdrawals table
    _, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS withdrawals (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            user_id INTEGER,
            amount REAL,
            address TEXT,
            status TEXT,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
        )
    `)
    if err != nil {
        return nil, err
    }

    // Insert default settings if not exists
    _, err = db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('referral_reward', '0.5')")
    if err != nil {
        return nil, err
    }
    _, err = db.Exec("INSERT OR IGNORE INTO settings (key, value) VALUES ('min_withdrawal', '7.0')")
    if err != nil {
        return nil, err
    }

    return db, nil
}

type User struct {
    ID                int64
    Username          string
    Balance           float64
    ReferrerID        sql.NullInt64
    State             string
    WithdrawalAddress string
}

type Setting struct {
    Key   string
    Value string
}

type Withdrawal struct {
    ID        int64
    UserID    int64
    Amount    float64
    Address   string
    Status    string
    CreatedAt time.Time
}


func getUser(db *sql.DB, userID int64) (User, error) {
    var user User
    err := db.QueryRow("SELECT id, username, balance, referrer_id, state, withdrawal_address FROM users WHERE id = ?", userID).Scan(
        &user.ID, &user.Username, &user.Balance, &user.ReferrerID, &user.State, &user.WithdrawalAddress,
    )
    if err == sql.ErrNoRows {
        return user, nil
    }
    return user, err
}

func getUserByUsername(db *sql.DB, username string) (User, error) {
    var user User
    err := db.QueryRow("SELECT id, username, balance, referrer_id, state, withdrawal_address FROM users WHERE username = ?", username).Scan(
        &user.ID, &user.Username, &user.Balance, &user.ReferrerID, &user.State, &user.WithdrawalAddress,
    )
    if err == sql.ErrNoRows {
        return user, nil
    }
    return user, err
}

func updateUser(db *sql.DB, user User) error {
    _, err := db.Exec(
        "INSERT OR REPLACE INTO users (id, username, balance, referrer_id, state, withdrawal_address) VALUES (?, ?, ?, ?, ?, ?)",
        user.ID, user.Username, user.Balance, user.ReferrerID, user.State, user.WithdrawalAddress,
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

func createWithdrawal(db *sql.DB, userID int64, amount float64, address string) error {
    _, err := db.Exec(
        "INSERT INTO withdrawals (user_id, amount, address, status) VALUES (?, ?, ?, ?)",
        userID, amount, address, "pending",
    )
    return err
}

func getPendingWithdrawals(db *sql.DB) ([]Withdrawal, error) {
    rows, err := db.Query("SELECT id, user_id, amount, address, status, created_at FROM withdrawals WHERE status = 'pending'")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var withdrawals []Withdrawal
    for rows.Next() {
        var w Withdrawal
        if err := rows.Scan(&w.ID, &w.UserID, &w.Amount, &w.Address, &w.Status, &w.CreatedAt); err != nil {
            return nil, err
        }
        withdrawals = append(withdrawals, w)
    }
    return withdrawals, nil
}

func updateWithdrawalStatus(db *sql.DB, withdrawalID int64, status string) error {
    _, err := db.Exec("UPDATE withdrawals SET status = ? WHERE id = ?", status, withdrawalID)
    return err
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

    BOT_USERNAME = "@" + bot.Self.UserName
    bot.Debug = true
    log.Printf("Authorized on account %s", bot.Self.UserName)

    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60

    updates := bot.GetUpdatesChan(u)

    for update := range updates {
        go handleUpdate(bot, db, update)
    }
}

func handleUpdate(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update) {
    if update.Message != nil {
        user, err := getUser(db, update.Message.From.ID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        if user.ID == 0 {
            user.ID = update.Message.From.ID
            user.Username = update.Message.From.UserName
            if update.Message.Text == "/start" {
                user.State = "start"
            } else if strings.HasPrefix(update.Message.Text, "/start ") {
                refIDStr := strings.TrimPrefix(update.Message.Text, "/start ")
                refID, _ := strconv.ParseInt(refIDStr, 10, 64)
                user.ReferrerID = sql.NullInt64{Int64: refID, Valid: refID != 0}
                user.State = "start"
            }
            if err := updateUser(db, user); err != nil {
                log.Printf("Error updating user: %v", err)
                return
            }
        }

        switch user.State {
        case "start":
            handleStart(bot, db, update, user)
        case "withdrawal_address":
            handleWithdrawalAddress(bot, db, update, user)
        default:
            switch update.Message.Text {
            case "/start":
                handleStart(bot, db, update, user)
            case "/balance":
                bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Your balance: %.2f", user.Balance)))
            case "/referral":
                handleReferral(bot, db, update, user)
            case "/withdraw":
                handleWithdraw(bot, db, update, user)
            case "/admin":
                handleAdmin(bot, db, update, user)
            default:
                bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command. Use /start, /balance, /referral, /withdraw, or /admin (for admins)."))
            }
        }
    } else if update.CallbackQuery != nil {
        handleCallbackQuery(bot, db, update.CallbackQuery)
    }
}

func handleStart(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    user.State = ""
    if err := updateUser(db, user); err != nil {
        log.Printf("Error updating user: %v", err)
        return
    }

    msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Welcome to the Bot!\nUse /balance to check your balance.\nUse /referral to get your referral link.\nUse /withdraw to withdraw your earnings.")
    if user.ReferrerID.Valid {
        referrer, err := getUser(db, user.ReferrerID.Int64)
        if err != nil {
            log.Printf("Error getting referrer: %v", err)
            return
        }
        if referrer.ID != 0 {
            reward, err := getSetting(db, "referral_reward")
            if err != nil {
                log.Printf("Error getting referral_reward: %v", err)
                return
            }
            rewardFloat, err := strconv.ParseFloat(reward, 64)
            if err != nil {
                log.Printf("Error parsing referral_reward: %v", err)
                return
            }
            referrer.Balance += rewardFloat
            if err := updateUser(db, referrer); err != nil {
                log.Printf("Error updating referrer: %v", err)
                return
            }
            bot.Send(tgbotapi.NewMessage(referrer.ID, fmt.Sprintf("You earned %.2f for referring a new user!", rewardFloat)))
        }
    }
    bot.Send(msg)
}

func handleReferral(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    referralLink := fmt.Sprintf("https://t.me/%s?start=%d", BOT_USERNAME, user.ID)
    msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Your referral link: %s", referralLink))
    bot.Send(msg)
}

func handleWithdraw(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    if user.WithdrawalAddress == "" {
        user.State = "withdrawal_address"
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
            return
        }
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Please enter your withdrawal address:"))
        return
    }

    if update.Message.Text == "/withdraw" {
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Please enter the amount to withdraw (e.g., /withdraw 7.0):")
        bot.Send(msg)
        return
    }

    amountStr := strings.TrimPrefix(update.Message.Text, "/withdraw ")
    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil {
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Invalid amount. Please use /withdraw <amount> (e.g., /withdraw 7.0)"))
        return
    }

    minWithdrawal, err := getSetting(db, "min_withdrawal")
    if err != nil {
        log.Printf("Error getting min_withdrawal: %v", err)
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Error: Could not fetch minimum withdrawal limit"))
        return
    }
    minWithdrawalFloat, err := strconv.ParseFloat(minWithdrawal, 64)
    if err != nil {
        log.Printf("Error parsing min_withdrawal: %v", err)
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Error: Invalid minimum withdrawal limit"))
        return
    }
    if amount < minWithdrawalFloat {
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Minimum withdrawal is %v", minWithdrawalFloat)))
        return
    }

    if user.Balance < amount {
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Insufficient balance"))
        return
    }

    user.Balance -= amount
    if err := updateUser(db, user); err != nil {
        log.Printf("Error updating user: %v", err)
        return
    }

    if err := createWithdrawal(db, user.ID, amount, user.WithdrawalAddress); err != nil {
        log.Printf("Error creating withdrawal: %v", err)
        return
    }

    msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Withdrawal request of %.2f to %s submitted!", amount, user.WithdrawalAddress))
    msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
        tgbotapi.NewInlineKeyboardRow(
            tgbotapi.NewInlineKeyboardButtonURL("CHANN", "https://t.me/+qB1lX2vD8pY5N2M1"),
            tgbotapi.NewInlineKeyboardButtonURL("JOIN", "https://t.me/+qB1lX2vD8pY5N2M1"),
        ),
    )
    bot.Send(msg)
}

func handleWithdrawalAddress(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    user.WithdrawalAddress = update.Message.Text
    user.State = ""
    if err := updateUser(db, user); err != nil {
        log.Printf("Error updating user: %v", err)
        return
    }
    bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Withdrawal address saved! Use /withdraw to request a withdrawal."))
}

func handleAdmin(bot *tgbotapi.BotAPI, db *sql.DB, update tgbotapi.Update, user User) {
    if user.ID != ADMIN_ID {
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "🚫 Unauthorized"))
        return
    }

    withdrawals, err := getPendingWithdrawals(db)
    if err != nil {
        log.Printf("Error getting withdrawals: %v", err)
        return
    }

    if len(withdrawals) == 0 {
        bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "No pending withdrawals"))
        return
    }

    for _, w := range withdrawals {
        user, err := getUser(db, w.UserID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            continue
        }
        msg := tgbotapi.NewMessage(update.Message.Chat.ID, fmt.Sprintf(
            "Withdrawal ID: %d\nUser: @%s\nAmount: %.2f\nAddress: %s\nCreated: %s",
            w.ID, user.Username, w.Amount, w.Address, w.CreatedAt.Format(time.RFC822),
        ))
        msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(
            tgbotapi.NewInlineKeyboardRow(
                tgbotapi.NewInlineKeyboardButtonData("Approve", fmt.Sprintf("approve_%d", w.ID)),
                tgbotapi.NewInlineKeyboardButtonData("Reject", fmt.Sprintf("reject_%d", w.ID)),
            ),
        )
        bot.Send(msg)
    }
}

func handleCallbackQuery(bot *tgbotapi.BotAPI, db *sql.DB, callback *tgbotapi.CallbackQuery) {
    user, err := getUser(db, callback.From.ID)
    if err != nil {
        log.Printf("Error getting user: %v", err)
        return
    }

    if user.ID != ADMIN_ID {
        callbackConfig := tgbotapi.NewCallback(callback.ID, "🚫 Unauthorized")
        _, err := bot.Request(callbackConfig)
        if err != nil {
            log.Printf("Error answering callback: %v", err)
        }
        return
    }

    data := callback.Data
    if strings.HasPrefix(data, "approve_") {
        withdrawalIDStr := strings.TrimPrefix(data, "approve_")
        withdrawalID, err := strconv.ParseInt(withdrawalIDStr, 10, 64)
        if err != nil {
            log.Printf("Error parsing withdrawal ID: %v", err)
            return
        }

        if err := updateWithdrawalStatus(db, withdrawalID, "approved"); err != nil {
            log.Printf("Error updating withdrawal: %v", err)
            return
        }

        withdrawal, err := getWithdrawal(db, withdrawalID)
        if err != nil {
            log.Printf("Error getting withdrawal: %v", err)
            return
        }

        user, err := getUser(db, withdrawal.UserID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        callbackConfig := tgbotapi.NewCallback(callback.ID, "✅ Approved")
        _, err = bot.Request(callbackConfig)
        if err != nil {
            log.Printf("Error answering callback: %v", err)
        }
        bot.Send(tgbotapi.NewMessage(user.ID, fmt.Sprintf("Your withdrawal of %.2f to %s has been approved!", withdrawal.Amount, withdrawal.Address)))
    } else if strings.HasPrefix(data, "reject_") {
        withdrawalIDStr := strings.TrimPrefix(data, "reject_")
        withdrawalID, err := strconv.ParseInt(withdrawalIDStr, 10, 64)
        if err != nil {
            log.Printf("Error parsing withdrawal ID: %v", err)
            return
        }

        withdrawal, err := getWithdrawal(db, withdrawalID)
        if err != nil {
            log.Printf("Error getting withdrawal: %v", err)
            return
        }

        user, err := getUser(db, withdrawal.UserID)
        if err != nil {
            log.Printf("Error getting user: %v", err)
            return
        }

        user.Balance += withdrawal.Amount
        if err := updateUser(db, user); err != nil {
            log.Printf("Error updating user: %v", err)
            return
        }

        if err := updateWithdrawalStatus(db, withdrawalID, "rejected"); err != nil {
            log.Printf("Error updating withdrawal: %v", err)
            return
        }

        callbackConfig := tgbotapi.NewCallback(callback.ID, "❌ Rejected")
        _, err = bot.Request(callbackConfig)
        if err != nil {
            log.Printf("Error answering callback: %v", err)
        }
        bot.Send(tgbotapi.NewMessage(user.ID, fmt.Sprintf("Your withdrawal of %.2f to %s has been rejected.", withdrawal.Amount, withdrawal.Address)))
    } else if strings.HasPrefix(data, "qrcode_") {
        withdrawalIDStr := strings.TrimPrefix(data, "qrcode_")
        withdrawalID, err := strconv.ParseInt(withdrawalIDStr, 10, 64)
        if err != nil {
            log.Printf("Error parsing withdrawal ID: %v", err)
            return
        }

        withdrawal, err := getWithdrawal(db, withdrawalID)
        if err != nil {
            log.Printf("Error getting withdrawal: %v", err)
            return
        }

        qr, err := qrcode.New(withdrawal.Address, qrcode.Medium)
        if err != nil {
            log.Printf("Error generating QR code: %v", err)
            return
        }

        var buf bytes.Buffer
        if err := qr.Write(256, &buf); err != nil {
            log.Printf("Error writing QR code: %v", err)
            return
        }

        photo := tgbotapi.NewPhoto(callback.Message.Chat.ID, tgbotapi.FileBytes{
            Name:  "qrcode.png",
            Bytes: buf.Bytes(),
        })
        photo.Caption = fmt.Sprintf("QR Code for withdrawal address: %s", withdrawal.Address)
        bot.Send(photo)

        callbackConfig := tgbotapi.NewCallback(callback.ID, "📷 QR Code Generated")
        _, err = bot.Request(callbackConfig)
        if err != nil {
            log.Printf("Error answering callback: %v", err)
        }
    }
}


func getWithdrawal(db *sql.DB, withdrawalID int64) (Withdrawal, error) {
    var w Withdrawal
    err := db.QueryRow("SELECT id, user_id, amount, address, status, created_at FROM withdrawals WHERE id = ?", withdrawalID).Scan(
        &w.ID, &w.UserID, &w.Amount, &w.Address, &w.Status, &w.CreatedAt,
    )
    return w, err
}
