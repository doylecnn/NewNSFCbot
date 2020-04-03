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
	"github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdAddMyIsland(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	argstr := strings.TrimSpace(message.CommandArguments())
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
			ReplyText: "请使用 N 表示北半球，S 表示南半球狸",
		}
	}
	owner := strings.TrimSpace(args[2])
	fruits := args[3:]

	var island *storage.Island
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil {
		if strings.HasPrefix(err.Error(), "Not found userID:") {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("请先添加FC狸。error info: %v", err),
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("添加岛屿时失败狸。error info: %v", err),
		}
	}
	if island, err = u.GetAnimalCrossingIsland(ctx); err != nil {
		return nil, Error{InnerError: err,
			ReplyText: fmt.Sprintf("请先添加FC狸。error info: %v", err),
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
				ReplyText: fmt.Sprintf("更新岛屿信息时出错狸"),
			}
		}
	} else {
		island = &storage.Island{Name: islandName, Hemisphere: hemisphere, AirportIsOpen: false, AirportPassword: "", Fruits: fruits, Owner: owner}
		if err = u.AddAnimalCrossingIsland(ctx, *island); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("记录岛屿时出错狸"),
			}
		}
	}

	if strings.HasSuffix(islandName, "岛") {
		islandName += "岛"
	}

	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("完成狸。添加了岛屿 %s 的信息狸。", islandName)}},
		nil
}

func cmdMyIsland(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil && !strings.HasPrefix(err.Error(), "Not found userID:") {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if err != nil && strings.HasPrefix(err.Error(), "Not found userID:") {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"}},
			nil
	}
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdOpenIsland(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	password := strings.TrimSpace(message.CommandArguments())

	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil && !strings.HasPrefix(err.Error(), "Not found userID:") {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if u == nil && strings.HasPrefix(err.Error(), "Not found userID:") {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	logrus.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	if !island.AirportIsOpen || island.AirportPassword != password {
		island.AirportIsOpen = true
		island.AirportPassword = password
		u.SetAirportStatus(ctx, *island)
	}
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdCloseIsland(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil && !strings.HasPrefix(err.Error(), "Not found userID:") {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if err != nil && strings.HasPrefix(err.Error(), "Not found userID:") {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	logrus.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	if island.AirportIsOpen {
		island.AirportIsOpen = false
		island.AirportPassword = ""
		u.SetAirportStatus(ctx, *island)
	}
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdListIslands(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://tgbot-ns-fc-wed-18-mar-2020.appspot.com/islands"}},
		nil
}

func cmdDTCPriceUpdate(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	pricestr := strings.TrimSpace(message.CommandArguments())
	if len(pricestr) == 0 {
		return cmdDTCMaxPriceInGroup(message)
	}

	price, err := strconv.ParseInt(pricestr, 10, 64)
	if err != nil || price < 1 || price > 999 {
		return nil, Error{InnerError: err,
			ReplyText: "只接受[1-999]之间的正整数报价狸",
		}
	}

	uid := message.From.ID

	ctx := context.Background()
	err = storage.UpdateDTCPrice(ctx, uid, int(price))
	if err != nil {
		if "Not found game: animal_crossing" == err.Error() {
			return nil, Error{InnerError: err,
				ReplyText: "请先登记你的岛屿狸",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "更新报价时出错狸",
		}
	}
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新大头菜报价成功狸"}},
		nil
}

func cmdDTCMaxPriceInGroup(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	users, err := storage.GetGroupUsers(ctx, message.Chat.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
	}

	var priceUsers []*storage.User
	for _, u := range users {
		island, err := u.GetAnimalCrossingIsland(ctx)
		if err != nil || island == nil {
			continue
		}
		if time.Since(island.LastPrice.Date) > 12*time.Hour {
			continue
		}
		if !strings.HasSuffix(island.Name, "岛") {
			island.Name += "岛"
		}
		u.Island = *island
		priceUsers = append(priceUsers, u)
	}

	if len(priceUsers) == 0 {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "本群最近12小时内没有有效的报价狸"}},
			nil
	}
	sort.Slice(priceUsers, func(i, j int) bool {
		return priceUsers[i].Island.LastPrice.Price > priceUsers[j].Island.LastPrice.Price
	})

	var top10Price []*storage.User
	if len(priceUsers) > 10 {
		top10Price = priceUsers[:10]
	} else {
		top10Price = priceUsers
	}

	var dtcPrices []string
	for _, u := range top10Price {
		if u != nil {
			dtcPrices = append(dtcPrices, fmt.Sprintf("%s的岛：%s 上的菜价：%d", u.Name, u.Island.Name, u.Island.LastPrice.Price))
		}
	}

	replyText := "今日高价（前十）：\n" + strings.Join(dtcPrices, "\n")

	if len(priceUsers) > 10 {
		var lowestPrice = priceUsers[len(priceUsers)-1]
		dtcLowsetPrices := fmt.Sprintf("%s的岛：%s 上的菜价：%d", lowestPrice.Name, lowestPrice.Island.Name, lowestPrice.Island.LastPrice.Price)
		replyText += "\n今日最低：\n" + dtcLowsetPrices
	}

	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: replyText}},
		nil
}

func cmdWhois(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	query := strings.TrimSpace(message.CommandArguments())
	if len(query) == 0 {
		return
	}
	var groupID int64 = message.Chat.ID
	ctx := context.Background()
	users, err := storage.GetUsersByName(ctx, query, groupID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply}},
			err
	}

	users, err = storage.GetUsersByNSAccountName(ctx, query, groupID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply}},
			err
	}

	users, err = storage.GetUsersByAnimalCrossingIslandName(ctx, query, groupID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply}},
			err
	}

	users, err = storage.GetUsersByAnimalCrossingIslandOwnerName(ctx, query, groupID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询时出错狸",
		}
	}
	if len(users) > 0 {
		reply, err := userInfo(ctx, message.Chat.ID, users)
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: reply}},
			err
	}

	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "没找岛狸"}},
		nil
}

