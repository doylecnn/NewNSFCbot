package chatbot

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

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
		return nil, Error{InnerError: err,
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
			u = &storage.User{
				ID:              message.From.ID,
				Name:            username,
				NameInsensitive: strings.ToLower(username),
				GroupIDs:        []int64{groupID},
			}
		} else {
			u = &storage.User{
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
		if island, err = u.GetAnimalCrossingIsland(ctx); err != nil && status.Code(err) != codes.NotFound {
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
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: rstText}},
		nil
}

func cmdUpdateIslandBaseInfo(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	if len(args) == 0 {
		return
	}
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{{
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
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新了岛屿信息：\n" + island.BaseInfo}},
		nil
}

func cmdSetIslandTimezone(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
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
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
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
	island.Timezone = timezone
	if err = island.Update(ctx); err != nil {
		logrus.WithError(err).Error("更新时区时出错狸")
		return nil, Error{InnerError: err,
			ReplyText: "更新时区时出错狸",
		}
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新了岛屿时区：\n" + island.Timezone.String()}},
		nil
}

func cmdMyIsland(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错了狸",
		}
	}
	if island == nil {
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录"}},
			nil
	}
	return []*tgbotapi.MessageConfig{{
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
	logrus.Debugf("user:%s", u.Name)
	island, err := u.GetAnimalCrossingIsland(ctx)
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
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "没有找到您的记录，请先使用 addisland 命令添加岛屿记录狸"}},
			nil
	}
	if !island.AirportIsOpen || island.Info != islandInfo {
		island.AirportIsOpen = true
		island.Info = islandInfo
		island.Update(ctx)
	}
	return []*tgbotapi.MessageConfig{{
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
	if err != nil && status.Code(err) != codes.NotFound {
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if err != nil && status.Code(err) == codes.NotFound {
		return []*tgbotapi.MessageConfig{{
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
		return []*tgbotapi.MessageConfig{{
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
		island.Update(ctx)
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdListIslands(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: fmt.Sprintf("https://%s/islands", _domain)}},
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
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "请先登记你的岛屿狸。\n本bot 原本是为交换Nintendo Switch Friend Code而生。\n所以建议先/addfc 登记fc，再/addisland 登记岛屿，再/dtcj 发布价格。\n具体命令帮助请/help",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "更新报价时出错狸",
		}
	}
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "更新大头菜报价成功狸"}},
		nil
}

// cmdDTCWeekPriceAndPredict 当周菜价回看/预测
func cmdDTCWeekPriceAndPredict(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	args := strings.TrimSpace(message.CommandArguments())
	uid := message.From.ID
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, uid)
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
	var prices []*storage.PriceHistory
	var weekStartDateUTC = time.Now().AddDate(0, 0, 0-int(time.Now().Weekday())).Truncate(24 * time.Hour)
	var weekStartDateLoc = time.Date(weekStartDateUTC.Year(), weekStartDateUTC.Month(), weekStartDateUTC.Day(), 0, 0, 0, 0, island.Timezone.Location())
	var weekStartDate = weekStartDateLoc.UTC()
	var weekEndDate = weekStartDate.AddDate(0, 0, 7)
	if len(args) != 0 {
		prices, err = makeWeeklyPrice(strings.TrimSpace(message.CommandArguments()), island.Timezone, weekStartDate, weekEndDate)
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "更新一周报价时出错狸，请确认格式：\n[买入价] [上午价/下午价]……\n价格范围[1, 999]",
			}
		}
	}
	priceHistory, err := storage.GetWeeklyDTCPriceHistory(ctx, uid, weekStartDate, weekEndDate)
	if err != nil {
		logrus.WithError(err).Error("GetWeeklyDTCPriceHistory")
		return nil, Error{InnerError: err,
			ReplyText: "查找报价信息时出错狸",
		}
	}
	if priceHistory == nil {
		logrus.Error("priceHistory == nil")
	}
	if len(priceHistory) == 0 {
		copy(priceHistory, prices)
	} else {
		if l := len(prices); prices != nil || l > 0 {
			var i, j int = 0, 0
			for ; i < l; i++ {
				if j < len(priceHistory) {
					ophd := priceHistory[j].LocationDateTime()
					nphd := prices[i].LocationDateTime()
					if nphd.Weekday() == ophd.Weekday() &&
						((nphd.Hour() >= 8 && nphd.Hour() < 12 &&
							ophd.Hour() < 12) ||
							(nphd.Hour() >= 12 && nphd.Hour() < 21 &&
								ophd.Hour() >= 12)) {
						priceHistory[j].Delete(ctx)
					} else {
						priceHistory = append(priceHistory, nil)
						copy(priceHistory[j+1:], priceHistory[j:])
					}
					priceHistory[j] = prices[i]
					j++
				}
			}
		}
	}
	for _, ph := range priceHistory {
		if err = ph.Set(ctx, uid); err != nil {
			logrus.WithError(err).Error("set price history")
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
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: false},
			Text:      replyText,
			ParseMode: "MarkdownV2",
		}},
		nil
}

