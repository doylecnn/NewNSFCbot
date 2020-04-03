package web

import (
	"context"
	"net/http"
	"net/url"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

var (
	//SecretKey used as hmac-sha256 secret key
	SecretKey [32]byte
	//TgBotClient tgbotapi
	TgBotClient *tgbotapi.BotAPI
	//FirestoreClient client of firestore
	FirestoreClient *firestore.Client
	//FirestoreClientContext context of client of filestore
	FirestoreClientContext context.Context
	//Domain is domain
	Domain string
	//AdminID is admin telegram id
	AdminID int
)

func setCookie(c *gin.Context, name, value string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    url.QueryEscape(value),
		MaxAge:   86400,
		Path:     "/",
		Domain:   Domain,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

func delCookie(c *gin.Context, name string) {
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     name,
		Value:    "",
		MaxAge:   -1,
		Path:     "/",
		Domain:   Domain,
		Secure:   true,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}
