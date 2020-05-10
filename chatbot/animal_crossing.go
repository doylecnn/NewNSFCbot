package chatbot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdAddMyIsland(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	argstr := strings.TrimSpace(message.CommandArguments())
	var username = message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName + " " + message.From.LastName
	}
	args := strings.Split(argstr, " ")
	if len(args) < 3 {
		return nil, Error{InnerError: errors.New("args length less 3"),
			ReplyText: `/addisland 详细语法：
/addisland 命令至少需要3个参数，第一个是岛的名字，第二个是南北半球，第三个是岛主，其它内容将作为岛屿的基本信息。所有参数使用空格分割
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
		return nil, Error{InnerError: errors.New("args wrong"),
			ReplyText: "请使用 N 表示北半球，S 表示南半球狸",
		}
	}
	owner := strings.TrimSpace(args[2])
	baseinfo := strings.Join(args[3:], ", ")

	var FCNotExists bool = false
	var island *storage.Island
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil {
		if status.Code(err) != codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("添加岛屿时失败狸。error info: %v", err),
			}
		}
		var username = message.From.UserName
		if len(username) == 0 {
			username = message.From.FirstName + " " + message.From.LastName
		}
		if groupID != 0 {
			u = storage.User{
				ID:              message.From.ID,
				Name:            username,
				NameInsensitive: strings.ToLower(username),
				GroupIDs:        []int64{groupID},
			}
		} else {
			u = storage.User{
				ID:              message.From.ID,
				Name:            username,
				NameInsensitive: strings.ToLower(username),
			}
		}
		FCNotExists = true
		if err = u.Set(ctx); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("添加岛屿时失败狸。error info: %v", err),
			}
		}
	}
	if FCNotExists {
		island = &storage.Island{
			Path:             fmt.Sprintf("users/%d/games/animal_crossing", u.ID),
			Name:             islandName,
			NameInsensitive:  strings.ToLower(islandName),
			Hemisphere:       hemisphere,
			AirportIsOpen:    false,
			BaseInfo:         baseinfo,
			Info:             "",
			Timezone:         storage.Timezone(8 * 3600),
			Owner:            owner,
			OwnerInsensitive: strings.ToLower(owner)}
		if err = island.Update(ctx); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("记录岛屿时出错狸。error info: %v", err),
			}
		}
	} else {
		if island, _, err = u.GetAnimalCrossingIsland(ctx); err != nil && status.Code(err) != codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("添加岛屿时失败狸。error info: %v", err),
			}
		} else if err == nil && island != nil {
			island.Name = islandName
			island.NameInsensitive = strings.ToLower(islandName)
			island.Hemisphere = hemisphere
			island.BaseInfo = baseinfo
			island.Owner = owner
			island.OwnerInsensitive = strings.ToLower(owner)
			if err = island.Update(ctx); err != nil {
				return nil, Error{InnerError: err,
					ReplyText: fmt.Sprintf("更新岛屿信息时出错狸。error info: %v", err),
				}
			}
		} else {
			island = &storage.Island{
				Path:             fmt.Sprintf("users/%d/games/animal_crossing", u.ID),
				Name:             islandName,
				NameInsensitive:  strings.ToLower(islandName),
				Hemisphere:       hemisphere,
				AirportIsOpen:    false,
				BaseInfo:         baseinfo,
				Info:             "",
				Timezone:         storage.Timezone(8 * 3600),
				Owner:            owner,
				OwnerInsensitive: strings.ToLower(owner)}
			if err = island.Update(ctx); err != nil {
				return nil, Error{InnerError: err,
					ReplyText: fmt.Sprintf("记录岛屿时出错狸。error info: %v", err),
				}
			}
		}
	}

	if !strings.HasSuffix(islandName, "岛") {
		islandName += "岛"
	}

	var rstText = fmt.Sprintf("完成狸。添加了岛屿 %s 的信息狸。", islandName)
	if FCNotExists {
		rstText += "但您还没有登记您的FC。\n将来可使用/addfc 命令登记，方便群友通过FC 添加您为好友狸。"
	}
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: rstText}},
		nil
}

func cmdUpdateIslandBaseInfo(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	if len(args) == 0 {
		return
	}
	ctx := context.Background()
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"}},
			nil
	}
	if island.BaseInfo != args {
		island.BaseInfo = args
		if err = island.Update(ctx); err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "更新岛屿信息是时出错了狸",
			}
		}
	}
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新了岛屿信息：\n" + island.BaseInfo}},
		nil
}

func cmdSetIslandTimezone(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	uid := message.From.ID
	args := strings.TrimSpace(message.CommandArguments())
	if len(args) == 0 {
		return
	}
	hm, err := strconv.Atoi(args)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "请输入正确的时间范围：[UTC-1200, UTC+1400]\n默认时区为+0800",
		}
	}
	if hm > 1400 || hm < -1200 {
		return nil, Error{InnerError: nil,
			ReplyText: "请输入正确的时间范围：[UTC-1200, UTC+1400]\n默认时区为+0800",
		}
	}
	hours := hm / 100
	minutes := hm % 100
	if math.Abs(float64(minutes)) >= 60 {
		return nil, Error{InnerError: nil,
			ReplyText: "请输入正确的时间范围：[UTC-1200, UTC+1400]\n默认时区为+0800",
		}
	}
	timezone := storage.Timezone(hours*60*60 + minutes*60)

	ctx := context.Background()
	island, residentUID, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return nil, Error{InnerError: err,
			ReplyText: "更新时区时出错狸",
		}
	}
	if residentUID > 0 {
		uid = residentUID
	}
	oldtimezone := island.Timezone
	island.Timezone = timezone
	if err = island.Update(ctx); err != nil {
		_logger.Error().Err(err).Msg("更新时区时出错狸")
		return nil, Error{InnerError: err,
			ReplyText: "更新时区时出错狸",
		}
	}
	//updateWeekPrice
	var nowLoc = time.Now().In(oldtimezone.Location())
	if nowLoc.Hour() < 5 {
		nowLoc = nowLoc.Add(-(time.Duration(nowLoc.Hour()+1) * time.Hour))
	}
	var weekStartDateLoc = nowLoc.AddDate(0, 0, 0-int(nowLoc.Weekday()))
	weekStartDateLoc = time.Date(weekStartDateLoc.Year(), weekStartDateLoc.Month(), weekStartDateLoc.Day(), 0, 0, 0, 0, oldtimezone.Location())
	var weekStartDate = weekStartDateLoc.UTC()
	var weekEndDate = weekStartDate.AddDate(0, 0, 7)
	weekStartDate = weekStartDate.Add(5 * time.Hour)
	oldPriceHistory, err := storage.GetWeeklyDTCPriceHistory(ctx, uid, weekStartDate, weekEndDate)

	var weekPrices []string = make([]string, 13)
	for _, p := range oldPriceHistory {
		var j = int(p.LocationDateTime().Weekday())
		if j == 0 {
			weekPrices[j] = strconv.Itoa(p.Price)
		} else if p.LocationDateTime().Hour() < 12 {
			weekPrices[j*2-1] = strconv.Itoa(p.Price)
		} else {
			weekPrices[j*2] = strconv.Itoa(p.Price)
		}
	}

	weekpriceStr := strings.TrimFunc(strings.Join(weekPrices, ","), func(r rune) bool { return r == ' ' || r == ',' })
	_, err = getWeeklyDTCPriceHistory(ctx, message, uid, weekpriceStr)
	if err != nil {
		_logger.Error().Err(err).Str("weekprice", weekpriceStr).Msg("updateweekprice error")
	}

	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("更新了岛屿时区：\n%s\n由于改动了时区，请使用 /weekprice 检查周报价是否正确。", island.Timezone.String())}},
		nil
}

func cmdMyIsland(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"}},
			nil
	}
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdOpenIsland(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	islandInfo := strings.TrimSpace(message.CommandArguments())

	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您没有登记过岛屿狸。请使用/addisland 添加岛屿。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	_logger.Debug().Str("user", u.Name).Send()
	island, _, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您没有登记过岛屿狸。请使用/addisland 添加岛屿。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
				},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸",
			}},
			nil
	}
	if !island.AirportIsOpen || island.Info != islandInfo {
		island.AirportIsOpen = true
		island.Info = islandInfo
		island.OpenTime = time.Now()
		island.Update(ctx)
	}
	var btn = tgbotapi.NewInlineKeyboardButtonData("点此创建新队列", "/queue")
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text: island.String()}},
		nil
}

func cmdCloseIsland(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	var groupID int64 = 0
	if !message.Chat.IsPrivate() {
		groupID = message.Chat.ID
	}
	ctx := context.Background()
	u, err := storage.GetUser(ctx, message.From.ID, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if err != nil && status.Code(err) == codes.NotFound {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	island, _, err := u.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	if island.AirportIsOpen {
		island.AirportIsOpen = false
		island.Info = ""
		if len(island.OnBoardQueueID) > 0 {
			queue, lerr := island.ClearOldOnboardQueue(ctx)
			if lerr != nil {
				return []tgbotapi.MessageConfig{
					{BaseChat: tgbotapi.BaseChat{
						ChatID:              int64(message.From.ID),
						DisableNotification: false},
						Text: "关闭岛屿时，清理队列时，发生错误。已通知bot 管理员。"},
					{BaseChat: tgbotapi.BaseChat{
						ChatID:              int64(botAdminID),
						DisableNotification: false},
						Text: "关闭岛屿时，清理队列时，发生错误。已通知bot 管理员。" + err.Error()},
				}, nil
			}
			replyMessage = append(replyMessage, notifyQueueDissmised(queue)...)
		}
		island.Update(ctx)
	}
	replyMessage = append(replyMessage, tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: true},
		Text: island.String()})
	return
}

func cmdListIslands(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("https://%s/islands", _domain)}},
		nil
}

func cmdDTCPriceUpdate(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	pricestr := strings.TrimSpace(message.CommandArguments())
	uid := message.From.ID
	if message.From.ID == botAdminID && message.Chat.IsPrivate() {
		if strings.HasPrefix(pricestr, "#") {
			args := strings.SplitN(pricestr, " ", 2)
			uid, err = strconv.Atoi(args[0][1:])
			if err != nil {
				return
			}
			if len(args) > 1 {
				pricestr = args[1]
			} else {
				pricestr = ""
			}
		}
	}
	if len(pricestr) == 0 {
		return cmdDTCMaxPriceInGroup(message)
	}

	price, err := strconv.ParseInt(pricestr, 10, 64)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "请检查参数，参数必须是数字狸",
		}
	}
	if price < 1 || price > 999 {
		return nil, Error{InnerError: err,
			ReplyText: "只接受[1-999]之间的正整数报价狸",
		}
	}

	ctx := context.Background()
	err = storage.UpdateDTCPrice(ctx, uid, int(price))
	if err != nil {
		_logger.Error().Err(err).Msg("update island last price")
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "请先登记你的岛屿狸。\n本bot 原本是为交换Nintendo Switch Friend Code而生。\n所以建议先/addfc 登记fc，再/addisland 登记岛屿，再/dtcj 发布价格。\n具体命令帮助请/help",
			}
		}
		if err.Error() == "buy price out of range" {
			return nil, Error{InnerError: err,
				ReplyText: "周日进价的范围应该是[90, 110]狸。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "更新报价时出错狸",
		}
	}
	return getWeeklyDTCPriceHistory(ctx, message, uid, "")
}

// cmdDTCWeekPriceAndPredict 当周菜价回看/预测
func cmdDTCWeekPriceAndPredict(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	argstr := strings.TrimSpace(message.CommandArguments())
	uid := message.From.ID
	if message.From.ID == botAdminID && message.Chat.IsPrivate() && strings.HasPrefix(argstr, "#") {
		args := strings.SplitN(argstr, " ", 2)
		uid, err = strconv.Atoi(args[0][1:])
		if err != nil {
			uid = message.From.ID
			argstr = ""
		} else {
			if len(args) > 1 {
				argstr = args[1]
			} else {
				argstr = ""
			}
		}
	}
	ctx := context.Background()
	return getWeeklyDTCPriceHistory(ctx, message, uid, argstr)
}

func getWeeklyDTCPriceHistory(ctx context.Context, message *tgbotapi.Message, uid int, argstr string) (replyMessage []tgbotapi.MessageConfig, err error) {
	island, residentUID, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "请先登记你的岛屿狸。\n本bot 原本是为交换Nintendo Switch Friend Code而生。\n所以建议先/addfc 登记fc，再/addisland 登记岛屿，再/dtcj 发布价格。\n也可以先不添加FC 只添加岛屿信息。\n具体命令帮助请/help",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查找您的岛屿信息时出错狸",
		}
	}
	if residentUID > 0 {
		uid = residentUID
	}
	var prices []storage.TurnipPrice
	var nowLoc = time.Now().In(island.Timezone.Location())
	if nowLoc.Hour() < 5 {
		nowLoc = nowLoc.Add(-(time.Duration(nowLoc.Hour()+1) * time.Hour))
	}
	var weekStartDateLoc = nowLoc.AddDate(0, 0, 0-int(nowLoc.Weekday()))
	weekStartDateLoc = time.Date(weekStartDateLoc.Year(), weekStartDateLoc.Month(), weekStartDateLoc.Day(), 0, 0, 0, 0, island.Timezone.Location())
	var weekStartDate = weekStartDateLoc.UTC()
	var weekEndDate = weekStartDate.AddDate(0, 0, 7)
	if len(argstr) != 0 {
		prices, err = makeWeeklyPrice(argstr, island.Timezone, weekStartDate, weekEndDate)
		if err != nil {
			if err.Error() == "buy price out of range" {
				return nil, Error{InnerError: err,
					ReplyText: "更新一周报价时出错狸。周日买入价格取值范围在[90, 110]",
				}
			}
			return nil, Error{InnerError: err,
				ReplyText: "更新一周报价时出错狸。请从周日进价开始，依次输入每一轮价格，逗号分割。\n举例：\n/weekprice 90,100,150……\n价格范围[1, 999]",
			}
		}
	}
	priceHistory, err := storage.GetWeeklyDTCPriceHistory(ctx, uid, weekStartDate, weekEndDate)
	if err != nil {
		_logger.Error().Err(err).Msg("GetWeeklyDTCPriceHistory")
		return nil, Error{InnerError: err,
			ReplyText: "查找报价信息时出错狸",
		}
	}
	if len(prices) > 0 {
		if priceHistory != nil && len(priceHistory) > 0 {
			client, err := firestore.NewClient(ctx, _projectID)
			if err != nil {
				_logger.Error().Err(err).Msg("cmdDTCWeekPriceAndPredict NewClient")
				return nil, Error{InnerError: err,
					ReplyText: fmt.Sprintf("保存一周报价时出错狸：%v", err),
				}
			}
			defer client.Close()
			col := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid))
			if err = storage.DeleteCollection(ctx, client, col, 10); err != nil {
				_logger.Error().Err(err).Msg("cmdDTCWeekPriceAndPredict")
				return nil, Error{InnerError: err,
					ReplyText: fmt.Sprintf("保存一周报价时出错狸：%v", err),
				}
			}
			priceHistory = priceHistory[0:0:0]
		}
		priceHistory = append(priceHistory, prices...)

		client, err := firestore.NewClient(ctx, _projectID)
		if err != nil {
			_logger.Error().Err(err).Msg("set price history")
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("保存一周报价时出错狸：%v", err),
			}
		}
		defer client.Close()

		batch := client.Batch()
		var ph storage.TurnipPrice
		for _, ph := range priceHistory {
			docRef := client.Collection(fmt.Sprintf("users/%d/games/animal_crossing/price_history", uid)).Doc(fmt.Sprintf("%d", ph.Date.Unix()))
			batch.Set(docRef, ph)
		}
		island.LastPrice = ph
		islandRef := client.Doc(island.Path)
		batch.Update(islandRef, []firestore.Update{{Path: "LastPrice", Value: ph}})
		if _, err = batch.Commit(ctx); err != nil {
			_logger.Error().Err(err).Msg("set price history")
			return nil, Error{InnerError: err,
				ReplyText: fmt.Sprintf("保存一周报价时出错狸：%v", err),
			}
		}
	}

	replyText, err := formatWeekPrices(priceHistory)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "格式化一周报价时出错",
		}
	}
	locNow := time.Now().In(island.Timezone.Location())
	formatedNow := markdownSafe(locNow.Format(time.RFC1123Z))
	replyText = fmt.Sprintf("您的岛上时间：%s\n", formatedNow) + replyText
	if message.Chat.IsPrivate() {
		replyMessage = []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: false,
			},
			Text:                  replyText,
			ParseMode:             "MarkdownV2",
			DisableWebPagePreview: false,
		}}
		return
	}
	_, _, changed, err := getTopPriceUsersAndLowestPriceUser(ctx, message.Chat.ID, locNow)
	if err != nil {
		_logger.Warn().Err(err).Send()
		err = nil
		replyMessage = []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: false,
			},
			Text:                  replyText,
			ParseMode:             "MarkdownV2",
			DisableWebPagePreview: false,
		}}
		return
	}
	if !changed {
		replyText = "*您本次的报价对高价排行无影响*\n" + replyText
	} else {
		replyText = "*您本次的报价对高价排行有影响，请使用指令 /gj 查看*\n" + replyText
	}
	replyMessage = []tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: false,
		},
		Text:                  replyText,
		ParseMode:             "MarkdownV2",
		DisableWebPagePreview: false,
	}}
	return replyMessage, nil
}

func makeWeeklyPrice(args string, islandTimezone storage.Timezone, startDate, endDate time.Time) (priceHistory []storage.TurnipPrice, err error) {
	prices := strings.Split(strings.Trim(args, ","), ",")
	if len(prices) < 1 || len(prices) > 13 {
		return nil, errors.New("wrong format")
	}
	var intPrice []int
	for _, tp := range prices {
		ip, err := strconv.Atoi(strings.TrimSpace(tp))
		if ip < 0 || ip > 999 || err != nil {
			return nil, errors.New("wrong format")
		}
		intPrice = append(intPrice, ip)
	}
	for i := 0; i < len(intPrice); i++ {
		if i == 0 {
			if intPrice[i] > 0 {
				if intPrice[i] < 90 && intPrice[i] > 110 {
					return nil, errors.New("buy price out of range")
				}
				priceHistory = append(priceHistory, storage.TurnipPrice{Date: startDate.Add(5 * time.Hour), Price: intPrice[i], Timezone: islandTimezone})
			}
			startDate = startDate.AddDate(0, 0, 1)
		} else {
			if i%2 == 1 {
				startDate = startDate.Add(8 * time.Hour)
				if intPrice[i] > 0 {
					priceHistory = append(priceHistory, storage.TurnipPrice{Date: startDate, Price: intPrice[i], Timezone: islandTimezone})
				}
			} else {
				startDate = startDate.Add(4 * time.Hour)
				if intPrice[i] > 0 {
					priceHistory = append(priceHistory, storage.TurnipPrice{Date: startDate, Price: intPrice[i], Timezone: islandTimezone})
				}
				startDate = startDate.Add(12 * time.Hour)
			}
		}
	}
	return
}

/*	本周您的报价如下: 点我 看大图，或 点我 看表格
 *	| Sun | Mon | Tue | Wed | Thu | Fri | Sat |
 *	| - | -/105 | -/- | -/- | -/- | -/- | -/- |
 *	未录入星期日数据 无法生成查询数据 */
func formatWeekPrices(priceHistory []storage.TurnipPrice) (text string, err error) {
	var weekPrices []string = weekPriceStrings(priceHistory)
	var datePrice []string = make([]string, 7)
	datePrice[0] = weekPrices[0]
	for i := 1; i < 13; i += 2 {
		datePrice[(i+1)/2] = fmt.Sprintf("%s/%s", weekPrices[i], weekPrices[i+1])
	}
	urlpath1 := strings.TrimRight(strings.Join(weekPrices, "\\-"), ",\\-")
	urlpath2 := strings.Join(weekPrices, "\\.")
	return fmt.Sprintf("本周您的[报价](https://ac-turnip.com/p-%s.png)如下: [点我](https://ac-turnip.com/share?f=%s) 看大图，或 [点我](https://turnipprophet.io/?prices=%s) 看表格\n"+
		"\\| Sun \\| Mon \\| Tue \\| Wed \\| Thu \\| Fri \\| Sat \\|\n"+
		"\\| %s \\|", urlpath1, urlpath1, urlpath2, strings.Join(datePrice, " \\| ")), nil
}

func formatWeekPricesURL(priceHistory []storage.TurnipPrice) (text string) {
	return fmt.Sprintf("https://ac-turnip.com/share?f=%s", strings.TrimRight(strings.Join(weekPriceStrings(priceHistory), "-"), ",-"))
}

func weekPriceStrings(priceHistory []storage.TurnipPrice) (weekPrices []string) {
	weekPrices = make([]string, 13)
	for _, p := range priceHistory {
		var j = int(p.LocationDateTime().Weekday())
		if j == 0 {
			weekPrices[j] = strconv.Itoa(p.Price)
		} else if p.LocationDateTime().Hour() < 12 {
			weekPrices[j*2-1] = strconv.Itoa(p.Price)
		} else {
			weekPrices[j*2] = strconv.Itoa(p.Price)
		}
	}
	return
}

func cmdDTCMaxPriceInGroup(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() && message.From.ID != botAdminID {
		return
	}
	chatid := message.Chat.ID
	uid := message.From.ID
	localtime := time.Now()
	ctx := context.Background()
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
	if err != nil {
		localtime = localtime.In(time.FixedZone("+0800", 8*3600))
	} else {
		localtime = localtime.In(island.Timezone.Location())
	}
	if message.From.ID == botAdminID && message.Chat.IsPrivate() && strings.HasPrefix(message.CommandArguments(), "#") {
		argstr := message.CommandArguments()
		args := strings.SplitN(argstr, " ", 2)
		chatid, err = strconv.ParseInt(args[0][1:], 10, 64)
		if err != nil {
			chatid = message.Chat.ID
		} else {
			if len(args) > 1 {
				localtime, err = time.Parse("2006-01-02 15:04:05", args[1])
				if err != nil {
					_logger.Debug().Err(err).Str("input date", args[1]).Msg("error")
				}
			}
		}
	}
	topPriceUsers, lowestPriceUsers, _, err := getTopPriceUsersAndLowestPriceUser(ctx, chatid, localtime)
	if err != nil {
		if err.Error() == "NoValidPrice" {
			return []tgbotapi.MessageConfig{{
					BaseChat: tgbotapi.BaseChat{
						ChatID:              message.Chat.ID,
						ReplyToMessageID:    message.MessageID,
						DisableNotification: true},
					Text: "本群最近12小时内没有有效的报价狸"}},
				nil
		}
		return
	}
	var replyText string
	if localtime.Weekday() == 0 {
		var lowestDTCPrices []string
		for i, u := range lowestPriceUsers {
			lowestDTCPrices = append(lowestDTCPrices, formatIslandDTCPrice(u, i+1))
		}
		replyText = fmt.Sprintf("*今日低进价（前 %d）：*\n%s", len(lowestDTCPrices), strings.Join(lowestDTCPrices, "\n"))
		if len(lowestPriceUsers) > 0 {
			var dtcPrices []string
			for i, u := range topPriceUsers {
				dtcPrices = append(dtcPrices, formatIslandDTCPrice(u, i+1))
			}
			replyText += fmt.Sprintf("\n*今日最高进价：*\n%s", strings.Join(dtcPrices, "\n"))
		}
	} else {
		var dtcPrices []string
		for i, u := range topPriceUsers {
			dtcPrices = append(dtcPrices, formatIslandDTCPrice(u, i+1))
		}
		replyText = fmt.Sprintf("*今日高卖价（前 %d）：*\n%s", len(dtcPrices), strings.Join(dtcPrices, "\n"))
		if len(lowestPriceUsers) > 0 {
			var lowestDTCPrices []string
			for i, u := range lowestPriceUsers {
				lowestDTCPrices = append(lowestDTCPrices, formatIslandDTCPrice(u, i+1))
			}
			replyText += fmt.Sprintf("\n*今日最低卖价：*\n%s", strings.Join(lowestDTCPrices, "\n"))
		}
	}

	replyText = strings.ReplaceAll(replyText, "+", "\\+")
	replyText = strings.ReplaceAll(replyText, "-", "\\-")
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text:                  replyText,
			ParseMode:             "MarkdownV2",
			DisableWebPagePreview: true,
		}},
		nil
}

func getTopPriceUsersAndLowestPriceUser(ctx context.Context, chatID int64, localtime time.Time) (topPriceUsers []storage.User, lowestPriceUsers []storage.User, changed bool, err error) {
	group, err := storage.GetGroup(ctx, chatID)
	if err != nil {
		_logger.Error().Err(err).Msg("GetGroup")
		return nil, nil, false, err
	}
	users, err := storage.GetGroupUsers(ctx, chatID)
	if err != nil {
		_logger.Error().Err(err).Msg("GetGroupUsers")
		return nil, nil, false, err
	}

	var priceUsers []storage.User
	for _, u := range users {
		island, residentUID, err := u.GetAnimalCrossingIsland(ctx)
		if err != nil || island == nil {
			continue
		}
		if island.LastPrice.Price <= 0 || island.LastPrice.Price > 999 {
			continue
		}
		var localDate = island.LastPrice.LocationDateTime()
		var h = localDate.Hour()
		if localtime.Sub(localDate) > 24*time.Hour || localtime.Before(localDate) {
			continue
		} else if localtime.Weekday() == 0 {
			if localDate.Weekday() > 0 {
				continue
			} else if localtime.Sub(localDate) > 7*time.Hour {
				continue
			}
		} else if localtime.Weekday() > 0 {
			if h >= 8 && h < 12 && localtime.Sub(localDate) > 4*time.Hour {
				continue
			} else if h >= 12 && h < 22 && localtime.Sub(localDate) > 10*time.Hour {
				continue
			} else if h >= 0 && h < 8 || h >= 22 && h < 24 {
				continue
			}
		}
		if !strings.HasSuffix(island.Name, "岛") {
			island.Name += "岛"
		}
		var nowLoc = time.Now().In(island.Timezone.Location())
		if nowLoc.Hour() < 5 {
			nowLoc = nowLoc.Add(-(time.Duration(nowLoc.Hour()+1) * time.Hour))
		}
		var weekStartDateLoc = nowLoc.AddDate(0, 0, 0-int(nowLoc.Weekday()))
		weekStartDateLoc = time.Date(weekStartDateLoc.Year(), weekStartDateLoc.Month(), weekStartDateLoc.Day(), 0, 0, 0, 0, island.Timezone.Location())
		var weekStartDate = weekStartDateLoc.UTC()
		var weekEndDate = weekStartDate.AddDate(0, 0, 7)
		weekStartDate = weekStartDate.Add(5 * time.Hour)
		uid := u.ID
		if residentUID > 0 {
			uid = residentUID
		}
		island.WeekPriceHistory, err = storage.GetWeeklyDTCPriceHistory(ctx, uid, weekStartDate, weekEndDate)
		if err != nil {
			_logger.Error().Err(err).Int("uid", u.ID).
				Time("weekStart", weekStartDate).
				Time("weekEndDate", weekStartDate).
				Time("weekStartDateLoc", weekStartDateLoc).
				Msg("GetWeeklyDTCPriceHistory")
		}
		u.Island = island
		priceUsers = append(priceUsers, u)
	}

	if len(priceUsers) == 0 {
		return nil, nil, false, errors.New("NoValidPrice")
	}

	topPriceCount := 5
	lowestPriceCount := 1
	if localtime.Weekday() == 0 {
		topPriceCount = 1
		lowestPriceCount = 5
	}
	l := len(priceUsers)
	var topRecords = []storage.ACNHTurnipPricesBoardRecord{}
	var lowestRecords = []storage.ACNHTurnipPricesBoardRecord{}
	if localtime.Weekday() > 0 {
		sort.Slice(priceUsers, func(i, j int) bool {
			return priceUsers[i].Island.LastPrice.Price > priceUsers[j].Island.LastPrice.Price
		})
		if l > topPriceCount {
			for i := topPriceCount; i < l; i++ {
				if priceUsers[i].Island.LastPrice.Price < 500 {
					topPriceCount = i
					break
				}
			}
			topPriceUsers = priceUsers[:topPriceCount]
			lowestPriceUsers = priceUsers[l-1 : l]
		} else {
			topPriceUsers = priceUsers
		}
	} else {
		sort.Slice(priceUsers, func(i, j int) bool {
			return priceUsers[i].Island.LastPrice.Price < priceUsers[j].Island.LastPrice.Price
		})
		if l > lowestPriceCount {
			for i := lowestPriceCount; i < l; i++ {
				if priceUsers[i].Island.LastPrice.Price > 90 {
					lowestPriceCount = i
					break
				}
			}
			lowestPriceUsers = priceUsers[:lowestPriceCount]
			topPriceUsers = priceUsers[l-1 : l]
		} else {
			lowestPriceUsers = priceUsers
		}
	}

	for _, u := range topPriceUsers {
		topRecords = append(topRecords, storage.ACNHTurnipPricesBoardRecord{UserID: u.ID, Price: u.Island.LastPrice.Price})
	}
	for _, u := range lowestPriceUsers {
		lowestRecords = append(lowestRecords, storage.ACNHTurnipPricesBoardRecord{UserID: u.ID, Price: u.Island.LastPrice.Price})
	}

	newACNHTurnipPricesBoard := &storage.ACNHTurnipPricesBoard{TopPriceRecords: topRecords, LowestPriceRecords: lowestRecords}
	changed = !group.ACNHTurnipPricesBoard.Equals(newACNHTurnipPricesBoard)
	if changed {
		group.ACNHTurnipPricesBoard = newACNHTurnipPricesBoard
		if err = group.Update(ctx); err != nil {
			_logger.Error().Err(err).Msg("update group ACNHTurnipPricesBoard")
		}
	}
	return
}

func formatIslandDTCPrice(user storage.User, rank int) string {
	if !strings.HasSuffix(user.Island.Name, "岛") {
		user.Island.Name += "岛"
	}
	var priceTimeout int // minutes
	var timeoutOrCloseDoor string
	d := user.Island.LastPrice.LocationDateTime()
	{
		H := d.Hour()
		var HH int = 0
		if d.Weekday() > 0 {
			if H >= 8 && H < 12 {
				HH = 12
				timeoutOrCloseDoor = "失效"
			} else if H >= 12 && H < 22 {
				HH = 22
				timeoutOrCloseDoor = "关店"
			}
		} else {
			if H >= 5 && H < 12 {
				HH = 12
				timeoutOrCloseDoor = "离开"
			}
		}
		shift := time.Date(d.Year(), d.Month(), d.Day(), HH, 0, 0, 0, user.Island.LastPrice.Timezone.Location())
		priceTimeout = int(shift.UTC().Sub(time.Now()).Minutes())
	}
	var formatedString string
	if d.Weekday() == 0 {
		formatedString = fmt.Sprintf("%d\\. %s的 %s 菜价：__%d__，曹卖可能于%d 分钟后*%s*。", rank, markdownSafe(user.Name), markdownSafe(user.Island.Name), user.Island.LastPrice.Price, priceTimeout, timeoutOrCloseDoor)
	} else {
		url := formatWeekPricesURL(user.Island.WeekPriceHistory)
		formatedString = fmt.Sprintf("%d\\. %s的 %s 菜价：[%d](%s)，将于%d 分钟后*%s*。", rank, markdownSafe(user.Name), markdownSafe(user.Island.Name), user.Island.LastPrice.Price, url, priceTimeout, timeoutOrCloseDoor)
	}
	return formatedString
}

func cmdWhois(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	query := strings.TrimSpace(message.CommandArguments())
	if len(query) == 0 {
		return
	}
	var usermap map[string]struct{} = make(map[string]struct{})
	var groupID int64 = message.Chat.ID
	ctx := context.Background()
	var wg sync.WaitGroup
	wg.Add(5)
	var foundUsersByUserName []storage.User
	go func() {
		foundUsersByUserName, err = storage.GetUsersByName(ctx, query, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Error().Err(err).Msg("error in GetUsersByName")
		}
		wg.Done()
	}()
	var foundUsersByNSAccountName []storage.User
	go func() {
		foundUsersByNSAccountName, err = storage.GetUsersByNSAccountName(ctx, query, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Error().Err(err).Msg("error in GetUsersByNSAccountName")
		}
		wg.Done()
	}()

	var foundUsersByIslandName []storage.User
	go func() {
		foundUsersByIslandName, err = storage.GetUsersByAnimalCrossingIslandName(ctx, query, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Error().Err(err).Msg("error in GetUsersByAnimalCrossingIslandName")
		}
		wg.Done()
	}()

	var foundUserByOwnerName []storage.User
	go func() {
		foundUserByOwnerName, err = storage.GetUsersByAnimalCrossingIslandOwnerName(ctx, query, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Error().Err(err).Msg("error in GetUsersByAnimalCrossingIslandOwnerName")
		}
		wg.Done()
	}()

	var foundUserByIslandInfo []storage.User
	go func() {
		foundUserByIslandInfo, err = storage.GetUsersByAnimalCrossingIslandInfo(ctx, query, groupID)
		if err != nil && status.Code(err) != codes.NotFound {
			_logger.Error().Err(err).Msg("error in GetUsersByAnimalCrossingIslandOpenInfo")
		}
		wg.Done()
	}()
	wg.Wait()

	var replyText string
	if len(foundUserByIslandInfo) > 0 {
		formatedUserInfo := formatUserSearchResult(ctx, usermap, foundUserByIslandInfo)
		if len(formatedUserInfo) > 0 {
			replyText += fmt.Sprintf("找到 岛屿信息中包含近似内容 为 %s 的用户和Ta的岛屿：\n%s\n", query, formatedUserInfo)
		}
	}
	if len(foundUserByOwnerName) > 0 {
		formatedUserInfo := formatUserSearchResult(ctx, usermap, foundUserByOwnerName)
		if len(formatedUserInfo) > 0 {
			replyText += fmt.Sprintf("找到 岛民代表名称 为 %s 的用户和Ta的岛屿：\n%s\n", query, formatedUserInfo)
		}
	}
	if len(foundUsersByIslandName) > 0 {
		formatedUserInfo := formatUserSearchResult(ctx, usermap, foundUsersByIslandName)
		if len(formatedUserInfo) > 0 {
			replyText += fmt.Sprintf("找到 岛屿名称 为 %s 的用户和Ta的岛屿：\n%s\n", query, formatedUserInfo)
		}
	}
	if len(foundUsersByNSAccountName) > 0 {
		formatedUserInfo := formatUserSearchResult(ctx, usermap, foundUsersByNSAccountName)
		if len(formatedUserInfo) > 0 {
			replyText += fmt.Sprintf("找到 NSAccount 为 %s 的用户和Ta的岛屿：\n%s\n", query, formatedUserInfo)
		}
	}
	if len(foundUsersByUserName) > 0 {
		formatedUserInfo := formatUserSearchResult(ctx, usermap, foundUsersByUserName)
		if len(formatedUserInfo) > 0 {
			replyText += fmt.Sprintf("找到用户名为 %s 的用户和Ta的岛屿：\n%s\n", query, formatedUserInfo)
		}
	}

	replyText = strings.TrimSpace(replyText)
	if len(replyText) == 0 {
		replyText = "没有找到狸。"
	}

	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: replyText}},
		nil
}

func formatUserSearchResult(ctx context.Context, usermap map[string]struct{}, users []storage.User) (replyMessage string) {
	var rst []string
	for _, u := range users {
		if _, ok := usermap[u.Name]; ok {
			continue
		}
		usermap[u.Name] = struct{}{}
		if u.Island != nil && u.Island.AirportIsOpen {
			if time.Since(u.Island.OpenTime).Hours() > 24 {
				u.Island.Close(ctx)
				continue
			}
			locOpenTime := u.Island.OpenTime.In(u.Island.Timezone.Location())
			locNow := time.Now().In(u.Island.Timezone.Location())
			if locNow.Hour() >= 5 && (locOpenTime.Hour() >= 0 && locOpenTime.Hour() < 5 ||
				locNow.Day()-locOpenTime.Day() >= 1) {
				u.Island.Close(ctx)
			}
		}
		rst = append(rst, u.Name)
		for _, a := range u.NSAccounts {
			rst = append(rst, a.String())
		}
		if u.Island != nil {
			if !strings.HasSuffix(u.Island.Name, "岛") {
				u.Island.Name += "岛"
			}
			if u.Island.AirportIsOpen {
				rst = append(rst, fmt.Sprintf("岛屿：%s。现正开放，已开放：%d 分钟。\n本会开放特别信息：%s", u.Island.Name, int(time.Since(u.Island.OpenTime).Minutes()), u.Island.Info))
			} else {
				rst = append(rst, "岛屿："+u.Island.Name)
			}
		}
	}
	return strings.Join(rst, "\n")
}

func cmdSearchAnimalCrossingInfo(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if message.Chat.IsPrivate() {
		return
	}
	args := strings.TrimSpace(message.CommandArguments())
	ctx := context.Background()
	var us []storage.User
	if message.ReplyToMessage != nil && message.ReplyToMessage.From.ID != message.From.ID {
		groupID := message.Chat.ID
		u, err := storage.GetUser(ctx, message.ReplyToMessage.From.ID, groupID)
		if err != nil {
			if status.Code(err) == codes.NotFound || err.Error() == "NotFound" {
				return nil, Error{InnerError: err,
					ReplyText: "没有找岛狸",
				}
			}
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
		us = []storage.User{u}
	} else if len(args) > 1 && strings.HasPrefix(args, "@") && args[1:] != message.From.UserName {
		groupID := message.Chat.ID
		us, err = storage.GetUsersByName(ctx, args[1:], groupID)
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "查询记录时出错狸",
			}
		}
	} else if len(args) == 0 {
		return cmdMyIsland(message)
	} else {
		return cmdWhois(message)
	}

	if len(us) == 0 {
		_logger.Info().Msg("users count == 0")
		return []tgbotapi.MessageConfig{{
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
			user = &u
			break
		}
	}

	if user == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到对方的记录狸"}},
			nil
	}

	island, _, err := user.GetAnimalCrossingIsland(ctx)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if island == nil {
		return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "对方尚未登记过自己的 动森 岛屿狸"}},
			nil
	}

	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdHuaShiJiaoHuanBiaoGe(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://docs.google.com/spreadsheets/d/1ZycWgFx7HGTNR7NkMNFwUz-Oiqr4rtXdtHzQ0qW1HGY/edit?usp=sharing"}},
		nil
}