func makeWeeklyPrice(args string, islandTimezone storage.Timezone, startDate, endDate time.Time) (priceHistory []*storage.PriceHistory, err error) {
	prices := strings.Split(strings.Trim(args, "/"), " ")
	if len(prices) < 1 || len(prices) > 7 {
		return nil, errors.New("wrong format")
	}
	if strings.Contains(prices[0], "/") {
		return nil, errors.New("wrong format")
	}
	var intPrice []int
	ip, err := strconv.Atoi(prices[0])
	if err != nil {
		return nil, errors.New("wrong format")
	}
	intPrice = append(intPrice, ip)
	l1 := len(prices[1:])
	for i, price := range prices[1:] {
		ps := strings.Split(price, "/")
		l2 := len(ps)
		if l2 > 2 || (i != l1-1 && l2 == 1) {
			return nil, errors.New("wrong format")
		}
		for _, p := range ps {
			ip, err = strconv.Atoi(p)
			if ip < 1 || ip > 999 || err != nil {
				return nil, errors.New("wrong format")

			}
			intPrice = append(intPrice, ip)
		}
	}
	for i := 0; i < len(intPrice); i++ {
		if i == 0 {
			priceHistory = append(priceHistory, &storage.PriceHistory{Date: startDate, Price: intPrice[i]})
			startDate = startDate.AddDate(0, 0, 1)
		} else {
			if i%2 == 1 {
				startDate = startDate.Add(8 * time.Hour)
				priceHistory = append(priceHistory, &storage.PriceHistory{Date: startDate, Price: intPrice[i]})
			} else {
				startDate = startDate.Add(4 * time.Hour)
				priceHistory = append(priceHistory, &storage.PriceHistory{Date: startDate, Price: intPrice[i]})
				startDate = startDate.Add(12 * time.Hour)
			}
		}
	}
	return
}

/*	本周您的报价如下: 可以 点我 查询本周价格趋势
 *	| Sun | Mon | Tue | Wed | Thu | Fri | Sat |
 *	| - | -/105 | -/- | -/- | -/- | -/- | -/- |
 *	未录入星期日数据 无法生成查询数据 */
