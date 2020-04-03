package web

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"

	"github.com/doylecnn/new-nsfc-bot/chatbot"
	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web/middleware"
	"github.com/gin-gonic/gin"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api"
	"github.com/sirupsen/logrus"
)

// Web is web
type Web struct {
	Domain      string
	Port        string
	TgBotToken  string
	TgBotClient *tgbotapi.BotAPI
	SecretKey   [32]byte
	Route       *gin.Engine
	AdminID     int
}

// NewWeb return new Web
func NewWeb(token, appID, projectID, port string, adminID int, bot chatbot.ChatBot) (web Web, updates chan tgbotapi.Update) {
	AdminID = adminID
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

	admin := r.Group("/admin", middleware.TelegramAdminAuth(SecretKey, AdminID))
	{
		admin.GET("/export/:userid", export)
		admin.GET("/botrestart", func(c *gin.Context) {
			bot.RestartBot()
		})
	}

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
		AdminID:     adminID,
	}
	return
}

// Run run the web
func (w Web) Run() {
	w.Route.Run(fmt.Sprintf(":%s", w.Port))
}

type exportData struct {
	ID         int         `json:"id"`
	Name       string      `json:"name"`
	NSAccounts []nsAccount `json:"ns_accounts,omitempty"`
	Games      []game      `json:"games,omitempty"`
	Groups     []group     `json:"groups,omitempty"`
}

type nsAccount struct {
	Name string `json:"name"`
	FC   int64  `json:"friend_code"`
}

type game struct {
	Name string                 `json:"name"`
	Info map[string]interface{} `json:"info"`
}

type group struct {
	ID    int64  `json:"id"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func export(c *gin.Context) {
	if v, exists := c.Get("authed"); exists {
		if authed, ok := v.(bool); ok && authed {
			authData, _ := c.Cookie("auth_data_str")
			userID, err := middleware.GetAuthDataInfo(authData, "id")
			if err != nil {
				logrus.Print(err)
				c.Abort()
				return
			}
			userid := c.Param("userid")
			if userID == userid {
				uid, err := strconv.ParseInt(userid, 10, 64)
				if err != nil {
					logrus.Warn(err)
					c.Abort()
					return
				}
				if int(uid) == AdminID {
					ctx := context.Background()
					us, err := storage.GetAllUsers(ctx)
					if err != nil {
						logrus.Warn(err)
						c.Abort()
						return
					}

					var allgroups map[int64]group = make(map[int64]group)
					if gs, err := storage.GetAllGroups(ctx); err == nil {
						for _, g := range gs {
							allgroups[g.ID] = group{ID: g.ID,
								Type:  g.Type,
								Title: g.Title}
						}
					}

					var userinfos []exportData
					for _, u := range us {
						var nsaccounts []nsAccount
						for _, a := range u.NSAccounts {
							nsaccounts = append(nsaccounts, nsAccount{FC: int64(a.FC), Name: a.Name})
						}
						var games []game
						if axi, err := u.GetAnimalCrossingIsland(ctx); err == nil {
							if axi != nil {
								var pricehistory map[int64]map[string]int64 = map[int64]map[string]int64{}
								if ph, err := storage.GetPriceHistory(ctx, int(uid)); err == nil {
									for _, p := range ph {
										pricehistory[p.Date.Unix()] = map[string]int64{
											"date":  p.Date.Unix(),
											"price": int64(p.Price),
										}
									}
								}
								g := game{Name: "AnimalCrossing",
									Info: map[string]interface{}{
										"airportIsOpen":   axi.AirportIsOpen,
										"airportPassword": axi.AirportPassword,
										"fruits":          axi.Fruits,
										"hemisphere":      axi.Hemisphere,
										"name":            axi.Name,
										"owner":           axi.Owner,
										"priceHistory":    pricehistory,
									},
								}
								games = append(games, g)
							}
						}
						var groups []group
						for _, gid := range u.GroupIDs {
							groups = append(groups, allgroups[gid])
						}
						ui := exportData{
							ID:         u.ID,
							Name:       u.Name,
							NSAccounts: nsaccounts,
							Games:      games,
							Groups:     groups,
						}
						userinfos = append(userinfos, ui)
					}
					c.SecureJSON(http.StatusOK, userinfos)
					return
				}
			}
		}
	}
	errormessage := c.Query("error")
	c.HTML(200, "login.html", gin.H{
		"errorMessage": errormessage,
	})
}
