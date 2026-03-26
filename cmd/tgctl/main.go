package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aqin/go-tdlib/client"
)

const usage = `tgctl - Telegram CLI powered by TDLib

Usage:
  tgctl login                          Login to Telegram
  tgctl me                             Show current user info
  tgctl send <chat> <message>          Send a message
  tgctl chats [limit]                  List chats
  tgctl create-bot <name> <username>   Create a new bot via BotFather
  tgctl history <chat> [limit]         Get chat history
  tgctl search <query>                 Search public chats
  tgctl contacts                       List contacts
  tgctl logout                         Logout

Environment:
  TELEGRAM_API_ID       Telegram API ID (required)
  TELEGRAM_API_HASH     Telegram API hash (required)
  TELEGRAM_PHONE        Phone number (optional, will prompt)
  TGCTL_DATA_DIR        Data directory (default: ~/.tgctl)
`

func main() {
	if len(os.Args) < 2 {
		fmt.Print(usage)
		os.Exit(1)
	}

	apiID := os.Getenv("TELEGRAM_API_ID")
	apiHash := os.Getenv("TELEGRAM_API_HASH")
	if apiID == "" || apiHash == "" {
		fmt.Fprintln(os.Stderr, "error: TELEGRAM_API_ID and TELEGRAM_API_HASH are required")
		os.Exit(1)
	}

	dataDir := os.Getenv("TGCTL_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".tgctl")
	}
	os.MkdirAll(dataDir, 0700)

	client.SetLogVerbosity(1)
	c := client.New()
	defer c.Close()

	// start auth flow
	go handleUpdates(c, apiID, apiHash, dataDir)

	// wait for authorization
	if !waitForAuth(c, 30*time.Second) {
		fmt.Fprintln(os.Stderr, "error: authorization timeout")
		os.Exit(1)
	}

	cmd := os.Args[1]
	switch cmd {
	case "login":
		fmt.Println("Logged in successfully.")
	case "me":
		cmdMe(c)
	case "send":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: tgctl send <chat> <message>")
			os.Exit(1)
		}
		cmdSend(c, os.Args[2], strings.Join(os.Args[3:], " "))
	case "chats":
		limit := 20
		if len(os.Args) > 2 {
			fmt.Sscanf(os.Args[2], "%d", &limit)
		}
		cmdChats(c, limit)
	case "create-bot":
		if len(os.Args) < 4 {
			fmt.Fprintln(os.Stderr, "usage: tgctl create-bot <name> <username>")
			os.Exit(1)
		}
		cmdCreateBot(c, os.Args[2], os.Args[3])
	case "history":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tgctl history <chat> [limit]")
			os.Exit(1)
		}
		limit := 20
		if len(os.Args) > 3 {
			fmt.Sscanf(os.Args[3], "%d", &limit)
		}
		cmdHistory(c, os.Args[2], limit)
	case "search":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: tgctl search <query>")
			os.Exit(1)
		}
		cmdSearch(c, os.Args[2])
	case "contacts":
		cmdContacts(c)
	case "logout":
		cmdLogout(c)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		fmt.Print(usage)
		os.Exit(1)
	}
}

var authReady = make(chan struct{})
var authDone bool

