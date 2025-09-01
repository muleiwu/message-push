package main

import (
	"embed"

	"cnb.cool/mliev/examples/go-web/cmd"
)

func main() {
	staticFs := map[string]embed.FS{}
	cmd.Start(staticFs)
}
