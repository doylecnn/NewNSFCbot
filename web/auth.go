package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

//Auth is auth page used telegram auth callback
func (w Web) Auth(c *gin.Context) {
	if expectedHash, ok := c.GetQuery("hash"); ok {
		var errorMessage string
		var datas []string
		for k, v := range c.Request.URL.Query() {
			if k == "hash" {
				continue
			}
			datas = append(datas, fmt.Sprintf("%s=%s", k, v[0]))
		}
		sort.Strings(datas)
		mac := hmac.New(sha256.New, w.SecretKey[:])
		authDataStr := strings.Join(datas, "\n")
		io.WriteString(mac, authDataStr)
		hash := fmt.Sprintf("%x", mac.Sum(nil))
		if expectedHash != hash {
			errorMessage = "data is not from Telegram"
		} else if authDate, err := strconv.Atoi(c.Query("auth_date")); err == nil {
			if int64(time.Now().Sub(time.Unix(int64(authDate), 0)).Seconds()) > 86400 {
				errorMessage = "Data is outdated"
			} else {
				w.setCookie(c, "auth_data_str", authDataStr)
				w.setCookie(c, "auth_data_hash", hash)
				userid, err := strconv.ParseInt(c.Query("id"), 10, 64)
				if err != nil {
					_logger.Printf("can not convert %s to int. err* %v", c.Query("id"), err)
				}
				msg := tgbotapi.NewMessage(userid, fmt.Sprintf("hello https://t.me/%d, welcome to NS_FC_bot.", userid))
				_, err = w.TgBotClient.Send(msg)
				if err != nil {
					_logger.Printf("send message to user telegram failed. err: %v", err)
				}
				w.setCookie(c, "authed", "true")
				c.Redirect(http.StatusTemporaryRedirect, "/user/"+c.Query("id"))
				return
			}
		} else {
			errorMessage = err.Error()
		}
		c.Redirect(http.StatusTemporaryRedirect, "/login?error="+errorMessage)
		return
	}
}
