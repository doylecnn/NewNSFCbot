package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

//TelegramAuth 使用telegram 来登录
func TelegramAuth(secretKey [32]byte) gin.HandlerFunc {
	return func(c *gin.Context) {
		authData, err := c.Cookie("auth_data_str")
		if err != nil || len(authData) == 0 {
			redirectToLogin(c, "cookie: auth_data_str is not set")
			return
		}
		expectedHash, err := c.Cookie("auth_data_hash")
		if err != nil || len(expectedHash) == 0 {
			redirectToLogin(c, "cookie: auth_data_hash is not set")
			return
		}

		info, err := GetAuthDataInfo(authData, "auth_date")
		if err != nil {
			redirectToLogin(c, err.Error())
			return
		}
		authDate, err := strconv.Atoi(info)
		if err != nil {
			redirectToLogin(c, fmt.Sprintf("authdate:%s, err: %v", info, err))
			return
		}

		mac := hmac.New(sha256.New, secretKey[:])
		io.WriteString(mac, authData)
		hash := fmt.Sprintf("%x", mac.Sum(nil))
		if expectedHash != hash {
			redirectToLogin(c, "data is not from Telegram")
			return
		} else if int64(time.Now().Sub(time.Unix(int64(authDate), 0)).Seconds()) > 86400 {
			redirectToLogin(c, "Data is outdated")
			return
		}
		c.Set("authed", true)
		c.Next()
	}
}

func redirectToLogin(c *gin.Context, errorMessage string) {
	DelCookie(c, "auth_data_str")
	DelCookie(c, "auth_data_hash")
	logrus.Printf("errormessage:%s", errorMessage)
	c.Redirect(http.StatusTemporaryRedirect, "/login?error=LoginFailed")
	c.Abort()
}

// GetAuthDataInfo get user auth data info
func GetAuthDataInfo(authData, key string) (value string, err error) {
	err = fmt.Errorf("key: %s not found", key)
	s := key + "="
	lIdx := strings.Index(authData, s)
	if lIdx == -1 {
		return
	}
	lIdx += len(s)
	rIdx := strings.Index(authData[lIdx:], "\n")
	if rIdx == -1 {
		return
	}
	rIdx += lIdx
	value = strings.TrimSpace(authData[lIdx:rIdx])
	if len(value) == 0 {
		err = fmt.Errorf("key: %s is exists, but value is empty", key)
	} else {
		err = nil
	}
	return
}

// DelCookie delete cookie
func DelCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Domain:   c.Request.URL.Host,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
