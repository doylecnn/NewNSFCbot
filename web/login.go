package web

import (
	"github.com/gin-gonic/gin"
)

//Login login page
func (w Web) Login(c *gin.Context) {
	errormessage := c.Query("error")
	c.HTML(200, "login.html", gin.H{
		"errorMessage": errormessage,
	})
}
