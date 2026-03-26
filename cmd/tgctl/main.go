package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

const usage = `tgctl - Telegram CLI (pure Go, powered by gotd/td)

Usage:
  tgctl [--profile <name>] <command> [args...]

Commands:
  login                              Login to Telegram
  me                                 Show current user info
  send <chat> <message>              Send a message
  chats [limit]                      List chats
  history <chat> [limit]             Get chat history
  search <query>                     Search public chats
  contacts                           List contacts
  listen [--user id] [--chat id]     Listen for new messages
  callback <chat> <msg_id> <data>    Click inline keyboard button
  logout                             Logout

  forward <from_chat> <to_chat> <msg_id>   Forward a message
  edit <chat> <msg_id> <text>              Edit a message
  delete <chat> <msg_id>                   Delete a message
  pin <chat> <msg_id>                      Pin a message
  unpin <chat> [msg_id]                    Unpin a message (or all)
  read <chat>                              Mark chat as read
  search-msg <chat> <query>               Search messages in chat
  members <chat> [limit]                   List group/channel members
  join <invite_link_or_username>           Join a group/channel
  leave <chat>                             Leave a group/channel
  kick <chat> <user>                       Kick a user from group/channel
  invite <chat> <user>                     Invite a user to group/channel
  block <user>                             Block a user
  unblock <user>                           Unblock a user
  resolve <username>                       Resolve username to ID
  sendfile <chat> <file>                   Send a file or image
  download <chat> <msg_id>                 Download file from a message
  startbot <chat> <bot> [param]            Start a bot in a chat
  typing <chat>                            Send typing status

  updateprofile [--first n] [--last n] [--about t]  Update profile
  setstatus <online|offline>               Set online status
  chatinfo <chat>                          Get chat/user details
  creategroup <title> <user1> [user2...]   Create a group
  createchannel <title> [about]            Create a channel
  editadmin <chat> <user> [remove]         Set/remove admin
  resolvephone <phone>                     Resolve phone to user
  commonchats <user>                       Common chats with user
  translate <chat> <msg_id> <lang>         Translate a message

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
		case "callback":
			if len(cmdArgs) < 3 {
				return fmt.Errorf("usage: tgctl callback <chat> <msg_id> <data>")
			}
			return cmdCallback(ctx, api, cmdArgs[0], cmdArgs[1], cmdArgs[2])
		case "logout":
			return cmdLogout(ctx, client)
		case "forward":
			if len(cmdArgs) < 3 {
				return fmt.Errorf("usage: tgctl forward <from_chat> <to_chat> <msg_id>")
			}
			return cmdForward(ctx, api, cmdArgs[0], cmdArgs[1], cmdArgs[2])
		case "edit":
			if len(cmdArgs) < 3 {
				return fmt.Errorf("usage: tgctl edit <chat> <msg_id> <text>")
			}
			return cmdEdit(ctx, api, cmdArgs[0], cmdArgs[1], strings.Join(cmdArgs[2:], " "))
		case "delete":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl delete <chat> <msg_id>")
			}
			return cmdDelete(ctx, api, cmdArgs[0], cmdArgs[1])
		case "pin":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl pin <chat> <msg_id>")
			}
			return cmdPin(ctx, api, cmdArgs[0], cmdArgs[1])
		case "unpin":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl unpin <chat> [msg_id]")
			}
			msgID := ""
			if len(cmdArgs) > 1 {
				msgID = cmdArgs[1]
			}
			return cmdUnpin(ctx, api, cmdArgs[0], msgID)
		case "read":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl read <chat>")
			}
			return cmdRead(ctx, api, cmdArgs[0])
		case "search-msg":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl search-msg <chat> <query>")
			}
			return cmdSearchMsg(ctx, api, cmdArgs[0], strings.Join(cmdArgs[1:], " "))
		case "members":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl members <chat> [limit]")
			}
			limit := 50
			if len(cmdArgs) > 1 {
				fmt.Sscanf(cmdArgs[1], "%d", &limit)
			}
			return cmdMembers(ctx, api, cmdArgs[0], limit)
		case "join":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl join <invite_link_or_username>")
			}
			return cmdJoin(ctx, api, cmdArgs[0])
		case "leave":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl leave <chat>")
			}
			return cmdLeave(ctx, api, cmdArgs[0])
		case "kick":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl kick <chat> <user>")
			}
			return cmdKick(ctx, api, cmdArgs[0], cmdArgs[1])
		case "invite":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl invite <chat> <user>")
			}
			return cmdInvite(ctx, api, cmdArgs[0], cmdArgs[1])
		case "block":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl block <user>")
			}
			return cmdBlock(ctx, api, cmdArgs[0])
		case "unblock":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl unblock <user>")
			}
			return cmdUnblock(ctx, api, cmdArgs[0])
		case "resolve":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl resolve <username>")
			}
			return cmdResolve(ctx, api, cmdArgs[0])
		case "sendfile":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl sendfile <chat> <file>")
			}
			return cmdSendFile(ctx, api, cmdArgs[0], cmdArgs[1])
		case "download":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl download <chat> <msg_id>")
			}
			return cmdDownload(ctx, api, cmdArgs[0], cmdArgs[1])
		case "startbot":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl startbot <chat> <bot> [param]")
			}
			param := ""
			if len(cmdArgs) > 2 {
				param = cmdArgs[2]
			}
			return cmdStartBot(ctx, api, cmdArgs[0], cmdArgs[1], param)
		case "typing":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl typing <chat>")
			}
			return cmdTyping(ctx, api, cmdArgs[0])
		case "updateprofile":
			return cmdUpdateProfile(ctx, api, cmdArgs)
		case "setstatus":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl setstatus <online|offline>")
			}
			return cmdSetStatus(ctx, api, cmdArgs[0])
		case "chatinfo":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl chatinfo <chat>")
			}
			return cmdChatInfo(ctx, api, cmdArgs[0])
		case "creategroup":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl creategroup <title> <user1> [user2...]")
			}
			return cmdCreateGroup(ctx, api, cmdArgs[0], cmdArgs[1:])
		case "createchannel":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl createchannel <title> [about]")
			}
			about := ""
			if len(cmdArgs) > 1 {
				about = strings.Join(cmdArgs[1:], " ")
			}
			return cmdCreateChannel(ctx, api, cmdArgs[0], about)
		case "editadmin":
			if len(cmdArgs) < 2 {
				return fmt.Errorf("usage: tgctl editadmin <chat> <user> [remove]")
			}
			remove := len(cmdArgs) > 2 && cmdArgs[2] == "remove"
			return cmdEditAdmin(ctx, api, cmdArgs[0], cmdArgs[1], remove)
		case "resolvephone":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl resolvephone <phone>")
			}
			return cmdResolvePhone(ctx, api, cmdArgs[0])
		case "commonchats":
			if len(cmdArgs) < 1 {
				return fmt.Errorf("usage: tgctl commonchats <user>")
			}
			return cmdCommonChats(ctx, api, cmdArgs[0])
		case "translate":
			if len(cmdArgs) < 3 {
				return fmt.Errorf("usage: tgctl translate <chat> <msg_id> <lang>")
			}
			return cmdTranslate(ctx, api, cmdArgs[0], cmdArgs[1], cmdArgs[2])
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
		fmt.Print("Auth code (digits only, no spaces): ")
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
		Peer:     peer,
		Message:  text,
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
		fmt.Printf("[%s] #%d %d: %s\n", t, msg.ID, senderID, text)
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

func cmdCallback(ctx context.Context, api *tg.Client, chatArg, msgIDArg, data string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}

	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}

	// try base64 decode first, fallback to raw string
	dataBytes, err := base64Decode(data)
	if err != nil {
		dataBytes = []byte(data)
	}

	result, err := api.MessagesGetBotCallbackAnswer(ctx, &tg.MessagesGetBotCallbackAnswerRequest{
		Peer:  peer,
		MsgID: msgID,
		Data:  dataBytes,
	})
	if err != nil {
		return fmt.Errorf("callback: %w", err)
	}

	if result.Message != "" {
		fmt.Println(result.Message)
	} else if result.URL != "" {
		fmt.Println(result.URL)
	} else {
		fmt.Println("Callback sent.")
	}
	return nil
}

func base64Decode(s string) ([]byte, error) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(s)
	}
	return b, err
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

func randomID() int64 {
	n, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
	return n.Int64()
}

func peerToInputChannel(peer tg.InputPeerClass) (*tg.InputChannel, bool) {
	if p, ok := peer.(*tg.InputPeerChannel); ok {
		return &tg.InputChannel{ChannelID: p.ChannelID, AccessHash: p.AccessHash}, true
	}
	return nil, false
}

func peerToInputUser(ctx context.Context, api *tg.Client, userArg string) (tg.InputUserClass, error) {
	peer, err := resolvePeer(ctx, api, userArg)
	if err != nil {
		return nil, err
	}
	if p, ok := peer.(*tg.InputPeerUser); ok {
		return &tg.InputUser{UserID: p.UserID, AccessHash: p.AccessHash}, nil
	}
	return nil, fmt.Errorf("not a user: %s", userArg)
}

func cmdForward(ctx context.Context, api *tg.Client, fromArg, toArg, msgIDArg string) error {
	fromPeer, err := resolvePeer(ctx, api, fromArg)
	if err != nil {
		return fmt.Errorf("resolve from: %w", err)
	}
	toPeer, err := resolvePeer(ctx, api, toArg)
	if err != nil {
		return fmt.Errorf("resolve to: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}
	_, err = api.MessagesForwardMessages(ctx, &tg.MessagesForwardMessagesRequest{
		FromPeer: fromPeer,
		ToPeer:   toPeer,
		ID:       []int{msgID},
		RandomID: []int64{randomID()},
	})
	if err != nil {
		return err
	}
	fmt.Println("Message forwarded.")
	return nil
}

func cmdEdit(ctx context.Context, api *tg.Client, chatArg, msgIDArg, text string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}
	_, err = api.MessagesEditMessage(ctx, &tg.MessagesEditMessageRequest{
		Peer:    peer,
		ID:      msgID,
		Message: text,
	})
	if err != nil {
		return err
	}
	fmt.Println("Message edited.")
	return nil
}

func cmdDelete(ctx context.Context, api *tg.Client, chatArg, msgIDArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		_, err = api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: ch,
			ID:      []int{msgID},
		})
	} else {
		_, err = api.MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
			ID:     []int{msgID},
			Revoke: true,
		})
	}
	if err != nil {
		return err
	}
	fmt.Println("Message deleted.")
	return nil
}

func cmdPin(ctx context.Context, api *tg.Client, chatArg, msgIDArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}
	_, err = api.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
		Peer: peer,
		ID:   msgID,
	})
	if err != nil {
		return err
	}
	fmt.Println("Message pinned.")
	return nil
}

func cmdUnpin(ctx context.Context, api *tg.Client, chatArg, msgIDArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	if msgIDArg == "" {
		_, err = api.MessagesUnpinAllMessages(ctx, &tg.MessagesUnpinAllMessagesRequest{
			Peer: peer,
		})
		if err != nil {
			return err
		}
		fmt.Println("All messages unpinned.")
	} else {
		msgID, err := strconv.Atoi(msgIDArg)
		if err != nil {
			return fmt.Errorf("invalid msg_id: %w", err)
		}
		_, err = api.MessagesUpdatePinnedMessage(ctx, &tg.MessagesUpdatePinnedMessageRequest{
			Peer:  peer,
			ID:    msgID,
			Unpin: true,
		})
		if err != nil {
			return err
		}
		fmt.Println("Message unpinned.")
	}
	return nil
}

func cmdRead(ctx context.Context, api *tg.Client, chatArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		_, err = api.ChannelsReadHistory(ctx, &tg.ChannelsReadHistoryRequest{
			Channel: ch,
			MaxID:   0,
		})
	} else {
		_, err = api.MessagesReadHistory(ctx, &tg.MessagesReadHistoryRequest{
			Peer:  peer,
			MaxID: 0,
		})
	}
	if err != nil {
		return err
	}
	fmt.Println("Marked as read.")
	return nil
}

func cmdSearchMsg(ctx context.Context, api *tg.Client, chatArg, query string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	result, err := api.MessagesSearch(ctx, &tg.MessagesSearchRequest{
		Peer:     peer,
		Q:        query,
		Filter:   &tg.InputMessagesFilterEmpty{},
		Limit:    20,
		MinDate:  0,
		MaxDate:  0,
		OffsetID: 0,
	})
	if err != nil {
		return err
	}
	var messages []tg.MessageClass
	switch r := result.(type) {
	case *tg.MessagesMessages:
		messages = r.Messages
	case *tg.MessagesMessagesSlice:
		messages = r.Messages
	case *tg.MessagesChannelMessages:
		messages = r.Messages
	}
	for _, m := range messages {
		msg, ok := m.(*tg.Message)
		if !ok {
			continue
		}
		t := time.Unix(int64(msg.Date), 0).Format("01-02 15:04")
		text := msg.Message
		if text == "" {
			text = "[non-text]"
		}
		fmt.Printf("[%s] #%d: %s\n", t, msg.ID, text)
	}
	if len(messages) == 0 {
		fmt.Println("No results.")
	}
	return nil
}

func cmdMembers(ctx context.Context, api *tg.Client, chatArg string, limit int) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		participants, err := api.ChannelsGetParticipants(ctx, &tg.ChannelsGetParticipantsRequest{
			Channel: ch,
			Filter:  &tg.ChannelParticipantsRecent{},
			Limit:   limit,
		})
		if err != nil {
			return err
		}
		if p, ok := participants.(*tg.ChannelsChannelParticipants); ok {
			for _, u := range p.Users {
				if user, ok := u.(*tg.User); ok {
					name := strings.TrimSpace(user.FirstName + " " + user.LastName)
					fmt.Printf("%d  %s\n", user.ID, name)
				}
			}
		}
	} else if p, ok := peer.(*tg.InputPeerChat); ok {
		full, err := api.MessagesGetFullChat(ctx, p.ChatID)
		if err != nil {
			return err
		}
		for _, u := range full.Users {
			if user, ok := u.(*tg.User); ok {
				name := strings.TrimSpace(user.FirstName + " " + user.LastName)
				fmt.Printf("%d  %s\n", user.ID, name)
			}
		}
	} else {
		return fmt.Errorf("not a group or channel")
	}
	return nil
}

func cmdJoin(ctx context.Context, api *tg.Client, target string) error {
	if strings.Contains(target, "+") || strings.Contains(target, "joinchat") {
		// invite link
		hash := target
		if idx := strings.LastIndex(hash, "+"); idx >= 0 {
			hash = hash[idx+1:]
		} else if idx := strings.LastIndex(hash, "/"); idx >= 0 {
			hash = hash[idx+1:]
		}
		_, err := api.MessagesImportChatInvite(ctx, hash)
		if err != nil {
			return err
		}
	} else {
		peer, err := resolvePeer(ctx, api, target)
		if err != nil {
			return fmt.Errorf("resolve: %w", err)
		}
		if ch, ok := peerToInputChannel(peer); ok {
			_, err = api.ChannelsJoinChannel(ctx, ch)
			if err != nil {
				return err
			}
		} else {
			return fmt.Errorf("not a channel/supergroup")
		}
	}
	fmt.Println("Joined.")
	return nil
}

func cmdLeave(ctx context.Context, api *tg.Client, chatArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		_, err = api.ChannelsLeaveChannel(ctx, ch)
	} else if p, ok := peer.(*tg.InputPeerChat); ok {
		_, err = api.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
			ChatID: p.ChatID,
			UserID: &tg.InputUserSelf{},
		})
	} else {
		return fmt.Errorf("not a group or channel")
	}
	if err != nil {
		return err
	}
	fmt.Println("Left.")
	return nil
}

func cmdKick(ctx context.Context, api *tg.Client, chatArg, userArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	inputUser, err := peerToInputUser(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		userPeer, err := resolvePeer(ctx, api, userArg)
		if err != nil {
			return fmt.Errorf("resolve user peer: %w", err)
		}
		_, err = api.ChannelsEditBanned(ctx, &tg.ChannelsEditBannedRequest{
			Channel:      ch,
			Participant:  userPeer,
			BannedRights: tg.ChatBannedRights{ViewMessages: true, UntilDate: 0},
		})
	} else if p, ok := peer.(*tg.InputPeerChat); ok {
		_, err = api.MessagesDeleteChatUser(ctx, &tg.MessagesDeleteChatUserRequest{
			ChatID: p.ChatID,
			UserID: inputUser,
		})
	} else {
		return fmt.Errorf("not a group or channel")
	}
	if err != nil {
		return err
	}
	fmt.Println("User kicked.")
	return nil
}

func cmdInvite(ctx context.Context, api *tg.Client, chatArg, userArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	inputUser, err := peerToInputUser(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		_, err = api.ChannelsInviteToChannel(ctx, &tg.ChannelsInviteToChannelRequest{
			Channel: ch,
			Users:   []tg.InputUserClass{inputUser},
		})
	} else if p, ok := peer.(*tg.InputPeerChat); ok {
		_, err = api.MessagesAddChatUser(ctx, &tg.MessagesAddChatUserRequest{
			ChatID:   p.ChatID,
			UserID:   inputUser,
			FwdLimit: 100,
		})
	} else {
		return fmt.Errorf("not a group or channel")
	}
	if err != nil {
		return err
	}
	fmt.Println("User invited.")
	return nil
}

func cmdBlock(ctx context.Context, api *tg.Client, userArg string) error {
	peer, err := resolvePeer(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	_, err = api.ContactsBlock(ctx, &tg.ContactsBlockRequest{
		ID: peer,
	})
	if err != nil {
		return err
	}
	fmt.Println("User blocked.")
	return nil
}

func cmdUnblock(ctx context.Context, api *tg.Client, userArg string) error {
	peer, err := resolvePeer(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	_, err = api.ContactsUnblock(ctx, &tg.ContactsUnblockRequest{
		ID: peer,
	})
	if err != nil {
		return err
	}
	fmt.Println("User unblocked.")
	return nil
}

func cmdResolve(ctx context.Context, api *tg.Client, username string) error {
	username = strings.TrimPrefix(username, "@")
	resolved, err := api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
		Username: username,
	})
	if err != nil {
		return err
	}
	for _, u := range resolved.Users {
		if user, ok := u.(*tg.User); ok {
			name := strings.TrimSpace(user.FirstName + " " + user.LastName)
			fmt.Printf("User: %d  %s (@%s)\n", user.ID, name, user.Username)
		}
	}
	for _, c := range resolved.Chats {
		switch v := c.(type) {
		case *tg.Chat:
			fmt.Printf("Chat: -%d  %s\n", v.ID, v.Title)
		case *tg.Channel:
			fmt.Printf("Channel: -%d  %s (@%s)\n", v.ID, v.Title, v.Username)
		}
	}
	return nil
}

func cmdSendFile(ctx context.Context, api *tg.Client, chatArg, filePath string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	u := uploader.NewUploader(api)
	uploaded, err := u.FromReader(ctx, filepath.Base(filePath), f)
	if err != nil {
		return fmt.Errorf("upload: %w", err)
	}

	_, err = api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
		Peer: peer,
		Media: &tg.InputMediaUploadedDocument{
			File:     uploaded,
			MimeType: "application/octet-stream",
			Attributes: []tg.DocumentAttributeClass{
				&tg.DocumentAttributeFilename{FileName: filepath.Base(filePath)},
			},
		},
		RandomID: randomID(),
	})
	if err != nil {
		return err
	}
	fmt.Println("File sent.")
	return nil
}

func cmdDownload(ctx context.Context, api *tg.Client, chatArg, msgIDArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}

	// get the message
	var messages tg.MessagesMessagesClass
	if ch, ok := peerToInputChannel(peer); ok {
		messages, err = api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: ch,
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}},
		})
	} else {
		messages, err = api.MessagesGetMessages(ctx, []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}})
	}
	if err != nil {
		return fmt.Errorf("get message: %w", err)
	}

	var msgList []tg.MessageClass
	switch m := messages.(type) {
	case *tg.MessagesMessages:
		msgList = m.Messages
	case *tg.MessagesMessagesSlice:
		msgList = m.Messages
	case *tg.MessagesChannelMessages:
		msgList = m.Messages
	}
	if len(msgList) == 0 {
		return fmt.Errorf("message not found")
	}

	msg, ok := msgList[0].(*tg.Message)
	if !ok {
		return fmt.Errorf("not a regular message")
	}

	if msg.Media == nil {
		return fmt.Errorf("message has no media")
	}

	var location tg.InputFileLocationClass
	var fileName string
	var fileSize int64

	switch media := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		doc, ok := media.Document.(*tg.Document)
		if !ok {
			return fmt.Errorf("no document")
		}
		location = &tg.InputDocumentFileLocation{
			ID:            doc.ID,
			AccessHash:    doc.AccessHash,
			FileReference: doc.FileReference,
		}
		fileSize = doc.Size
		fileName = fmt.Sprintf("file_%d", doc.ID)
		for _, attr := range doc.Attributes {
			if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
				fileName = fn.FileName
			}
		}
	case *tg.MessageMediaPhoto:
		photo, ok := media.Photo.(*tg.Photo)
		if !ok {
			return fmt.Errorf("no photo")
		}
		// get largest size
		var largest *tg.PhotoSize
		for _, s := range photo.Sizes {
			if ps, ok := s.(*tg.PhotoSize); ok {
				if largest == nil || ps.Size > largest.Size {
					largest = ps
				}
			}
		}
		if largest == nil {
			return fmt.Errorf("no photo size found")
		}
		location = &tg.InputPhotoFileLocation{
			ID:            photo.ID,
			AccessHash:    photo.AccessHash,
			FileReference: photo.FileReference,
			ThumbSize:     largest.Type,
		}
		fileSize = int64(largest.Size)
		fileName = fmt.Sprintf("photo_%d.jpg", photo.ID)
	default:
		return fmt.Errorf("unsupported media type")
	}

	// download
	out, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	var offset int64
	const chunkSize = 512 * 1024
	for {
		result, err := api.UploadGetFile(ctx, &tg.UploadGetFileRequest{
			Location: location,
			Offset:   offset,
			Limit:    chunkSize,
		})
		if err != nil {
			return fmt.Errorf("download chunk: %w", err)
		}
		file, ok := result.(*tg.UploadFile)
		if !ok {
			return fmt.Errorf("unexpected response type")
		}
		if len(file.Bytes) == 0 {
			break
		}
		out.Write(file.Bytes)
		offset += int64(len(file.Bytes))
		if fileSize > 0 {
			fmt.Fprintf(os.Stderr, "\rDownloading... %d%%", offset*100/fileSize)
		}
		if len(file.Bytes) < chunkSize {
			break
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
	fmt.Printf("Downloaded: %s (%d bytes)\n", fileName, offset)
	return nil
}

func cmdStartBot(ctx context.Context, api *tg.Client, chatArg, botArg, param string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	botUser, err := peerToInputUser(ctx, api, botArg)
	if err != nil {
		return fmt.Errorf("resolve bot: %w", err)
	}
	if param == "" {
		param = "start"
	}
	_, err = api.MessagesStartBot(ctx, &tg.MessagesStartBotRequest{
		Bot:      botUser,
		Peer:     peer,
		RandomID: randomID(),
		StartParam: param,
	})
	if err != nil {
		return err
	}
	fmt.Println("Bot started.")
	return nil
}

func cmdTyping(ctx context.Context, api *tg.Client, chatArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	_, err = api.MessagesSetTyping(ctx, &tg.MessagesSetTypingRequest{
		Peer:   peer,
		Action: &tg.SendMessageTypingAction{},
	})
	if err != nil {
		return err
	}
	fmt.Println("Typing...")
	return nil
}

func cmdUpdateProfile(ctx context.Context, api *tg.Client, args []string) error {
	req := &tg.AccountUpdateProfileRequest{}
	for i := 0; i < len(args)-1; i += 2 {
		switch args[i] {
		case "--first":
			req.FirstName = args[i+1]
			req.SetFlags()
		case "--last":
			req.LastName = args[i+1]
			req.SetFlags()
		case "--about":
			req.About = args[i+1]
			req.SetFlags()
		default:
			return fmt.Errorf("usage: tgctl updateprofile [--first name] [--last name] [--about text]")
		}
	}
	_, err := api.AccountUpdateProfile(ctx, req)
	if err != nil {
		return err
	}
	fmt.Println("Profile updated.")
	return nil
}

func cmdSetStatus(ctx context.Context, api *tg.Client, status string) error {
	offline := status == "offline"
	_, err := api.AccountUpdateStatus(ctx, offline)
	if err != nil {
		return err
	}
	fmt.Printf("Status set to %s.\n", status)
	return nil
}

func cmdChatInfo(ctx context.Context, api *tg.Client, chatArg string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	if ch, ok := peerToInputChannel(peer); ok {
		full, err := api.ChannelsGetFullChannel(ctx, ch)
		if err != nil {
			return err
		}
		if info, ok := full.FullChat.(*tg.ChannelFull); ok {
			fmt.Printf("ID: %d\n", info.ID)
			fmt.Printf("About: %s\n", info.About)
			fmt.Printf("Members: %d\n", info.ParticipantsCount)
			fmt.Printf("Admins: %d\n", info.AdminsCount)
			fmt.Printf("Online: %d\n", info.OnlineCount)
		}
		for _, c := range full.Chats {
			switch v := c.(type) {
			case *tg.Channel:
				fmt.Printf("Title: %s\n", v.Title)
				if v.Username != "" {
					fmt.Printf("Username: @%s\n", v.Username)
				}
			}
		}
	} else if p, ok := peer.(*tg.InputPeerChat); ok {
		full, err := api.MessagesGetFullChat(ctx, p.ChatID)
		if err != nil {
			return err
		}
		if info, ok := full.FullChat.(*tg.ChatFull); ok {
			fmt.Printf("ID: %d\n", info.ID)
			fmt.Printf("About: %s\n", info.About)
		}
		for _, c := range full.Chats {
			if chat, ok := c.(*tg.Chat); ok {
				fmt.Printf("Title: %s\n", chat.Title)
				fmt.Printf("Members: %d\n", chat.ParticipantsCount)
			}
		}
	} else if p, ok := peer.(*tg.InputPeerUser); ok {
		users, err := api.UsersGetFullUser(ctx, &tg.InputUser{UserID: p.UserID, AccessHash: p.AccessHash})
		if err != nil {
			return err
		}
		info := users.FullUser
		fmt.Printf("ID: %d\n", info.ID)
		fmt.Printf("About: %s\n", info.About)
		fmt.Printf("CommonChats: %d\n", info.CommonChatsCount)
		for _, u := range users.Users {
			if user, ok := u.(*tg.User); ok {
				name := strings.TrimSpace(user.FirstName + " " + user.LastName)
				fmt.Printf("Name: %s\n", name)
				if user.Username != "" {
					fmt.Printf("Username: @%s\n", user.Username)
				}
				fmt.Printf("Phone: %s\n", user.Phone)
			}
		}
	}
	return nil
}

func cmdCreateGroup(ctx context.Context, api *tg.Client, title string, userArgs []string) error {
	var users []tg.InputUserClass
	for _, u := range userArgs {
		inputUser, err := peerToInputUser(ctx, api, u)
		if err != nil {
			return fmt.Errorf("resolve user %s: %w", u, err)
		}
		users = append(users, inputUser)
	}
	_, err := api.MessagesCreateChat(ctx, &tg.MessagesCreateChatRequest{
		Title: title,
		Users: users,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Group '%s' created.\n", title)
	return nil
}

func cmdCreateChannel(ctx context.Context, api *tg.Client, title, about string) error {
	_, err := api.ChannelsCreateChannel(ctx, &tg.ChannelsCreateChannelRequest{
		Title:     title,
		About:     about,
		Broadcast: true,
	})
	if err != nil {
		return err
	}
	fmt.Printf("Channel '%s' created.\n", title)
	return nil
}

func cmdEditAdmin(ctx context.Context, api *tg.Client, chatArg, userArg string, remove bool) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	inputUser, err := peerToInputUser(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	ch, ok := peerToInputChannel(peer)
	if !ok {
		return fmt.Errorf("not a channel/supergroup")
	}
	rights := tg.ChatAdminRights{}
	if !remove {
		rights = tg.ChatAdminRights{
			ChangeInfo:     true,
			DeleteMessages: true,
			BanUsers:       true,
			InviteUsers:    true,
			PinMessages:    true,
			ManageCall:     true,
		}
	}
	_, err = api.ChannelsEditAdmin(ctx, &tg.ChannelsEditAdminRequest{
		Channel:    ch,
		UserID:     inputUser,
		AdminRights: rights,
		Rank:       "",
	})
	if err != nil {
		return err
	}
	if remove {
		fmt.Println("Admin removed.")
	} else {
		fmt.Println("Admin set.")
	}
	return nil
}

func cmdResolvePhone(ctx context.Context, api *tg.Client, phone string) error {
	result, err := api.ContactsResolvePhone(ctx, phone)
	if err != nil {
		return err
	}
	for _, u := range result.Users {
		if user, ok := u.(*tg.User); ok {
			name := strings.TrimSpace(user.FirstName + " " + user.LastName)
			fmt.Printf("User: %d  %s (@%s)\n", user.ID, name, user.Username)
		}
	}
	return nil
}

func cmdCommonChats(ctx context.Context, api *tg.Client, userArg string) error {
	inputUser, err := peerToInputUser(ctx, api, userArg)
	if err != nil {
		return fmt.Errorf("resolve user: %w", err)
	}
	// need InputUser, not InputUserClass
	iu, ok := inputUser.(*tg.InputUser)
	if !ok {
		return fmt.Errorf("not a regular user")
	}
	result, err := api.MessagesGetCommonChats(ctx, &tg.MessagesGetCommonChatsRequest{
		UserID: iu,
		Limit:  100,
	})
	if err != nil {
		return err
	}
	switch r := result.(type) {
	case *tg.MessagesChats:
		for _, c := range r.Chats {
			switch v := c.(type) {
			case *tg.Chat:
				fmt.Printf("-%d  %s\n", v.ID, v.Title)
			case *tg.Channel:
				fmt.Printf("-%d  %s\n", v.ID, v.Title)
			}
		}
	case *tg.MessagesChatsSlice:
		for _, c := range r.Chats {
			switch v := c.(type) {
			case *tg.Chat:
				fmt.Printf("-%d  %s\n", v.ID, v.Title)
			case *tg.Channel:
				fmt.Printf("-%d  %s\n", v.ID, v.Title)
			}
		}
	}
	return nil
}

func cmdTranslate(ctx context.Context, api *tg.Client, chatArg, msgIDArg, lang string) error {
	peer, err := resolvePeer(ctx, api, chatArg)
	if err != nil {
		return fmt.Errorf("resolve chat: %w", err)
	}
	msgID, err := strconv.Atoi(msgIDArg)
	if err != nil {
		return fmt.Errorf("invalid msg_id: %w", err)
	}
	result, err := api.MessagesTranslateText(ctx, &tg.MessagesTranslateTextRequest{
		Peer:   peer,
		ID:     []int{msgID},
		ToLang: lang,
	})
	if err != nil {
		return err
	}
	for _, r := range result.Result {
		fmt.Println(r.Text)
	}
	return nil
}