func handleUpdates(c *client.Client, apiID, apiHash, dataDir string) {
	reader := bufio.NewReader(os.Stdin)
	for update := range c.Updates() {
		var meta struct {
			Type string `json:"@type"`
		}
		json.Unmarshal(update, &meta)

		switch meta.Type {
		case "updateAuthorizationState":
			var u struct {
				State json.RawMessage `json:"authorization_state"`
			}
			json.Unmarshal(update, &u)
			var state struct {
				Type string `json:"@type"`
			}
			json.Unmarshal(u.State, &state)

			switch state.Type {
			case "authorizationStateWaitTdlibParameters":
				c.Send(map[string]interface{}{
					"@type":                    "setTdlibParameters",
					"database_directory":       filepath.Join(dataDir, "db"),
					"files_directory":          filepath.Join(dataDir, "files"),
					"use_message_database":     true,
					"use_secret_chats":         false,
					"api_id":                   apiID,
					"api_hash":                 apiHash,
					"system_language_code":     "en",
					"device_model":             "tgctl",
					"application_version":      "1.0.0",
					"use_test_dc":              false,
				})

			case "authorizationStateWaitPhoneNumber":
				phone := os.Getenv("TELEGRAM_PHONE")
				if phone == "" {
					fmt.Print("Phone number: ")
					phone, _ = reader.ReadString('\n')
					phone = strings.TrimSpace(phone)
				}
				c.Send(map[string]interface{}{
					"@type":        "setAuthenticationPhoneNumber",
					"phone_number": phone,
				})

			case "authorizationStateWaitCode":
				fmt.Print("Auth code: ")
				code, _ := reader.ReadString('\n')
				code = strings.TrimSpace(code)
				c.Send(map[string]interface{}{
					"@type": "checkAuthenticationCode",
					"code":  code,
				})

			case "authorizationStateWaitPassword":
				fmt.Print("2FA Password: ")
				pw, _ := reader.ReadString('\n')
				pw = strings.TrimSpace(pw)
				c.Send(map[string]interface{}{
					"@type":    "checkAuthenticationPassword",
					"password": pw,
				})

			case "authorizationStateReady":
				authDone = true
				close(authReady)

			case "authorizationStateClosed":
				os.Exit(0)
			}
		}
	}
}

func waitForAuth(c *client.Client, timeout time.Duration) bool {
	select {
	case <-authReady:
		return true
	case <-time.After(timeout):
		return false
	}
}

func cmdMe(c *client.Client) {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type": "getMe",
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var user struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  struct {
			ActiveUsernames []string `json:"active_usernames"`
		} `json:"usernames"`
		Phone string `json:"phone_number"`
	}
	json.Unmarshal(resp, &user)
	fmt.Printf("ID: %d\nName: %s %s\nPhone: %s\n", user.ID, user.FirstName, user.LastName, user.Phone)
	if len(user.Username.ActiveUsernames) > 0 {
		fmt.Printf("Username: @%s\n", user.Username.ActiveUsernames[0])
	}
}

func cmdSend(c *client.Client, chatArg, text string) {
	chatID := resolveChatID(c, chatArg)
	if chatID == 0 {
		fmt.Fprintf(os.Stderr, "error: cannot resolve chat: %s\n", chatArg)
		return
	}
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":   "sendMessage",
		"chat_id": chatID,
		"input_message_content": map[string]interface{}{
			"@type": "inputMessageText",
			"text": map[string]interface{}{
				"@type": "formattedText",
				"text":  text,
			},
		},
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var msg struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(resp, &msg)
	fmt.Printf("Message sent (id: %d)\n", msg.ID)
}

func cmdChats(c *client.Client, limit int) {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type": "getChats",
		"chat_list": map[string]interface{}{
			"@type": "chatListMain",
		},
		"limit": limit,
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var chats struct {
		ChatIDs []int64 `json:"chat_ids"`
	}
	json.Unmarshal(resp, &chats)
	for _, id := range chats.ChatIDs {
		info := getChatInfo(c, id)
		fmt.Printf("%d  %s\n", id, info)
	}
}