func formatWeekPrices(priceHistory []*storage.PriceHistory) (text string, err error) {
	var weekPrices []string = []string{"\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-", "\\-"}
	for _, p := range priceHistory {
		if p != nil {
			var j = int(p.Date.Weekday())
			if j == 0 {
				weekPrices[j] = strconv.Itoa(p.Price)
			} else if p.Date.Hour() < 12 {
				weekPrices[j*2-1] = strconv.Itoa(p.Price)
			} else {
				weekPrices[j*2] = strconv.Itoa(p.Price)
			}
		}
	}
	var datePrice []string = make([]string, 7)
	datePrice[0] = weekPrices[0]
	for i := 1; i < 13; i += 2 {
		datePrice[(i+1)/2] = fmt.Sprintf("%s/%s", weekPrices[i], weekPrices[i+1])
	}
	return fmt.Sprintf("本周您的报价如下: 可以 [点我](https://ac-turnip.com/#%s) 查询本周价格趋势\n"+
		"\\| Sun \\| Mon \\| Tue \\| Wed \\| Thu \\| Fri \\| Sat \\|\n"+
		"\\| %s \\|", strings.TrimRight(strings.Join(weekPrices, ","), ",\\-"), strings.Join(datePrice, " \\| ")), nil
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
		u.Island = island
		priceUsers = append(priceUsers, u)
	}

	if len(priceUsers) == 0 {
		return []*tgbotapi.MessageConfig{{
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

	return []*tgbotapi.MessageConfig{{
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
	foundUsersByUserName, err := storage.GetUsersByName(ctx, query, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("error in GetUsersByName")
	}

	foundUsersByNSAccountName, err := storage.GetUsersByNSAccountName(ctx, query, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("error in GetUsersByNSAccountName")
	}

	foundUsersByIslandName, err := storage.GetUsersByAnimalCrossingIslandName(ctx, query, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("error in GetUsersByAnimalCrossingIslandName")
	}

	foundUserByOwnerName, err := storage.GetUsersByAnimalCrossingIslandOwnerName(ctx, query, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("error in GetUsersByAnimalCrossingIslandOwnerName")
	}

	foundUserByIslandInfo, err := storage.GetUsersByAnimalCrossingIslandInfo(ctx, query, groupID)
	if err != nil && status.Code(err) != codes.NotFound {
		logrus.WithError(err).Error("error in GetUsersByAnimalCrossingIslandOpenInfo")
	}

	var replyText string
	if len(foundUsersByUserName) > 0 {
		replyText += fmt.Sprintf("找到用户名为 %s 的用户和Ta的岛屿：\n%s\n", query, userInfo(foundUsersByUserName))
	}

	if len(foundUsersByNSAccountName) > 0 {
		replyText += fmt.Sprintf("找到 NSAccount 为 %s 的用户和Ta的岛屿：\n%s\n", query, userInfo(foundUsersByNSAccountName))
	}

	if len(foundUsersByIslandName) > 0 {
		replyText += fmt.Sprintf("找到 岛屿名称 为 %s 的用户和Ta的岛屿：\n%s\n", query, userInfo(foundUsersByIslandName))
	}

	if len(foundUserByOwnerName) > 0 {
		replyText += fmt.Sprintf("找到 岛民代表名称 为 %s 的用户和Ta的岛屿：\n%s\n", query, userInfo(foundUserByOwnerName))
	}

	if len(foundUserByIslandInfo) > 0 {
		replyText += fmt.Sprintf("找到 岛屿信息中包含近似内容 为 %s 的用户和Ta的岛屿：\n%s\n", query, userInfo(foundUserByIslandInfo))
	}
	replyText = strings.TrimSpace(replyText)
	if len(replyText) == 0 {
		replyText = "没有找到狸。"
	}

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: replyText}},
		nil
}

func userInfo(users []*storage.User) (replyMessage string) {
	var rst []string
	for _, u := range users {
		rst = append(rst, u.Name)
		for _, a := range u.NSAccounts {
			rst = append(rst, a.String())
		}
		if u.Island != nil {
			if !strings.HasSuffix(u.Island.Name, "岛") {
				u.Island.Name += "岛"
			}
			rst = append(rst, "岛屿："+u.Island.Name)
		}
	}
	return strings.Join(rst, "\n")
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
			if status.Code(err)==codes.NotFound {
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
	} else if len(args) == 0 {
		return cmdMyIsland(message)
	} else {
		return cmdWhois(message)
	}

	if len(us) == 0 {
		logrus.Info("users count == 0")
		return []*tgbotapi.MessageConfig{{
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
		return []*tgbotapi.MessageConfig{{
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
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true},
				Text: "对方尚未登记过自己的 动森 岛屿狸"}},
			nil
	}

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: island.String()}},
		nil
}

func cmdHuaShiJiaoHuanBiaoGe(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "https://docs.google.com/spreadsheets/d/1ZycWgFx7HGTNR7NkMNFwUz-Oiqr4rtXdtHzQ0qW1HGY/edit?usp=sharing"}},
		nil
}
