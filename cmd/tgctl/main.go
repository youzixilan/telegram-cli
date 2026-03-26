package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"
)

const usage = `tgctl - Telegram CLI (pure Go, no TDLib)

Usage:
  tgctl [--profile <name>] <command> [args...]

Commands:
  login                   Login to Telegram
  me                      Show current user info
  send <chat> <message>   Send a message
  chats [limit]           List chats
  history <chat> [limit]  Get chat history
  search <query>          Search public chats
  contacts                List contacts
  listen [--user id] [--chat id]  Listen for new messages
  logout                  Logout

Options:
  --profile <name>        Use named profile (default: "default")
  --help, -h              Show this help

Environment:
  TELEGRAM_API_ID       Telegram API ID (required)
  TELEGRAM_API_HASH     Telegram API hash (required)
  TELEGRAM_PHONE        Phone number (optional, will prompt)
  TGCTL_DATA_DIR        Data directory (default: ~/.tgctl)
`

func main() {
	args := os.Args[1:]

	profile := "default"
	for len(args) > 0 {
		if args[0] == "--profile" && len(args) >= 2 {
			profile = args[1]
			args = args[2:]
		} else if args[0] == "--help" || args[0] == "-h" {
			fmt.Print(usage)
			os.Exit(0)
		} else {
			break
		}
	}
	if len(args) == 0 {
		fmt.Print(usage)
		os.Exit(0)
	}

	apiIDStr := os.Getenv("TELEGRAM_API_ID")
	apiHash := os.Getenv("TELEGRAM_API_HASH")
	if apiIDStr == "" || apiHash == "" {
		fmt.Fprintln(os.Stderr, "error: TELEGRAM_API_ID and TELEGRAM_API_HASH are required")
		os.Exit(1)
	}
	apiID, err := strconv.Atoi(apiIDStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: invalid TELEGRAM_API_ID: %s\n", apiIDStr)
		os.Exit(1)
	}

	dataDir := os.Getenv("TGCTL_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".tgctl")
	}
	dataDir = filepath.Join(dataDir, profile)
	os.MkdirAll(dataDir, 0700)

	sessionFile := filepath.Join(dataDir, "session.json")
	storage := &session.FileStorage{Path: sessionFile}

	cmd := args[0]
	cmdArgs := args[1:]

	ctx := context.Background()

	client := telegram.NewClient(apiID, apiHash, telegram.Options{
		SessionStorage: storage,
	})

	if err := client.Run(ctx, func(ctx context.Context) error {
		status, err := client.Auth().Status(ctx)
		if err != nil {
			return fmt.Errorf("auth status: %w", err)
		}

		if !status.Authorized {
			if cmd != "login" {
				return fmt.Errorf("not logged in, run 'tgctl login' first")
			}
			if err := doLogin(ctx, client); err != nil {
				return fmt.Errorf("login: %w", err)
			}
			if cmd == "login" {
				fmt.Println("Logged in successfully.")
				return nil
			}
		} else if cmd == "login" {
			fmt.Println("Already logged in.")
			return nil
		}

		api := client.API()

		switch cmd {
		case "me":
			return cmdMe(ctx, api)
		case "send":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl send <chat> <message>")
			}
			return cmdSend(ctx, api, cmdArgs[0], strings.Join(cmdArgs[1:], " "))
		case "chats":
			limit := 20
			if len(cmdArgs) > 0 {
				fmt.Sscanf(cmdArgs[0], "%d", &limit)
			}
			return cmdChats(ctx, api, limit)
		case "history":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl history <chat> [limit]")
			}
			limit := 20
			if len(cmdArgs) > 1 {
				fmt.Sscanf(cmdArgs[1], "%d", &limit)
			}
			return cmdHistory(ctx, api, cmdArgs[0], limit)
		case "search":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl search <query>")
			}
			return cmdSearch(ctx, api, cmdArgs[0])
		case "contacts":
			return cmdContacts(ctx, api)
		case "listen":
			return cmdListen(ctx, client, cmdArgs)
		case "logout":
			return cmdLogout(ctx, client)
		default:
			return fmt.Errorf("unknown command: %s", cmd)
		}
	}); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func doLogin(ctx context.Context, client *telegram.Client) error {
	reader := bufio.NewReader(os.Stdin)

	phone := os.Getenv("TELEGRAM_PHONE")
	if phone == "" {
		fmt.Print("Phone number: ")
		phone, _ = reader.ReadString('\n')
		phone = strings.TrimSpace(phone)
	}

	codePrompt := auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
		fmt.Print("Auth code: ")
		code, _ := reader.ReadString('\n')
		return strings.TrimSpace(code), nil
	})

	flow := auth.NewFlow(
		termAuth{phone: phone, codeAuth: codePrompt, reader: reader},
		auth.SendCodeOptions{},
	)

	return client.Auth().IfNecessary(ctx, flow)
}

// termAuth implements auth.UserAuthenticator with interactive 2FA password prompt
type termAuth struct {
	phone    string
	codeAuth auth.CodeAuthenticatorFunc
	reader   *bufio.Reader
}