func userInfo(ctx context.Context, chatID int64, users []*storage.User) (replyMessage string, err error) {
	var rst []string
	for _, u := range users {
		rst = append(rst, u.Name)
		for _, a := range u.NSAccounts {
			rst = append(rst, a.String())
		}
	}
	return strings.Join(rst, "\n"), nil
}

func cmdSearchAnimalCrossingInfo(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	args := strings.TrimSpace(message.CommandArguments())
	ctx := context.Background()
	var us []*storage.User
	if message.ReplyToMessage != nil && message.ReplyToMessage.From.ID != message.From.ID {
		groupID := message.Chat.ID
		u, err := storage.GetUser(ctx, message.ReplyToMessage.From.ID, groupID)
		if err != nil {
			if strings.HasPrefix(err.Error(), "Not found userID:") {
				return nil, Error{InnerError: err,
					ReplyText: "没有找岛狸",
				}
			}
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
		if u != nil {
			us = []*storage.User{u}
		}
	} else if len(args) > 1 && strings.HasPrefix(args, "@") && args[1:] != message.From.UserName {
		groupID := message.Chat.ID
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
		logrus.Info("users count == 0")
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到对方的记录狸"}},
			nil
	}

	var user *storage.User
	for _, u := range us {
		chatmember, err := tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: message.Chat.ID, UserID: u.ID})
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
		if chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator() {
			user = u
			break
		}
	}

	if user == nil {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到对方的记录狸"}},
			nil
	}

	island, err := user.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "对方尚未登记过自己的 动森 岛屿狸"}},
			nil
	}

	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdHuaShiJiaoHuanBiaoGe(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://docs.google.com/spreadsheets/d/1ZycWgFx7HGTNR7NkMNFwUz-Oiqr4rtXdtHzQ0qW1HGY/edit?usp=sharing"}},
		nil
}
