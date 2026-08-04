package main

import (
	"embed"
	"time"

	"cnb.cool/mliev/open/go-web/cmd"
	mpConfig "cnb.cool/mliev/push/message-push/config"
	"github.com/muleiwu/gomander"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFs embed.FS

func main() {
	// Persistence and framework defaults use UTC. Business calendar operations
	// explicitly use Asia/Shanghai through internal/timeutil.
	time.Local = time.UTC
	gomander.Run(func() {
		cmd.Start(
			cmd.WithTemplateFs(templateFS),
			cmd.WithWebStaticFs(staticFs),
			cmd.WithApp(mpConfig.App{}),
		)
	})
}
