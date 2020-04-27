package chatbot

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdAddFC(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
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

	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("创建用户信息时出错: %v", err),
		}
	}
	if err != nil && status.Code(err) == codes.NotFound {
		username := message.From.UserName
		if len(username) == 0 {
			username = strings.TrimSpace(message.From.FirstName + " " + message.From.LastName)
		}
		if len(username) == 0 {
			username = fmt.Sprintf("%d", message.From.ID)
		}
		if !message.Chat.IsPrivate() {
			u = &storage.User{ID: message.From.ID, Name: username, NSAccounts: accounts, GroupIDs: []int64{groupID}}
		} else {
			u = &storage.User{ID: message.From.ID, Name: username, NSAccounts: accounts}
		}
		u.NameInsensitive = strings.ToLower(u.Name)
		if err = u.Set(ctx); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("创建用户信息时出错: %v", err),
			}
		}
	} else {
		var accountNotExists []storage.NSAccount
		var accountNeedUpdate bool = false
		for _, a := range accounts {
			var accountExists = false
			var needEdit = false
			for i, account := range u.NSAccounts {
				if account.FC == a.FC && account.Name == a.Name {
					accountExists = true
					break
				} else if account.FC == a.FC && account.Name != a.Name {
					accountExists = true
					needEdit = true
					u.NSAccounts[i] = a
					break
				}
			}
			if !accountExists || needEdit {
				accountNeedUpdate = true
			}
			if !accountExists && !needEdit {
				accountNotExists = append(accountNotExists, a)
			}
		}
		if accountNeedUpdate {
			if len(accountNotExists) > 0 {
				u.NSAccounts = append(u.NSAccounts, accountNotExists...)
			}
			if err = u.Update(ctx); err != nil {
				return nil, Error{InnerError: err,
					ReplyText: fmt.Sprintf("更新用户信息时出错: %v", err)}
			}
		}
	}

	if message.Chat.IsPrivate() {
		return cmdMyFC(message)
	}

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("完成。添加/更新了 %d 个 Friend Code", len(accounts))}},
		nil
}

func cmdDelFC(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	arg := strings.TrimSpace(message.CommandArguments())
	if len(arg) < 12 {
		return
	}
	var fc storage.FriendCode
	if fcnum, lerr := strconv.ParseInt(arg, 10, 64); lerr == nil {
		fc = storage.FriendCode(fcnum)
	} else {
		return nil, lerr
	}

	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, 0)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("本bot 没有记录您的FC信息: %v", err),
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("执行指令时出错: %v", err),
		}
	}
	for _, a := range u.NSAccounts {
		if a.FC == fc {
			if err = u.DeleteNSAccount(ctx, a); err == nil {
				return []*tgbotapi.MessageConfig{{
						BaseChat: tgbotapi.BaseChat{
							ChatID:              message.Chat.ID,
							ReplyToMessageID:    message.MessageID,
							DisableNotification: true},
						Text: fmt.Sprintf("已删除您的FC：%s。", fc.String())}},
					nil
			}
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("删除FC时出错: %v", err),
			}
		}
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("没有找到FC为 %s 的记录狸。", fc.String())}},
		nil
}

func cmdDeleteMe(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	if u, err := storage.GetUser(ctx, message.From.ID, 0); err != nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("删除信息时出错: %v", err),
		}
	} else if u != nil {
		if err = u.Delete(ctx); err != nil {
			_logger.WithError(err).Error()
		}
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "所有信息已删除狸。如要使用需要从addfc 开始重新登记。"}},
		nil
}

func cmdMyFC(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, 0)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if err != nil && status.Code(err) == codes.NotFound {
		_logger.Debug("没有找到用户记录")
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addfc 命令添加记录"}},
			nil
	}
	if len(u.NSAccounts) == 0 {
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addfc 命令添加记录"}},
			nil
	}

	if message.Chat.IsPrivate() {
		var rows [][]tgbotapi.InlineKeyboardButton
		for i, account := range u.NSAccounts {
			var manageFCBtn = tgbotapi.NewInlineKeyboardButtonData(account.String(), fmt.Sprintf("/manageFriendCodes_%d_%d", message.From.ID, i))
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(manageFCBtn))
		}
		var cancelBtn = tgbotapi.NewInlineKeyboardButtonData("取消", "/cancel")
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(cancelBtn))
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      message.Chat.ID,
				ReplyMarkup: replyMarkup,
			},
			Text: "请点击要管理的 Friend Code\n/addfc [id]:[FC] 添加新 Friend Code"}}, nil
	}
	var astr []string
	for _, account := range u.NSAccounts {
		astr = append(astr, account.String())
	}
	var manageFCBtn = tgbotapi.NewInlineKeyboardButtonData("管理 Friend Code", "/manageFriendCodes")
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(manageFCBtn))
	var replyText = fmt.Sprintf("您的 Friend Code：\n%s\n您可私聊 bot 管理您的 FriendCode", strings.Join(astr, "\n"))
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text: replyText,
		}},
		nil
}

