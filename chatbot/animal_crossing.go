package chatbot

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	log "github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdAddMyIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	argstr := strings.TrimSpace(message.CommandArguments())
	if len(argstr) <= 1 {
		return
	}
	var username = message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName + " " + message.From.LastName
	}
	args := strings.Split(argstr, " ")
	if len(args) < 3 {
		return nil, Error{InnerError: err,
			ReplyText: `/addisland 详细语法：
/addisland 命令至少需要3个参数，第一个是岛的名字，第二个是南北半球，第三个是岛主，所有参数使用空格分割
南北半球请使用 N 或 S 表示：N 表示北半球，S 表示南半球`,
		}
	}
	islandName := strings.TrimSpace(args[0])
	var hemisphere int
	if args[1] == "N" {
		hemisphere = 0
	} else if args[1] == "S" {
		hemisphere = 1
	} else {
		return nil, Error{InnerError: err,
			ReplyText: "请使用 N 表示北半球，S 表示南半球",
		}
	}
	owner := strings.TrimSpace(args[2])
	fruits := args[3:]

	var island *storage.Island
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID)
	if err != nil || u == nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("请先添加FC。error info: %v", err),
		}
	}
	if island, err = u.GetAnimalCrossingIsland(ctx); err != nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("请先添加FC。error info: %v", err),
		}
	} else if err == nil && island != nil {
		island, err = u.GetAnimalCrossingIsland(ctx)
		if err != nil {
			return
		}
		island.Name = islandName
		island.Hemisphere = hemisphere
		island.Fruits = fruits
		island.Owner = owner
		if err = u.SetAirportStatus(ctx, *island); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("更新岛屿信息时出错"),
			}
		}
	} else {
		island = &storage.Island{Name: islandName, Hemisphere: hemisphere, AirportIsOpen: false, AirportPassword: "", Fruits: fruits, Owner: owner}
		if err = u.AddAnimalCrossingIsland(ctx, *island); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("记录岛屿时出错"),
			}
		}
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("完成。添加了岛屿 %s 的信息。", islandName)},
		nil
}

func cmdMyIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
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
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdOpenIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	password := strings.TrimSpace(message.CommandArguments())

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
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	if !island.AirportIsOpen || island.AirportPassword != password {
		island.AirportIsOpen = true
		island.AirportPassword = password
		u.SetAirportStatus(ctx, *island)
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdCloseIsland(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
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
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	log.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"},
			nil
	}
	if island.AirportIsOpen {
		island.AirportIsOpen = false
		island.AirportPassword = ""
		u.SetAirportStatus(ctx, *island)
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}

func cmdListIslands(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://tgbot-ns-fc-wed-18-mar-2020.appspot.com/islands"},
		nil
}

func cmdDTCPriceUpdate(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	pricestr := strings.TrimSpace(message.CommandArguments())
	if len(pricestr) == 0 {
		return cmdDTCMaxPriceInGroup(message)
	}

	price, err := strconv.ParseInt(pricestr, 10, 64)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "报价只能是数字",
		}
	}

	uid := message.From.ID

	ctx := context.Background()
	err = storage.UpdateDTCPrice(ctx, uid, int(price))
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "更新报价时出错",
		}
	}
	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新大头菜报价成功"},
		nil
}

func cmdDTCMaxPriceInGroup(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}

	var priceUsers []*storage.User
	for _, user := range users {
		island, err := user.GetAnimalCrossingIsland(ctx)
		if err != nil || island == nil {
			continue
		}
		if time.Since(island.LastPrice.Date) > 12*time.Hour {
			continue
		}
		chatmember, err := tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: message.Chat.ID, UserID: user.ID})
		if err != nil || !(chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator()) {
			continue
		}
		if !strings.HasSuffix(island.Name, "岛") {
			island.Name += "岛"
		}
		user.Island = *island
		priceUsers = append(priceUsers, user)
	}

	var top10Price []*storage.User
	sort.Slice(priceUsers, func(i, j int) bool {
		return priceUsers[i].Island.LastPrice.Price > priceUsers[j].Island.LastPrice.Price
	})
	top10Price = priceUsers[:10]

	var dtcPrices []string
	for _, u := range top10Price {
		dtcPrices = append(dtcPrices, fmt.Sprintf("%s的岛：%s 上的菜价：%d", u.Name, u.Island.Name, u.Island.LastPrice.Price))
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: strings.Join(dtcPrices, "\n")},
		nil
}

func cmdWhois(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
	query := strings.TrimSpace(message.CommandArguments())
	if len(query) == 0 {
		return
	}
	ctx := context.Background()
	users, err := storage.GetUsersByName(ctx, query)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply},
			err
	}

	users, err = storage.GetUsersByNSAccountName(ctx, query)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply},
			err
	}

	users, err = storage.GetUsersByAnimalCrossingIslandName(ctx, query)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply},
			err
	}

	users, err = storage.GetUsersByAnimalCrossingIslandOwnerName(ctx, query)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply},
			err
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "没找到。"},
		nil
}

func userInfo(ctx context.Context, chatID int64, users []*storage.User) (replyMessage string, err error) {
	var rst []string
	for _, u := range users {
		var chatmember tgbotapi.ChatMember
		chatmember, err = tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: chatID, UserID: u.ID})
		if err != nil || !(chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator()) {
			continue
		}
		rst = append(rst, u.Name)
		var accounts []storage.NSAccount
		accounts, err = u.GetAccounts(ctx)
		if err != nil {
			err = Error{InnerError: err,
				ReplyText: "查询时出错",
			}
			return
		}
		for _, a := range accounts {
			rst = append(rst, a.String())
		}
	}
	return strings.Join(rst, "\n"), nil
}

func cmdSearchAnimalCrossingInfo(message *tgbotapi.Message) (replyMessage *tgbotapi.MessageConfig, err error) {
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

	island, err := user.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了",
		}
	}
	if island == nil {
		return &tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "对方尚未登记过自己的 动森 岛屿"},
			nil
	}

	return &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()},
		nil
}
