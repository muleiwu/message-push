package main

import (
	"embed"

	"cnb.cool/mliev/open/go-web/cmd"
	mpConfig "cnb.cool/mliev/push/message-push/config"
	"github.com/muleiwu/gomander"
)

//go:embed templates
var templateFS embed.FS

//go:embed static
var staticFs embed.FS

func main() {
	gomander.Run(func() {
		cmd.Start(
			cmd.WithTemplateFs(templateFS),
			cmd.WithWebStaticFs(staticFs),
			cmd.WithApp(mpConfig.App{}),
		)
	})
}
