package impl

import (
	helper2 "cnb.cool/mliev/examples/go-web/internal/helper"
	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/muleiwu/gsr/logger_interface"
)

type HttpDeps struct {
	helper interfaces.HelperInterface
	engine *gin.Engine
}

func NewHttpDeps(helper interfaces.HelperInterface, engine *gin.Engine) *HttpDeps {
	return &HttpDeps{
		helper: helper,
	}
}

// WrapHandler 使用闭包包装处理函数
func (d *HttpDeps) WrapHandler(handler func(*gin.Context, interfaces.GetHelperInterface)) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler(c, d.getHttpDeps(d.getTraceId(c)))
	}
}

func (d *HttpDeps) getTraceId(c *gin.Context) string {
	return c.GetString("traceId")
}

func (d *HttpDeps) getHttpDeps(traceId string) interfaces.HelperInterface {
	h := &helper2.Helper{}
	h.SetLogger(d.getLogger(d.helper.GetLogger(), traceId))
	h.SetDatabase(d.helper.GetDatabase())
	h.SetRedis(d.helper.GetRedis())
	h.SetConfig(d.helper.GetConfig())
	h.SetEnv(d.helper.GetEnv())
	return h
}

func (d *HttpDeps) getLogger(logger logger_interface.LoggerInterface, traceId string) logger_interface.LoggerInterface {
	return NewHttpLogger(logger, traceId)
}