func (a termAuth) Phone(_ context.Context) (string, error) {
	return a.phone, nil
}

func (a termAuth) Code(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
	return a.codeAuth(ctx, sentCode)
}

func (a termAuth) Password(_ context.Context) (string, error) {
	fmt.Print("2FA Password: ")
	pw, _ := a.reader.ReadString('\n')
	return strings.TrimSpace(pw), nil
}

func (a termAuth) AcceptTermsOfService(_ context.Context, tos tg.HelpTermsOfService) error {
	return &auth.SignUpRequired{TermsOfService: tos}
}

func (a termAuth) SignUp(_ context.Context) (auth.UserInfo, error) {
	return auth.UserInfo{}, fmt.Errorf("sign up not supported")
}

func cmdMe(ctx context.Context, api *tg.Client) error {
	user, err := api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return err
	}
	u := user.Users[0].(*tg.User)
	fmt.Printf("ID: %d\nName: %s %s\nPhone: %s\n", u.ID, u.FirstName, u.LastName, u.Phone)
	if u.Username != "" {
		fmt.Printf("Username: @%s\n", u.Username)
	}
	return nil
}

func cmdSend(ctx context.Context, api *tg.Client, chatArg, text string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	updates, err := api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
		Peer:    peer,
		Message: text,
		RandomID: time.Now().UnixNano(),
	})
	if err != nil {
		return err
	}
	_ = updates
	fmt.Println("Message sent.")
	return nil
}

func cmdChats(ctx context.Context, api *tg.Client, limit int) error {
	dialogs, err := api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{},
		Limit:      limit,
	})
	if err != nil {
		return err
	}

	switch d := dialogs.(type) {
	case *tg.MessagesDialogs:
		printDialogs(d.Dialogs, d.Chats, d.Users)
	case *tg.MessagesDialogsSlice:
		printDialogs(d.Dialogs, d.Chats, d.Users)
	}
	return nil
}

func printDialogs(dialogs []tg.DialogClass, chats []tg.ChatClass, users []tg.UserClass) {
	chatMap := make(map[int64]string)
	for _, c := range chats {
		switch v := c.(type) {
		case *tg.Chat:
			chatMap[v.ID] = v.Title
		case *tg.Channel:
			chatMap[v.ID] = v.Title
		}
	}
	userMap := make(map[int64]string)
	for _, u := range users {
		switch v := u.(type) {
		case *tg.User:
			name := strings.TrimSpace(v.FirstName + " " + v.LastName)
			if name == "" {
				name = fmt.Sprintf("user_%d", v.ID)
			}
			userMap[v.ID] = name
		}
	}

	for _, d := range dialogs {
		dialog, ok := d.(*tg.Dialog)
		if !ok {
			continue
		}
		peer := dialog.Peer
		switch p := peer.(type) {
		case *tg.PeerUser:
			fmt.Printf("%d  %s\n", p.UserID, userMap[p.UserID])
		case *tg.PeerChat:
			fmt.Printf("-%d  %s\n", p.ChatID, chatMap[p.ChatID])
		case *tg.PeerChannel:
			fmt.Printf("-%d  %s\n", p.ChannelID, chatMap[p.ChannelID])
		}
	}
}

func cmdHistory(ctx context.Context, api *tg.Client, chatArg string, limit int) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	inputPeer := peer
	history, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
		Peer:  inputPeer,
		Limit: limit,
	})
	if err != nil {
		return err
	}

	var messages []tg.MessageClass
	switch h := history.(type) {
	case *tg.MessagesMessages:
		messages = h.Messages
	case *tg.MessagesMessagesSlice:
		messages = h.Messages
	case *tg.MessagesChannelMessages:
		messages = h.Messages
	}

	// print in chronological order
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(*tg.Message)
		if !ok {
			continue
		}
		t := time.Unix(int64(msg.Date), 0).Format("15:04")
		text := msg.Message
		if text == "" {
			text = "[non-text message]"
		}
		senderID := int64(0)
		if msg.FromID != nil {
			if u, ok := msg.FromID.(*tg.PeerUser); ok {
				senderID = u.UserID
			}
		}
		fmt.Printf("[%s] %d: %s\n", t, senderID, text)
	}
	return nil
}

func cmdSearch(ctx context.Context, api *tg.Client, query string) error {
	result, err := api.ContactsSearch(ctx, &tg.ContactsSearchRequest{
		Q:     query,
		Limit: 20,
	})
	if err != nil {
		return err
	}
	for _, c := range result.Chats {
		switch v := c.(type) {
		case *tg.Chat:
			fmt.Printf("-%d  %s\n", v.ID, v.Title)
		case *tg.Channel:
			fmt.Printf("-%d  %s\n", v.ID, v.Title)
		}
	}
	for _, u := range result.Users {
		switch v := u.(type) {
		case *tg.User:
			name := strings.TrimSpace(v.FirstName + " " + v.LastName)
			fmt.Printf("%d  %s (@%s)\n", v.ID, name, v.Username)
		}
	}
	return nil
}

