package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/doylecnn/new-nsfc-bot/storage"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	log "github.com/sirupsen/logrus"
)

func cmdAddFC(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	if len(args) <= 1 {
		return
	}
	var username = message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName + " " + message.From.LastName
	}
	accounts, parseAccountErr := storage.ParseAccountsFromString(args, username)
	if len(accounts) == 0 && parseAccountErr != nil {
		return nil, Error{InnerError: parseAccountErr,
			ReplyText: fmt.Sprintf("FC 格式错，接受完整FC 格式或不含 - 或 SW 的12位纯数字。%v", parseAccountErr),
		}
	}

	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("创建用户信息时出错: %v", err),
		}
	}
	if u == nil {
		username := message.From.UserName
		if len(username) == 0 {
			username = strings.TrimSpace(message.From.FirstName + " " + message.From.LastName)
		}
		if len(username) == 0 {
			username = fmt.Sprintf("%d", message.From.ID)
		}
		u = &storage.User{ID: message.From.ID, Name: username, NSAccounts: accounts}
		if err = u.Create(ctx); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("创建用户信息时出错: %v", err),
			}
		}
	} else {
		if err = u.AddNSAccounts(ctx, accounts); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("插入 Friend Code 时出错: %v", err),
			}
		}
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("完成。添加/更新了 %d 个 Friend Code", len(accounts))},
		nil
}

func cmdMyFC(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if u == nil {
		log.Debug("没有找到用户记录")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addfc 命令添加记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	accounts, err := u.GetAccounts(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if len(accounts) == 0 {
		log.Debug("account count: 0")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addfc 命令添加记录"},
			nil
	}
	var astr []string
	for _, account := range accounts {
		astr = append(astr, account.String())
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(astr, "\n")},
		nil
}

func cmdSearchFC(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	ctx := context.Background()
	var us []*storage.User
	if message.ReplyToMessage != nil && message.ReplyToMessage.From.ID != message.From.ID {
		u, err := storage.GetUser(ctx, message.ReplyToMessage.From.ID)
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错了",
			}
		}
		if u != nil {
			us = []*storage.User{u}
		}
	} else if len(args) > 1 && strings.HasPrefix(args, "@") && args[1:] != message.From.UserName {
		us, err = storage.GetUsersByName(ctx, args[1:])
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错了",
			}
		}
	} else {
		return nil, errors.New("not reply to any message or not at anyone")
	}

	if len(us) == 0 {
		log.Info("users count == 0")
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到对方的记录"},
			nil
	}

	var user *storage.User
	for _, u := range us {
		chatmember, err := tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: message.Chat.ID, UserID: u.ID})
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错了",
			}
		}
		if chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator() {
			user = u
			break
		}
	}

	if user == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到对方的记录"},
			nil
	}

	accounts, err := user.GetAccounts(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if len(accounts) == 0 {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "对方尚未登记过自己的 Friend Code"},
			nil
	}

	var astr []string
	for _, account := range accounts {
		astr = append(astr, account.String())
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(astr, "\n")},
		nil
}

func inlineQueryMyFC(query *tgbotapi.InlineQuery) (*tgbotapi.InlineConfig, error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, query.From.ID)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, errors.New("user not found")
	}
	log.Debugf("user:%s", u.Name)
	accounts, err := u.GetAccounts(ctx)
	if err != nil {
		return nil, err
	}
	if len(accounts) == 0 {
		return nil, errors.New("account not found")
	}
	var astr []string
	for _, account := range accounts {
		astr = append(astr, account.String())
	}

	return &tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       []interface{}{tgbotapi.NewInlineQueryResultArticle(query.ID, "您的 Friend Code 记录", strings.Join(astr, "\n"))},
		IsPersonal:    true,
	}, nil
}

func cmdWebLogin(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("https://%s.appspot.com/login", _projectID)},
		nil
}

func cmdListFriendCodes(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询时出错",
		}
		return
	}
	var rst []string
	for _, u := range users {
		var chatmember tgbotapi.ChatMember
		chatmember, err = tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: message.Chat.ID, UserID: u.ID})
		if err != nil || !(chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator()) {
			continue
		}
		accounts, err := u.GetAccounts(ctx)
		if err != nil {
			continue
		}
		userinfo := u.Name
		for _, a := range accounts {
			userinfo += fmt.Sprintf("\n\t%s", a.String())
		}
		rst = append(rst, userinfo)
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(rst, "\n")},
		nil
}
