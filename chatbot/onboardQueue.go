package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdStart(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	var argstr = message.CommandArguments()
	if len(argstr) > 0 {
		args := strings.SplitN(argstr, "_", 2)
		if args[0] == "join" {
			return cmdJoinQueue(message, args[1])
		}
	}
	return []tgbotapi.MessageConfig{{
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
/queue [密码] [最大客人数] 开启新的队列，同时根据队列信息，半自动邀请下一位旅客
/myqueue 列出自己创建的队列
/dismiss 解散自己创建的队列

队列参与者：
/list 列出自己加入的队列

其它：
/myfc 管理你登记的 Friend Code`,
	}}, nil
}

func cmdMyQueue(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您还没有登记您的岛屿，请用/addisland 添加您的岛屿信息",
			}
		}
		_logger.Error().Err(err).Msg("cmdMyQueue GetAnimalCrossingIslandByUserID")
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿时出错了。",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID: message.Chat.ID,
				},
				Text: "您没有开启队列",
			}}, nil
		}
		_logger.Error().Err(err).Msg("cmdMyQueue GetOnboardQueue")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	if queue.Dismissed {
		client, err := firestore.NewClient(ctx, _projectID)
		if err != nil {
			_logger.Error().Err(err).Msg("cmdMyQueue newClient")
			return nil, Error{InnerError: err,
				ReplyText: "查询队列时出错了",
			}
		}
		defer client.Close()
		queue.Delete(ctx, client)
		return []tgbotapi.MessageConfig{{
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
	var toggleQueueTypeBtnText string
	if queue.IsAuto {
		toggleQueueTypeBtnText = "切换为手动队列"
	} else {
		toggleQueueTypeBtnText = "切换为自动队列"
	}
	var toggleQueueTypeBtn = tgbotapi.NewInlineKeyboardButtonData(toggleQueueTypeBtnText, "/toggle_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn),
		tgbotapi.NewInlineKeyboardRow(toggleQueueTypeBtn),
	)
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/updatepassword 新密码 更新密码\n/dismiss 立即解散队列\n/myqueue 列出创建的队列\n*请使用下面的按钮操作*", queue.Password)

	return []tgbotapi.MessageConfig{{
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

func cmdUpdatePassword(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	password := message.Text
	if len(password) != 5 {
		return []tgbotapi.MessageConfig{
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
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, Error{InnerError: err,
				ReplyText: "您还没有登记您的岛屿，请用/addisland 添加您的岛屿信息",
			}
		}
		_logger.Error().Err(err).Msg("cmdMyQueue GetAnimalCrossingIslandByUserID")
		return nil, Error{InnerError: err,
			ReplyText: "查询岛屿时出错了。",
		}
	}
	if len(island.OnBoardQueueID) == 0 {
		return []tgbotapi.MessageConfig{{
			BaseChat: tgbotapi.BaseChat{
				ChatID: message.Chat.ID,
			},
			Text: "您没有开启队列",
		}}, nil
	}
	queue, err := island.GetOnboardQueue(ctx)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []tgbotapi.MessageConfig{{
				BaseChat: tgbotapi.BaseChat{
					ChatID: message.Chat.ID,
				},
				Text: "您没有开启队列",
			}}, nil
		}
		_logger.Error().Err(err).Msg("cmdMyQueue GetOnboardQueue")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		_logger.Error().Err(err).Msg("cmdMyQueue newClient")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	defer client.Close()
	if queue.Dismissed {
		defer client.Close()
		queue.Delete(ctx, client)
		return []tgbotapi.MessageConfig{{
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
	notifyNewPassword(queue)
	var shareBtn = tgbotapi.NewInlineKeyboardButtonSwitch("分享队列："+island.Name, "/share_"+queue.ID)
	var dismissBtn = tgbotapi.NewInlineKeyboardButtonData("解散队列", "/dismiss_"+queue.ID)
	var listBtn = tgbotapi.NewInlineKeyboardButtonData("查看队列", "/showqueuemember_"+queue.ID)
	var updatePasswordBtn = tgbotapi.NewInlineKeyboardButtonData("修改密码", "/updatepassword_"+queue.ID)
	var nextBtn = tgbotapi.NewInlineKeyboardButtonData("有请下一位", "/next_"+queue.ID)
	var toggleQueueTypeBtnText string
	if queue.IsAuto {
		toggleQueueTypeBtnText = "切换为手动队列"
	} else {
		toggleQueueTypeBtnText = "切换为自动队列"
	}
	var toggleQueueTypeBtn = tgbotapi.NewInlineKeyboardButtonData(toggleQueueTypeBtnText, "/toggle_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn),
		tgbotapi.NewInlineKeyboardRow(toggleQueueTypeBtn),
	)
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/dismiss 立即解散队列\n/myqueue 列出自己创建的队列\n*请使用下面的按钮操作*", queue.Password)
	tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(message.Chat.ID, message.ReplyToMessage.MessageID))
	return []tgbotapi.MessageConfig{{
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

// notifyNewPassword to current landed user when password is update
func notifyNewPassword(queue *storage.OnboardQueue) {
	updatePasswordText := fmt.Sprintf("岛主更新了密码，新密码如下：\n%s", queue.Password)
	for _, p := range queue.Landed {
		_, err := tgbot.Send(tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      int64(p.UID),
				ReplyMarkup: tgbotapi.ForceReply{ForceReply: true, Selective: true},
			},
			Text: updatePasswordText,
		})
		if err != nil {
			_logger.Error().Err(err).Msg("send new password failed")
			tgbot.Send(tgbotapi.MessageConfig{
				BaseChat: tgbotapi.BaseChat{
					ChatID:      int64(queue.OwnerID),
					ReplyMarkup: tgbotapi.ForceReply{ForceReply: true, Selective: true},
				},
				Text: fmt.Sprintf("通知新的密码给 %s 时，发生错误，请私聊给它新的密码", p.Name),
			})
		}
	}
}
func cmdJoinQueue(message *tgbotapi.Message, queueID string) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	uid := int64(message.From.ID)
	username := message.From.UserName
	if len(username) == 0 {
		username = message.From.FirstName
	}
	ctx := context.Background()
	client, err := firestore.NewClient(ctx, _projectID)
	if err != nil {
		_logger.Error().Err(err).Msg("create firestore client failed")
		return nil, Error{InnerError: err,
			ReplyText: "加入队列失败"}
	}
	defer client.Close()
	queue, err := storage.GetOnboardQueue(ctx, client, queueID)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return []tgbotapi.MessageConfig{tgbotapi.NewMessage(message.Chat.ID, "队列已取消")}, nil
		}
		_logger.Error().Err(err).Msg("query queue failed")
		return nil, Error{InnerError: err,
			ReplyText: "加入队列失败"}
	}
	if queue.OwnerID == uid {
		return []tgbotapi.MessageConfig{tgbotapi.NewMessage(message.Chat.ID, "自己不用排自己的队伍狸……")}, nil
	}
	if err = queue.Append(ctx, client, uid, username); err != nil {
		if err.Error() == "already in this queue" {
			return []tgbotapi.MessageConfig{tgbotapi.NewMessage(message.Chat.ID, "您已经加入了这个队列")}, nil
		} else if err.Error() == "already land island" {
			return []tgbotapi.MessageConfig{tgbotapi.NewMessage(message.Chat.ID, "请离岛后再重新排队")}, nil
		}
		_logger.Error().Err(err).Msg("append queue failed")
		return nil, Error{InnerError: err,
			ReplyText: "加入队列失败"}
	}
	t := queue.Len()
	l, err := queue.GetPosition(uid)
	if err != nil {
		t++
	}
	if queue.IsAuto && queue.LandedLen() < queue.MaxGuestCount {
		sendNotify(ctx, client, queue)
	} else {
		var queueType string
		if queue.IsAuto {
			queueType = "自助队列"
		} else {
			queueType = "岛主手动控制队列"
		}
		var myPositionBtn = tgbotapi.NewInlineKeyboardButtonData("我的位置？", fmt.Sprintf("/position_%s|%d", queue.ID, time.Now().Unix()))
		var leaveBtn = tgbotapi.NewInlineKeyboardButtonData("离开队列："+queue.Name, "/leave_"+queue.ID)
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(myPositionBtn, leaveBtn))
		tgbot.Send(tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:      uid,
				ReplyMarkup: replyMarkup,
			},
			Text: fmt.Sprintf("已加入前往 %s 的队列中排队，本队列为 %s ，当前位置：%d/%d。\n当前岛上有 %d 个客人", queue.Name, queueType, l, t, queue.LandedLen()),
		})
		sentMsg, err := tgbot.Send(tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID: queue.OwnerID,
			},
			Text: fmt.Sprintf("@%s 加入了您创建的队列", username),
		})
		if err != nil {
			_logger.Error().Err(err).Msg("send msg failed")
		} else {
			go func() {
				time.Sleep(55 * time.Second)
				tgbot.DeleteMessage(tgbotapi.NewDeleteMessage(sentMsg.Chat.ID, sentMsg.MessageID))
			}()
		}
	}
	return nil, nil
}

func cmdJoinedQueue(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		return
	}
	ctx := context.Background()
	uid := int64(message.From.ID)
	queues, err := storage.GetJoinedQueue(ctx, uid)
	if err != nil {
		_logger.Error().Err(err).Msg("GetJoinedQueue")
		return nil, Error{InnerError: err,
			ReplyText: "查询队列时出错了",
		}
	}
	if queues == nil || len(queues) == 0 {
		_logger.Warn().Msg("GetJoinedQueue not in any queue")
		return nil, Error{InnerError: nil,
			ReplyText: "您没有加入任何队列",
		}
	}
	var rows [][]tgbotapi.InlineKeyboardButton
	for _, q := range queues {
		position, err := q.GetPosition(uid)
		if err != nil {
			continue
		}
		var btn = tgbotapi.NewInlineKeyboardButtonData(fmt.Sprintf("队列：%s，位置 %d/%d", q.Name, position, q.Len()), fmt.Sprintf("/showqueueinfo_%s", q.ID))
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(btn))
	}
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(rows...)

	return []tgbotapi.MessageConfig{{
		BaseChat: tgbotapi.BaseChat{
			ChatID:      uid,
			ReplyMarkup: replyMarkup,
		},
		Text: "请选择要操作的队列",
	}}, nil
}

func cmdOpenIslandQueue(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonData("点此创建新队列", "/queue")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []tgbotapi.MessageConfig{{
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
	island, residentUID, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
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
	uid := message.From.ID
	if residentUID > 0 {
		uid = residentUID
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
			ReplyText: "/queue 指令至少需要一个参数：开岛密码。请使用下面格式：\n/queue [密码] 开启新的队列\n/queue [密码] [最大客人数] 开启新的队列，同时根据队列信息，半自动邀请下一位旅客",
		}
	}
	password := args[0]
	if len(password) != 5 {
		return nil, Error{InnerError: err,
			ReplyText: "请输入 Dodo Airlines 工作人员 莫里（Orville）提供的 5 位密码",
		}
	}
	var maxGuestCount = 0
	if len(args) == 2 {
		maxGuestCount, err = strconv.Atoi(args[1])
		if err != nil || maxGuestCount < 1 || maxGuestCount > 7 {
			return nil, Error{InnerError: err,
				ReplyText: "第二个参数：同时登岛客人数必须是数字，取值范围 [1，7]",
			}
		}
	}
	owner := message.From.UserName
	if len(owner) == 0 {
		owner = message.From.FirstName
	}
	queue, err := island.CreateOnboardQueue(ctx, int64(uid), owner, password, maxGuestCount)
	if err != nil {
		_logger.Error().Err(err).Msg("创建队列时出错")
		return []tgbotapi.MessageConfig{{
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
	var toggleQueueTypeBtnText string
	if queue.IsAuto {
		toggleQueueTypeBtnText = "切换为手动队列"
	} else {
		toggleQueueTypeBtnText = "切换为自动队列"
	}
	var toggleQueueTypeBtn = tgbotapi.NewInlineKeyboardButtonData(toggleQueueTypeBtnText, "/toggle_"+queue.ID)
	var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(shareBtn, dismissBtn),
		tgbotapi.NewInlineKeyboardRow(listBtn, updatePasswordBtn),
		tgbotapi.NewInlineKeyboardRow(nextBtn),
		tgbotapi.NewInlineKeyboardRow(toggleQueueTypeBtn))
	var replyText = fmt.Sprintf("队列已创建成功，密码：%s\n请使用分享按钮选择要分享排队的群/朋友\n*选择群组后请等待 telegram 弹出分享提示后点击提示！*\n/dismiss 立即解散队列\n/myqueue 列出自己创建的队列\n/comment 留下您的建议或意见\n/donate 您愿意的话可以捐助本项目\n*请使用下面的按钮操作*", queue.Password)

	return []tgbotapi.MessageConfig{{
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

func cmdDismissIslandQueue(message *tgbotapi.Message) (replyMessage []tgbotapi.MessageConfig, err error) {
	if !message.Chat.IsPrivate() {
		var btn = tgbotapi.NewInlineKeyboardButtonURL("请私聊我", "https://t.me/ns_fc_bot")
		var replyMarkup = tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(btn))
		return []tgbotapi.MessageConfig{{
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
	island, _, err := storage.GetAnimalCrossingIslandByUserID(ctx, message.From.ID)
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
		return []tgbotapi.MessageConfig{{
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
	replyMessage = append(replyMessage, tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			DisableNotification: false,
		},
		Text:      fmt.Sprintf("排队前来*%s*的航程已被取消，正在通知旅客狸", queue.Name),
		ParseMode: "MarkdownV2",
	})
	return
}

func notifyQueueDissmised(queue *storage.OnboardQueue) (replyMessage []tgbotapi.MessageConfig) {
	for _, p := range queue.Queue {
		replyMessage = append(replyMessage, tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              p.UID,
				DisableNotification: false,
			},
			Text: fmt.Sprintf("排队前往*%s*的旅客请注意，本次航程已被取消狸", queue.Name),
		})
	}
	return
}
