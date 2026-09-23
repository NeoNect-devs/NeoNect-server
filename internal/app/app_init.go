package app

import (
	"NeoNect/internal/api"
	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
	"NeoNect/internal/service"
	"NeoNect/security"
	"context"
	"crypto/rand"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

const (
	NetworkTCP = "tcp"
)

type App struct {
	DB          *persistence.Database
	SessionRepo persistence.SessionRepository
	Handlers    *api.HandlerManager
	Config      config.AppConfig
	Logger      *AppLogger
}

func (app *App) bootstrap(secretPath string) {
	app.ensureStoragePath()
	app.ensureMasterKey(secretPath)

	vault, err := security.NewSystemVault(secretPath)
	if err != nil {
		app.Logger.Fatalf("Vault Initialization Failed: %v", err)
	}

	dbManager, err := app.setupDatabase(vault)
	if err != nil {
		app.Logger.Fatalf("Database Initialization Failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), config.DatabaseInitializationWait)
	defer cancel()

	if err := dbManager.Initialize(ctx); err != nil {
		app.Logger.Fatalf("Database Schema Error: %v", err)
	}
	app.DB = dbManager

	userRepo := persistence.NewUserRepository(dbManager.DB())
	sessionRepo := persistence.NewSessionRepository(dbManager.DB())
	deviceRepo := persistence.NewDeviceRepository(dbManager.DB())
	queueRepo := persistence.NewDeliveryQueueRepository(dbManager.DB())
	integrityRepo := persistence.NewIntegrityRepository(dbManager.DB())
	friendRepo := persistence.NewFriendshipRepository(dbManager.DB())

	prekeyRepo := persistence.NewPrekeyRepository(dbManager.DB())

	app.SessionRepo = sessionRepo

	notifier := service.NewNoopNotifier()
	wsManager := service.NewWebSocketManager(app.Config.AllowedOrigins)
	pushAdapter := service.NewPushAdapter()

	authSvc := service.NewAuthService(userRepo, sessionRepo)
	deviceSvc := service.NewDeviceService(userRepo, deviceRepo, vault, app.Config.MaxDevicesPerUser)
	rs := service.NewRelayService(userRepo, deviceRepo, queueRepo, friendRepo, notifier, wsManager, pushAdapter)
	friendSvc := service.NewFriendService(userRepo, friendRepo)
	prekeySvc := service.NewPrekeyService(deviceRepo, prekeyRepo, friendRepo, vault)

	app.Handlers = api.NewHandlerManager(authSvc, rs, wsManager, deviceSvc, friendSvc, prekeySvc, userRepo, integrityRepo, vault, app.Config.TrustedProxies)
}

func NewApp(secretPath string) *App {
	appConfig := config.LoadConfig()
	app := &App{
		Config: appConfig,
		Logger: newAppLogger(appConfig.Debug),
	}
	app.bootstrap(secretPath)
	return app
}

func (app *App) Run() {
	mux := http.NewServeMux()
	app.RegisterRoutes(mux)

	listener := app.createListener()
	app.logAccessPoints(listener.Addr().String())

	mw := app.RecoverMiddleware(
		app.TimeoutMiddleware(
			app.HeadersMiddleware(
				app.CorsMiddleware(
					app.BodyLimitMiddleware(
						app.HSTSMiddleware(
							app.Handlers.RateLimitMiddleware(mux),
						),
					),
				),
			),
		),
	)

	srv := app.startServer(listener, mw)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	app.handleGracefulShutdown(sigChan, srv)
}

func (app *App) Close(ctx context.Context) {
	if app.Handlers != nil {
		if app.Handlers.RelayService != nil {
			app.Handlers.RelayService.Shutdown(ctx)
		}
		if app.Handlers.RateLimiter != nil {
			app.Handlers.RateLimiter.Stop()
		}
	}
	if app.SessionRepo != nil {
		app.SessionRepo.Shutdown()
	}
	if app.DB != nil {
		_ = app.DB.Close()
	}
}

func (app *App) ensureStoragePath() {
	storageRoot := config.GetStoragePath()

	dirs := []string{app.Config.DatabaseDir, app.Config.MasterKeyDir}
	for _, dir := range dirs {
		if app.Config.Environment != "development" {
			if err := config.ValidateStoragePath(dir, storageRoot); err != nil {
				app.Logger.Fatalf("Critical Misconfiguration: Storage directory %s is outside the project root storage (%s). Error: %v", dir, storageRoot, err)
			}
		}

		if err := os.MkdirAll(dir, os.FileMode(config.DirPerm)); err != nil {
			app.Logger.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}
}

func (app *App) ensureMasterKey(path string) {
	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		key := make([]byte, config.MasterKeySize)
		if _, err := rand.Read(key); err != nil {
			app.Logger.Fatalf("Entropy Failure: %v", err)
		}

		if err := security.CreateMasterKey(path, key); err != nil {
			app.Logger.Fatalf("Failed to seal master key: %v", err)
		}
	}
}

func (app *App) setupDatabase(vault *security.SystemVault) (*persistence.Database, error) {
	dbPath := filepath.Join(app.Config.DatabaseDir, vault.Hash(app.Config.DatabaseName)+config.SqliteExt)
	app.Logger.Infof("Database path: %s", dbPath)
	return persistence.NewDatabase(dbPath)
}
