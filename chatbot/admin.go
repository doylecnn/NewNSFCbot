package chatbot

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

func cmdImportData(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	args := strings.Split(strings.TrimSpace(message.CommandArguments()), ";")
	ctx := context.Background()
	for _, arg := range args {
		var parts = strings.Split(arg, ",")
		if len(parts) != 3 {
			continue
		}
		tgidstr, fcstr, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), strings.TrimSpace(parts[2])
		tgid, err := strconv.ParseInt(tgidstr, 10, 64)
		if err != nil {
			logrus.Warnf("user name:%s, id:%s, err:%v", name, tgidstr, err)
		}
		fc, err := strconv.ParseInt(fcstr, 10, 64)
		if err != nil {
			logrus.Warnf("user name:%s, id:%s, fc:%s, err:%v", name, tgidstr, fcstr, err)
		}
		u := storage.User{ID: int(tgid), Name: name, NSAccounts: []storage.NSAccount{storage.NSAccount{Name: name, FC: storage.FriendCode(fc)}}}
		_, err = storage.GetUser(ctx, u.ID, 0)
		if err != nil {
			if !strings.HasPrefix(err.Error(), "Not found userID:") {
				continue
			}
			u.Create(ctx)
		} else {
			u.Update(ctx)
		}
	}
	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
			BaseChat: tgbotapi.BaseChat{
				ChatID:              message.Chat.ID,
				ReplyToMessageID:    message.MessageID,
				DisableNotification: true},
			Text: "导入完毕。"}},
		nil
}

// cmdUpdateGroupInfo 刷新群信息，移除用户已退出的群
func cmdUpdateGroupInfo(message *tgbotapi.Message) (replyMessage []*tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询用户时出错",
		}
		return
	}
	groups, err := storage.GetAllGroups(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询群组时出错",
		}
		return
	}

	var groupIDs map[int64]struct{} = make(map[int64]struct{})
	for _, g := range groups {
		if _, exists := groupIDs[g.ID]; !exists {
			groupIDs[g.ID] = struct{}{}
		}
	}

	for _, u := range users {
		var changed bool = false
		for _, gid := range u.GroupIDs {
			if _, exists := groupIDs[gid]; !exists {
				groupIDs[gid] = struct{}{}
			}
		}
		for gid := range groupIDs {
			var chatmember tgbotapi.ChatMember
			chatmember, err = tgbot.GetChatMember(tgbotapi.ChatConfigWithUser{ChatID: gid, UserID: u.ID})
			if err != nil {
				if err.Error() == "Bad Request: user not found" {
					logrus.WithError(err).WithFields(logrus.Fields{
						"uid":           u.ID,
						"user_groupids": u.GroupIDs,
						"gid":           gid,
					}).Warn("error when get chat member")
					continue
				}
				logrus.WithError(err).WithFields(logrus.Fields{
					"uid":           u.ID,
					"user_groupids": u.GroupIDs,
					"gid":           gid,
				}).Warn("error when get chat member")
				continue
			}
			if chatmember.HasLeft() || !(chatmember.IsMember() || chatmember.IsCreator() || chatmember.IsAdministrator()) {
				// if len(u.GroupIDs) > 0 {
				// 	for i, gid := range u.GroupIDs {
				// 		if gid == message.Chat.ID {
				// 			// Remove the element at index i from a.
				// 			u.GroupIDs[i] = u.GroupIDs[len(u.GroupIDs)-1] // Copy last element to index i.
				// 			u.GroupIDs[len(u.GroupIDs)-1] = 0             // Erase last element (write zero value).
				// 			u.GroupIDs = u.GroupIDs[:len(u.GroupIDs)-1]   // Truncate slice.
				// 			changed = true
				// 			break
				// 		}
				// 	}
				// }
				logrus.WithFields(logrus.Fields{
					"uid":           u.ID,
					"name":          u.Name,
					"user_groupids": u.GroupIDs,
					"gid":           gid,
				}).Debug("用户不在群")
				continue
			} else {
				if len(u.GroupIDs) == 0 {
					changed = true
					u.GroupIDs = append(u.GroupIDs, gid)
				} else {
					var groupidExists bool = false
					for _, id := range u.GroupIDs {
						if id == gid {
							groupidExists = true
							break
						}
					}
					if !groupidExists {
						changed = true
						u.GroupIDs = append(u.GroupIDs, gid)
					}
				}
			}
		}
		if changed {
			u.Update(ctx)
		}
	}

	return []*tgbotapi.MessageConfig{&tgbotapi.MessageConfig{
		BaseChat: tgbotapi.BaseChat{
			ChatID:              message.Chat.ID,
			ReplyToMessageID:    message.MessageID,
			DisableNotification: true},
		Text: "done"}}, nil
}

func cmdListAllFriendCodes(message *tgbotapi.Message) (replyMessages []*tgbotapi.MessageConfig, err error) {
	if message.From.ID != botAdminID {
		return
	}
	ctx := context.Background()
	users, err := storage.GetAllUsers(ctx)
	if err != nil {
		err = Error{InnerError: err,
			ReplyText: "查询时出错",
		}
		return
	}
	var rst []string
	for i, u := range users {
		userinfo := fmt.Sprintf("%s: %d", u.Name, u.ID)
		for _, a := range u.NSAccounts {
			userinfo += fmt.Sprintf("\n +- %s", a.String())
		}
		rst = append(rst, userinfo)
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
