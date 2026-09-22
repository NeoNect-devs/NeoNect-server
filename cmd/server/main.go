package main

import (
	"path/filepath"

	"NeoNect/internal/app"
	"NeoNect/internal/config"
)

func main() {
	cfg := config.LoadConfig()
	secretPath := filepath.Join(cfg.MasterKeyDir, cfg.MasterKeyFilename)

	application := app.NewApp(secretPath)
	application.Run()
}