func cmdCreateBot(c *client.Client, name, username string) {
	// resolve BotFather
	botFatherID := resolveChatID(c, "BotFather")
	if botFatherID == 0 {
		fmt.Fprintln(os.Stderr, "error: cannot find BotFather")
		return
	}

	steps := []string{"/newbot", name, username}
	for _, msg := range steps {
		sendText(c, botFatherID, msg)
		time.Sleep(2 * time.Second)
	}

	// wait and read last messages to find token
	time.Sleep(3 * time.Second)
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":   "getChatHistory",
		"chat_id": botFatherID,
		"limit":   5,
		"from_message_id": 0,
		"offset": 0,
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error reading BotFather response: %v\n", err)
		return
	}

	var history struct {
		Messages []struct {
			Content struct {
				Text struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"content"`
		} `json:"messages"`
	}
	json.Unmarshal(resp, &history)

	for _, m := range history.Messages {
		text := m.Content.Text.Text
		if strings.Contains(text, ":") && strings.Contains(text, "Use this token") {
			// extract token
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.Contains(line, ":") && !strings.Contains(line, " ") && len(line) > 30 {
					fmt.Printf("Bot created!\nToken: %s\n", line)
					return
				}
			}
		}
	}

	// fallback: print last messages
	fmt.Println("BotFather response:")
	for _, m := range history.Messages {
		if m.Content.Text.Text != "" {
			fmt.Println(m.Content.Text.Text)
			fmt.Println("---")
		}
	}
}

func cmdHistory(c *client.Client, chatArg string, limit int) {
	chatID := resolveChatID(c, chatArg)
	if chatID == 0 {
		fmt.Fprintf(os.Stderr, "error: cannot resolve chat: %s\n", chatArg)
		return
	}
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":           "getChatHistory",
		"chat_id":         chatID,
		"limit":           limit,
		"from_message_id": 0,
		"offset":          0,
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var history struct {
		Messages []struct {
			ID     int64 `json:"id"`
			Sender struct {
				Type   string `json:"@type"`
				UserID int64  `json:"user_id"`
			} `json:"sender_id"`
			Content struct {
				Text struct {
					Text string `json:"text"`
				} `json:"text"`
			} `json:"content"`
			Date int64 `json:"date"`
		} `json:"messages"`
	}
	json.Unmarshal(resp, &history)
	for i := len(history.Messages) - 1; i >= 0; i-- {
		m := history.Messages[i]
		t := time.Unix(m.Date, 0).Format("15:04")
		text := m.Content.Text.Text
		if text == "" {
			text = "[non-text message]"
		}
		fmt.Printf("[%s] %d: %s\n", t, m.Sender.UserID, text)
	}
}

func cmdSearch(c *client.Client, query string) {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type": "searchPublicChats",
		"query": query,
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var result struct {
		ChatIDs []int64 `json:"chat_ids"`
	}
	json.Unmarshal(resp, &result)
	for _, id := range result.ChatIDs {
		info := getChatInfo(c, id)
		fmt.Printf("%d  %s\n", id, info)
	}
}

func cmdContacts(c *client.Client) {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type": "getContacts",
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	var result struct {
		UserIDs []int64 `json:"user_ids"`
	}
	json.Unmarshal(resp, &result)
	for _, id := range result.UserIDs {
		user := getUserInfo(c, id)
		fmt.Printf("%d  %s\n", id, user)
	}
}

func cmdLogout(c *client.Client) {
	_, err := c.SendAndWait(map[string]interface{}{
		"@type": "logOut",
	}, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	fmt.Println("Logged out.")
}

// helpers

func resolveChatID(c *client.Client, chatArg string) int64 {
	var chatID int64
	if _, err := fmt.Sscanf(chatArg, "%d", &chatID); err == nil {
		return chatID
	}
	// search by username
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":    "searchPublicChat",
		"username": strings.TrimPrefix(chatArg, "@"),
	}, 10*time.Second)
	if err != nil {
		return 0
	}
	var chat struct {
		ID int64 `json:"id"`
	}
	json.Unmarshal(resp, &chat)
	return chat.ID
}

func getChatInfo(c *client.Client, chatID int64) string {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":   "getChat",
		"chat_id": chatID,
	}, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("(chat %d)", chatID)
	}
	var chat struct {
		Title string `json:"title"`
	}
	json.Unmarshal(resp, &chat)
	return chat.Title
}

func getUserInfo(c *client.Client, userID int64) string {
	resp, err := c.SendAndWait(map[string]interface{}{
		"@type":   "getUser",
		"user_id": userID,
	}, 5*time.Second)
	if err != nil {
		return fmt.Sprintf("(user %d)", userID)
	}
	var user struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
	}
	json.Unmarshal(resp, &user)
	return fmt.Sprintf("%s %s", user.FirstName, user.LastName)
}

func sendText(c *client.Client, chatID int64, text string) {
	c.Send(map[string]interface{}{
		"@type":   "sendMessage",
		"chat_id": chatID,
		"input_message_content": map[string]interface{}{
			"@type": "inputMessageText",
			"text": map[string]interface{}{
				"@type": "formattedText",
				"text":  text,
			},
		},
	})
}