func cmdSearchFC(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	args := strings.TrimSpace(message.CommandArguments())
	ctx := context.Background()
	var us []*storage.User
	if message.ReplyToMessage != nil {
		if message.ReplyToMessage.From.ID == message.From.ID {
			return cmdMyFC(message)
		}
		var groupID int64 = message.Chat.ID
		u, err := storage.GetUser(ctx, message.ReplyToMessage.From.ID, groupID)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				return nil, Error{InnerError: err,
					ReplyText: "没有找岛（到）这位用户的信息狸",
				}
			}
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
		if u != nil {
			us = []*storage.User{u}
		}
	} else if len(args) > 1 && strings.HasPrefix(args, "@") {
		if args[1:] == message.From.UserName {
			return cmdMyFC(message)
		}
		var groupID int64 = message.Chat.ID
		us, err = storage.GetUsersByName(ctx, args[1:], groupID)
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
	} else {
		return nil, errors.New("not reply to any message or not at anyone")
	}

	if len(us) == 0 {
		_logger.Info("users count == 0")
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找岛（到）对方的记录狸"}},
			nil
	}
	u := us[0]
	var astr []string
	for _, account := range u.NSAccounts {
		astr = append(astr, account.String())
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(astr, "\n")}},
		nil
}

func inlineQueryMyFC(query *tgbotapi.InlineQuery) (*tgbotapi.InlineConfig, error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, query.From.ID, 0)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, err
	}
	if err != nil && status.Code(err) == codes.NotFound {
		return nil, errors.New("user not found")
	}
	var astr []string
	for _, account := range u.NSAccounts {
		astr = append(astr, account.String())
	}

	return &tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       []interface{}{tgbotapi.NewInlineQueryResultArticle(query.ID, "您的 Friend Code 记录", strings.Join(astr, "\n"))},
		IsPersonal:    true,
	}, nil
}

func inlineQueryMyIsland(query *tgbotapi.InlineQuery) (*tgbotapi.InlineConfig, error) {
	ctx := context.Background()
	u, err := storage.GetUser(ctx, query.From.ID, 0)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, nil
	}
	if err != nil && status.Code(err) == codes.NotFound {
		return &tgbotapi.InlineConfig{
			InlineQueryID: query.ID,
			Results:       []interface{}{tgbotapi.NewInlineQueryResultArticle(query.ID, "您没有记录过您的 Friend Code", "请先使 addFC 登记，再用 addisland 命令添加岛屿")},
			IsPersonal:    true,
		}, nil
	}
	island, _, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		_logger.WithError(err).Info("inlineQueryMyIsland GetAnimalCrossingIsland")
		return nil, nil
	}
	if island == nil {
		return &tgbotapi.InlineConfig{
			InlineQueryID: query.ID,
			Results:       []interface{}{tgbotapi.NewInlineQueryResultArticle(query.ID, "您没有记录过您的 Friend Code", "请先使 addFC 登记，再用 addisland 命令添加岛屿")},
			IsPersonal:    true,
		}, nil
	}
	return &tgbotapi.InlineConfig{
		InlineQueryID: query.ID,
		Results:       []interface{}{tgbotapi.NewInlineQueryResultArticle(query.ID, "您的 岛屿 记录", island.String())},
		IsPersonal:    true,
	}, nil
}

func cmdWebLogin(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("http://%s/login", _domain)}},
		nil
}

func cmdListFriendCodes(message *tgbotapi.Message) (replyMessages []*tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	users, err := storage.GetGroupUsers(ctx, message.Chat.ID)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
		return
	}
	var rst []string
	var i = 0
	for _, u := range users {
		if len(u.NSAccounts) == 0 {
			continue
		}
		if len(u.NSAccounts) == 1 {
			a := u.NSAccounts[0]
			if len(a.Name) == 0 {
				rst = append(rst, u.Name+a.String())
			} else {
				rst = append(rst, a.String())
			}
		} else {
			userinfo := fmt.Sprintf("%s:", u.Name)
			for _, a := range u.NSAccounts {
				userinfo += fmt.Sprintf("\n +- %s", a.String())
			}
			rst = append(rst, userinfo)
		}
		i++
		if i != 0 && i%50 == 0 {
			replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: strings.Join(rst, "\n")})
			rst = rst[:0]
		}
	}
	if len(rst) > 0 {
		replyMessages = append(replyMessages, &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(rst, "\n")})
		rst = rst[:0]
	}

	return replyMessages, nil
}
