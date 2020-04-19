package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdStart(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID: message.Chat.ID,
		},
		Text: `排队相关指令：
岛主：
/open 用于开发岛屿
/open [本次开岛特色信息] 用于更新 本次开岛特色信息。
/close 用于关闭岛屿
/islandinfo 更新岛屿基本信息

队列主：
/queue [密码] 开启新的队列
/queue [密码] [开岛说明] 开启新的队列，同时更新开岛说明
/queue [密码] [开岛说明] [最大客人数] 开启新的队列，同时更新开岛说明，同时根据队列信息，半自动邀请下一位旅客（尚未实现）
/myqueue 列出自己创建的队列
/dismiss 解散自己创建的队列

队列参与者：
/list 列出自己加入的队列

其它：
/myfc 管理你登记的 Friend Code`,
	}}, nil
}

func cmdMyQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您还没有登记您的岛屿，请用/addisland 添加您的岛屿信息",
			}
		}
		logrus.WithError(err).Error("cmdMyQueue GetAnimalCrossingIslandByUserID")
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿时出错了。",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID: message.Chat.ID,
				},
				Text: "您没有开启队列",
			}}, nil
		}
		logrus.WithError(err).Error("cmdMyQueue GetOnboardQueue")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	if queue.Dismissed {
		client, err := firestore.NewClient(ctx, _projectID)
		if err != nil {
			logrus.WithError(err).Error("cmdMyQueue newClient")
			return nil, Error{InnerError: err,
				ReplyText: "查询队列时出错了",
			}
		}
		defer client.Close()
		queue.Delete(ctx, client)
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+island.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn))
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/updatepassword 新密码 更新密码\n/dismiss 立即解散队列\n/myqueue 列出创建的队列\n*请使用下面的按钮操作*", queue.Password)

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text:      replyText,
			ParseMode: "MarkdownV2",
		}},
		nil
}

func cmdUpdatePassword(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	password := message.Text
	if len(password) != 5 {
		return []*tgbotapi.MessageConfig{
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID: message.Chat.ID,
				},
				Text: "密码一定有 5 位",
			},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:      message.Chat.ID,
					ReplyMarkup: tgbotapi.ForceReply{ForceReply: true, Selective: true},
				},
				Text: "请输入新的密码",
			},
		}, nil
	}
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您还没有登记您的岛屿，请用/addisland 添加您的岛屿信息",
			}
		}
		logrus.WithError(err).Error("cmdMyQueue GetAnimalCrossingIslandByUserID")
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿时出错了。",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID: message.Chat.ID,
				},
				Text: "您没有开启队列",
			}}, nil
		}
		logrus.WithError(err).Error("cmdMyQueue GetOnboardQueue")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		logrus.WithError(err).Error("cmdMyQueue newClient")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	defer client.Close()
	if queue.Dismissed {
		defer client.Close()
		queue.Delete(ctx, client)
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	queue.Password = password
	if err = queue.Update(ctx, client); err != nil {
		return nil, Error{InnerError: err,
			ReplyText: "更新队列密码时出错了",
		}
	}
	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+island.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn))
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/dismiss 立即解散队列\n/myqueue 列出自己创建的队列\n*请使用下面的按钮操作*", queue.Password)
	tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(message.Chat.ID, message.ReplyToMessage.MessageID))
	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text:      replyText,
			ParseMode: "MarkdownV2",
		}},
		nil
}

func cmdJoinedQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	uid := int64(message.From.ID)
	queues, err := storage.GetJoinedQueue(ctx, uid)
	if err != nil {
		logrus.WithError(err).Error("cmdMyQueue newClient")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	if queues == nil || len(queues) == 0 {
		logrus.WithError(err).Error("cmdMyQueue newClient")
		return nil, Error{InnerError: err,
			ReplyText: "您没有加入任何队列",
		}
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, q := range queues {
		position, err := q.GetPosition(uid)
		if err != nil {
			continue
		}
		var btn = tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("队列：%s，位置 %d/%d", q.Name, position+1, q.Len()), "/showqueueinfo_"+q.ID)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)

	return []*tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      uid,
			ReplyMarkup: replyMarkup,
		},
		Text: "请选择要操作的队列",
	}}, nil
}

func cmdOpenIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonData("点此创建新队列", "/queue")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊 @NS_FC_bot 后使用 */queue* \\[密码\\] 开始创建上岛队列。\n您必须已经使用 */addisand* 登记过岛屿。\n创建队列后，密码不会直接公开。\n创建队列是队列创建成功后，bot 会记录您的岛屿为开放状态。",
				ParseMode: "MarkdownV2",
			}},
			nil
	}
	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "没有找到您的岛屿信息狸，如未记录，请先使用/addisland 登记岛屿信息狸。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询记录时出错狸",
		}
	}
	if len(island.OnBoardQueueID) != 0 {
		queue, _ := island.GetOnboardQueue(ctx)
		if queue != nil {
			if queue.Dismissed {
				_, err = island.ClearOldOnboardQueue(ctx)
			} else {
				return nil, Error{InnerError: err,
					ReplyText: "请先 /dismiss 解散您当前已发起的队列",
				}
			}
		}
	}

	argstr := strings.TrimSpace(message.CommandArguments())
	args := strings.Split(argstr, " ")
	if len(args) == 0 {
		return nil, Error{InnerError: err,
			ReplyText: "/queue 至少需要一个参数，开岛密码。",
		}
	}
	password := args[0]
	if len(password) != 5 {
		return nil, Error{InnerError: err,
			ReplyText: "请输入 Dodo Airlines 工作人员 莫里（Orville）提供的 5 位密码",
		}
	}
	var specialInfo = ""
	if len(args) >= 2 {
		specialInfo = args[1]
	}
	var maxGuestCount = 0
	if len(args) == 3 {
		maxGuestCount, err = strconv.Atoi(args[2])
		if err != nil {
			return nil, Error{InnerError: err,
				ReplyText: "第三个参数：同时登岛客人数必须是数字，取值范围 [1，7]",
			}
		}
		if maxGuestCount < 1 || maxGuestCount > 7 {
			return nil, Error{InnerError: err,
				ReplyText: "第三个参数：同时登岛客人数必须是数字，取值范围 [1，7]",
			}
		}
	}
	owner := message.From.UserName
	if len(owner) == 0 {
		owner = message.From.FirstName
	}
	queue, err := island.CreateOnboardQueue(ctx, int64(message.From.ID), owner, password, specialInfo, maxGuestCount)
	if err != nil {
		logrus.WithError(err).Error("创建队列时出错")
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("创建队列 %s 时出错狸，已通知bot 管理员。", queue.Name)},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("创建队列 %s 时出错，error：%v", queue.Name, err)},
		}, nil
	}
	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+island.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn))
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/dismiss 立即解散队列\n/myqueue 列出自己创建的队列\n*请使用下面的按钮操作*", queue.Password)

	return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true,
				ReplyMarkup:         replyMarkup,
			},
			Text:      replyText,
			ParseMode: "MarkdownV2",
		}},
		nil
}

func cmdDismissIslandQueue(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []*tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              message.Chat.ID,
					ReplyToMessageID:    message.MessageID,
					DisableNotification: true,
					ReplyMarkup:         replyMarkup,
				},
				Text:      "请私聊bot 后使用 */dismiss* 解散队列",
				ParseMode: "MarkdownV2",
			}},
			nil
	}

	ctx := context.Background()
	island, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "没有找到您的岛屿信息狸，如未记录，请先使用/addisland 登记岛屿信息狸。",
			}
		}
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿记录时出错狸",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return nil, Error{InnerError: err,
			ReplyText: "当前没有创建登岛队列狸。如有需要请使用 /queue [密码] [最大同时登岛客人数] 创建队列",
		}
	}
	queue, err := island.ClearOldOnboardQueue(ctx)
	if err != nil {
		return []*tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("清理队列 %s 时出错，已通知bot 管理员。error：%v", queue.Name, err)},
			{
				BaseChat: tgbotapi.BaseChat{
					ChatID:              int64(botAdminID),
					DisableNotification: false,
				},
				Text: fmt.Sprintf("清理队列 %s 时出错，error：%v", queue.Name, err)},
		}, nil
	}
	replyMessage = append(replyMessage, notifyQueueDissmised(queue)...)
	replyMessage = append(replyMessage, &tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text:      fmt.Sprintf("排队前来*%s*的航程已被取消，正在通知旅客狸", queue.Name),
		ParseMode: "MarkdownV2",
	})
	return
}

func notifyQueueDissmised(queue *storage.OnboardQueue) (replyMessage []*tgbotapi.MessageConfig) {
	for _, p := range queue.Queue {
		replyMessage = append(replyMessage, &tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              p.UID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("排队前往*%s*的旅客请注意，本次航程已被取消狸", queue.Name),
		})
	}
	return
}