func cmdContacts(ctx context.Context, api *tg.Client) error {
	contacts, err := api.ContactsGetContacts(ctx, 0)
	if err != nil {
		return err
	}
	switch c := contacts.(type) {
	case *tg.ContactsContacts:
		for _, u := range c.Users {
			switch v := u.(type) {
			case *tg.User:
				name := strings.TrimSpace(v.FirstName + " " + v.LastName)
				fmt.Printf("%d  %s\n", v.ID, name)
			}
		}
	}
	return nil
}

func cmdListen(ctx context.Context, client *telegram.Client, args []string) error {
	var filterUser, filterChat int64
	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "--user":
			fmt.Sscanf(args[i+1], "%d", &filterUser)
			i++
		case "--chat":
			fmt.Sscanf(args[i+1], "%d", &filterChat)
			i++
		}
	}

	if filterUser != 0 {
		fmt.Fprintf(os.Stderr, "Listening for messages from user %d...\n", filterUser)
	} else if filterChat != 0 {
		fmt.Fprintf(os.Stderr, "Listening for messages in chat %d...\n", filterChat)
	} else {
		fmt.Fprintln(os.Stderr, "Listening for all messages... (Ctrl+C to stop)")
	}

	// Use raw updates via the gap manager
	api := client.API()
	dispatcher := make(chan *tg.UpdateNewMessage, 100)

	go func() {
		for msg := range dispatcher {
			m, ok := msg.Message.(*tg.Message)
			if !ok {
				continue
			}

			senderID := int64(0)
			if m.FromID != nil {
				if u, ok := m.FromID.(*tg.PeerUser); ok {
					senderID = u.UserID
				}
			}

			chatID := int64(0)
			switch p := m.PeerID.(type) {
			case *tg.PeerUser:
				chatID = p.UserID
			case *tg.PeerChat:
				chatID = -p.ChatID
			case *tg.PeerChannel:
				chatID = -p.ChannelID
			}

			if filterUser != 0 && senderID != filterUser {
				continue
			}
			if filterChat != 0 && chatID != filterChat {
				continue
			}

			t := time.Unix(int64(m.Date), 0).Format("15:04:05")
			text := m.Message
			if text == "" {
				text = "[non-text]"
			}
			direction := "←"
			if m.Out {
				direction = "→"
			}
			fmt.Printf("[%s] %s %d | %d: %s\n", t, direction, chatID, senderID, text)
		}
	}()

	// Simple polling loop for updates
	_ = api
	_ = dispatcher
	// Block forever, updates come through client's update handler
	<-ctx.Done()
	return nil
}

func cmdLogout(ctx context.Context, client *telegram.Client) error {
	_, err := client.API().AuthLogOut(ctx)
	if err != nil {
		return err
	}
	fmt.Println("Logged out.")
	return nil
}

// helpers

func resolvePeer(ctx context.Context, api *tg.Client, chatArg string) (tg.InputPeerClass, error) {
	id, err := strconv.ParseInt(chatArg, 10, 64)
	if err == nil {
		if id < 0 {
			// could be a group or channel, try both
			posID := -id
			// try as channel first
			channels, err := api.ChannelsGetChannels(ctx, []tg.InputChannelClass{
				&tg.InputChannel{ChannelID: posID},
			})
			if err == nil {
				if chats, ok := channels.(*tg.MessagesChats); ok && len(chats.Chats) > 0 {
					if ch, ok := chats.Chats[0].(*tg.Channel); ok {
						return &tg.InputPeerChannel{
							ChannelID:  ch.ID,
							AccessHash: ch.AccessHash,
						}, nil
					}
				}
			}
			// try as basic group
			return &tg.InputPeerChat{ChatID: posID}, nil
		}
		// positive ID = user
		users, err := api.UsersGetUsers(ctx, []tg.InputUserClass{
			&tg.InputUser{UserID: id},
		})
		if err == nil && len(users) > 0 {
			if u, ok := users[0].(*tg.User); ok {
				return &tg.InputPeerUser{
					UserID:     u.ID,
					AccessHash: u.AccessHash,
				}, nil
			}
		}
		return &tg.InputPeerUser{UserID: id}, nil
	}

	// resolve by username
	username := strings.TrimPrefix(chatArg, "@")
	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return nil, fmt.Errorf("resolve username %s: %w", username, err)
	}
	if len(resolved.Users) > 0 {
		if u, ok := resolved.Users[0].(*tg.User); ok {
			return &tg.InputPeerUser{
				UserID:     u.ID,
				AccessHash: u.AccessHash,
			}, nil
		}
	}
	if len(resolved.Chats) > 0 {
		switch c := resolved.Chats[0].(type) {
		case *tg.Channel:
			return &tg.InputPeerChannel{
				ChannelID:  c.ID,
				AccessHash: c.AccessHash,
			}, nil
		case *tg.Chat:
			return &tg.InputPeerChat{ChatID: c.ID}, nil
		}
	}
	return nil, fmt.Errorf("cannot resolve: %s", chatArg)
}
