package impl

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"runtime"
	"syscall"
	"time"

	"cnb.cool/mliev/examples/go-web/internal/interfaces"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type HttpServer struct {
	Helper     interfaces.HelperInterface
	routerFunc func(router *gin.Engine)
}

func NewHttpServer(helper interfaces.HelperInterface) *HttpServer {
	return &HttpServer{
		Helper: helper,
	}
}

// RunHttp 启动HTTP服务器并注册路由和中间件
func (receiver *HttpServer) RunHttp() {
	// 设置Gin模式
	if receiver.Helper.GetConfig().GetString("http.mode", "") == "release" {
		gin.SetMode(gin.ReleaseMode)
	}

	// 完全替换gin的默认Logger
	gin.DisableConsoleColor()
	//zapLogger := receiver.logger
	//gin.DefaultWriter = &zapLogWriter{zapLogger: zapLogger}
	//gin.DefaultErrorWriter = &zapLogWriter{zapLogger: zapLogger, isError: true}

	// 配置Gin引擎
	// 配置Gin引擎并替换默认logger
	engine := gin.New()
	// 增加链路的追踪ID
	engine.Use(receiver.traceIdMiddleware())
	// 对路由的输入输出记录
	engine.Use(receiver.ginLogger())
	// 防止 panic 导致的程序崩溃
	engine.Use(gin.Recovery())

	// 加载HTML模板
	if err := receiver.loadTemplates(engine); err != nil {
		receiver.Helper.GetLogger().Error(fmt.Sprintf("加载模板失败: %v", err))
	}

	// 加载网站静态资源
	receiver.loadWebStatic(engine)

	// 注册中间件
	//handlerFuncs := config.MiddlewareConfig{}.Get()
	middlewareFuncList := receiver.Helper.GetConfig().Get("http.middleware", []gin.HandlerFunc{}).([]gin.HandlerFunc)
	for _, handlerFunc := range middlewareFuncList {
		if handlerFunc == nil {
			continue
		}
		engine.Use(handlerFunc)
		receiver.Helper.GetLogger().Info(fmt.Sprintf("注册中间件: %s", receiver.GetFunctionName(handlerFunc)))
	}
	receiver.Helper.GetLogger().Info(fmt.Sprintf("注册中间件: %d 个", len(middlewareFuncList)))

	deps := NewHttpDeps(receiver.Helper, engine)
	header := receiver.Helper.GetConfig().Get("http.router", func(router *gin.Engine, deps *HttpDeps) {

	}).(func(*gin.Engine, *HttpDeps))

	header(engine, deps)

	//receiver.routerFunc(engine)

	// 创建一个HTTP服务器，以便能够优雅关闭
	addr := receiver.Helper.GetConfig().GetString("http.addr", ":8080")
	srv := &http.Server{
		Addr:    addr,
		Handler: engine,
	}

	// 创建一个通道来接收中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	// 在单独的goroutine中启动服务器
	go func() {
		receiver.Helper.GetLogger().Info(fmt.Sprintf("服务器启动于 %s", addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			receiver.Helper.GetLogger().Error(fmt.Sprintf("启动服务器失败: %v", err))
		}
	}()

	// 在单独的goroutine中等待中断信号以便优雅关闭
	go func() {
		// 等待中断信号
		<-quit
		receiver.Helper.GetLogger().Info("正在关闭服务器...")

		// 创建一个5秒的上下文用于超时控制
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 优雅地关闭服务器
		if err := srv.Shutdown(ctx); err != nil {
			receiver.Helper.GetLogger().Error(fmt.Sprintf("服务器强制关闭: %v", err))
		}

		receiver.Helper.GetLogger().Info("服务器已优雅关闭")
	}()
}

// loadTemplates 加载模板到Gin引擎
func (receiver *HttpServer) loadTemplates(engine *gin.Engine) error {
	staticFs := receiver.Helper.GetConfig().Get("static.fs", map[string]embed.FS{}).(map[string]embed.FS)

	templates, ok := staticFs["templates"]

	if !ok {
		return errors.New("没有模板目录需要初始化")
	}

	// 从嵌入的文件系统创建子文件系统
	subFS, err := fs.Sub(templates, "templates")
	if err != nil {
		return err
	}

	parseFS, err := template.New("").ParseFS(subFS, "*.html")

	if err != nil {
		return err
	}

	// 创建模板并解析所有模板文件
	tmpl := template.Must(parseFS, err)

	// 设置HTML模板
	engine.SetHTMLTemplate(tmpl)

	return nil
}

func (receiver *HttpServer) loadWebStatic(engine *gin.Engine) {
	staticHandler := NewStaticHandler(receiver.Helper, engine)
	staticHandler.setupStaticFileServers()
}

// GetFunctionName 获取函数名
func (receiver *HttpServer) GetFunctionName(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

func (receiver *HttpServer) traceIdMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		newUUID, err := uuid.NewV7()
		if err != nil {
			newUUID = uuid.New()
		}
		c.Set("traceId", newUUID.String())
		c.Writer.Header().Set("trace-id", newUUID.String())
		c.Next()
	}
}

func (receiver *HttpServer) ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		traceId := c.GetString("traceId")

		c.Next()

		// 请求处理完成后记录日志
		cost := time.Since(start)
		zapLogger := receiver.Helper.GetLogger()

		// 根据状态码决定日志级别
		statusCode := c.Writer.Status()
		if statusCode >= 500 {
			zapLogger.Error("请求处理",
				NewLoggerField("traceId", traceId),
				NewLoggerField("method", c.Request.Method),
				NewLoggerField("path", path),
				NewLoggerField("query", query),
				NewLoggerField("status", statusCode),
				NewLoggerField("ip", c.ClientIP()),
				NewLoggerField("latency", cost),
				NewLoggerField("user-agent", c.Request.UserAgent()),
				NewLoggerField("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			)
		} else if statusCode >= 400 {
			zapLogger.Warn("请求处理",
				NewLoggerField("traceId", traceId),
				NewLoggerField("method", c.Request.Method),
				NewLoggerField("path", path),
				NewLoggerField("query", query),
				NewLoggerField("status", statusCode),
				NewLoggerField("ip", c.ClientIP()),
				NewLoggerField("latency", cost),
				NewLoggerField("user-agent", c.Request.UserAgent()),
			)
		} else {
			zapLogger.Info("请求处理",
				NewLoggerField("traceId", traceId),
				NewLoggerField("method", c.Request.Method),
				NewLoggerField("path", path),
				NewLoggerField("query", query),
				NewLoggerField("status", statusCode),
				NewLoggerField("ip", c.ClientIP()),
				NewLoggerField("latency", cost),
				NewLoggerField("user-agent", c.Request.UserAgent()),
			)
		}
	}
}
