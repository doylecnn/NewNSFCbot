package web

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/doylecnn/new-nsfc-bot/chatbot"
	"github.com/doylecnn/new-nsfc-bot/web/middleware"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
)

type Web struct {
	Domain      string
	Port        string
	TgBotToken  string
	TgBotClient *tgbotapi.BotAPI
	SecretKey   [32]byte
	Route       *gin.Engine
}

// NewWeb return new Web
func NewWeb(token, appID, projectID, port string, adminID int, bot chatbot.ChatBot) (web Web, updates chan tgbotapi.Update) {
	gin.SetMode(gin.ReleaseMode)
	TgBotClient = bot.TgBotClient
	r := gin.New()

	r.Use(gin.Recovery())
	r.LoadHTMLGlob("web/templates/*")

	r.GET("/", Index)
	r.GET("/index", Index)
	r.GET("/auth", Auth)
	r.GET("/login", Login)

	SecretKey = sha256.Sum256([]byte(token))
	authorized := r.Group("/", middleware.TelegramAuth(SecretKey))
	{
		authorized.GET("/user/:userid", User)
		authorized.GET("/islands", Islands)
		authorized.GET("/logout", Logout)
	}

	r.GET("/botoffline", func(c *gin.Context) {
		bot.Stop()
		close(updates)
	})

	updates = make(chan tgbotapi.Update, bot.TgBotClient.Buffer)
	r.POST("/"+token, func(c *gin.Context) {
		bytes, _ := ioutil.ReadAll(c.Request.Body)

		var update tgbotapi.Update
		json.Unmarshal(bytes, &update)

		updates <- update
	})

	web = Web{
		Domain:      fmt.Sprintf("%s.appspot.com", appID),
		Port:        port,
		TgBotToken:  token,
		TgBotClient: bot.TgBotClient,
		SecretKey:   SecretKey,
		Route:       r,
	}
	return
}

// Run run the web
func (w Web) Run() {
	w.Route.Run(fmt.Sprintf(":%s", w.Port))
}
