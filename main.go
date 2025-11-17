package main

import (
	"embed"

	"cnb.cool/mliev/push/message-push/cmd"
)

//go:embed templates/**
var templateFS embed.FS

//go:embed static/**
var staticFs embed.FS

func main() {
	staticFs := map[string]embed.FS{
		"templates":  templateFS,
		"web.static": staticFs,
	}
	cmd.Start(staticFs)
}
