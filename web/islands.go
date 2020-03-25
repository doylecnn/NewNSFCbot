package web

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web/middleware"

	"github.com/gin-gonic/gin"
)

//Islands public islands page
func Islands(c *gin.Context) {
	if v, exists := c.Get("authed"); exists {
		if authed, ok := v.(bool); ok && authed {
			ctx := context.Background()
			users, err := storage.GetAllUsers(ctx)
			if err != nil {
				c.AbortWithError(http.StatusInternalServerError, err)
			} else if len(users) == 0 {
				c.AbortWithError(http.StatusInternalServerError, errors.New("not found user by userid"))
			} else {
				var haveIslandUsers []storage.User
				var priceOutDate []bool
				for i, user := range users {
					nsaccounts, err := user.GetAccounts(ctx)
					if err != nil {
						log.Debug(err)
					} else {
						users[i].NSAccounts = nsaccounts
					}
					island, err := user.GetAnimalCrossingIsland(ctx)
					if err != nil || island == nil {
						continue
					}
					if !strings.HasSuffix(island.Name, "岛") {
						island.Name += "岛"
					}
					users[i].Island = *island
					priceOutDate = append(priceOutDate, time.Since(island.LastPrice.Date) > 12*time.Hour)

					haveIslandUsers = append(haveIslandUsers, *users[i])
				}
				authData, _ := c.Cookie("auth_data_str")
				userID, err := middleware.GetAuthDataInfo(authData, "id")
				if err != nil {
					log.Print(err)
				}
				c.HTML(200, "islands.html", gin.H{
					"uid":          userID,
					"users":        haveIslandUsers,
					"priceOutDate": priceOutDate,
				})
			}
			return
		}
	}
	c.Redirect(http.StatusTemporaryRedirect, "/login")
}
