package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	"github.com/doylecnn/new-nsfc-bot/storage"
	"github.com/doylecnn/new-nsfc-bot/web/middleware"
	"github.com/sirupsen/logrus"

	"github.com/gin-gonic/gin"
)

//User user page
func (w Web) User(c *gin.Context) {
	if v, exists := c.Get("authed"); exists {
		if authed, ok := v.(bool); ok && authed {
			authData, _ := c.Cookie("auth_data_str")
			userID, err := middleware.GetAuthDataInfo(authData, "id")
			if err != nil {
				logrus.Print(err)
			}
			userid := c.Param("userid")
			if userID == userid {
				ctx := context.Background()
				uid, err := strconv.ParseInt(userid, 10, 64)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
				}
				user, err := storage.GetUser(ctx, int(uid), 0)
				if err != nil {
					c.AbortWithError(http.StatusInternalServerError, err)
				} else if user == nil {
					c.AbortWithError(http.StatusInternalServerError, errors.New("not found user by userid"))
				} else {
					island, err := user.GetAnimalCrossingIsland(ctx)
					if err != nil {
						c.AbortWithError(http.StatusInternalServerError, err)
					}
					pricehistory, err := storage.GetPriceHistory(ctx, int(uid))
					if err != nil {
						c.AbortWithError(http.StatusInternalServerError, err)
					}
					c.HTML(200, "user.html", gin.H{
						"userID":       user.ID,
						"name":         user.Name,
						"island":       island,
						"pricehistory": pricehistory,
					})
				}
			} else {
				logrus.Printf("userid: [%s] != userid: [%s]", userID, userid)
				c.AbortWithStatus(http.StatusForbidden)
			}
			return
		}
	}
	c.Redirect(http.StatusTemporaryRedirect, "/login")
}
