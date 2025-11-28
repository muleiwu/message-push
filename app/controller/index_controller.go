package controller

import (
	"net/http"

	"cnb.cool/mliev/push/message-push/internal/interfaces"
	"github.com/gin-gonic/gin"
)

type IndexController struct {
	BaseResponse
}

func (receiver IndexController) GetIndex(c *gin.Context, helper interfaces.HelperInterface) {
	helper.GetLogger().Info("visiting homepage")
	c.HTML(http.StatusOK, "index.html", gin.H{
		"title": "Mulei Message Service",
	})
}
