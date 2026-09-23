package app

import (
	"NeoNect/internal/config"
	"context"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
)

const (
	msgNodeAddress  = "Node address: %s"
	msgMode         = "Mode: RELAY_ONLY"
	msgApiBasePath  = "API Base Path: %s"
	msgListenerErr  = "Failed to create listener: %v"
	msgServerErr    = "Server critical failure: %v"
	msgShutdownInit = "Signal %v: Graceful shutdown initiated"
)

func (app *App) createListener() net.Listener {
	l, err := net.Listen(NetworkTCP, app.Config.BindAddress)
	if err != nil {
		app.Logger.Fatalf(msgListenerErr, err)
	}
	return l
}

func (app *App) startServer(l net.Listener, handler http.Handler) *http.Server {
	srv := &http.Server{
		Handler:      handler,
		ReadTimeout:  app.Config.ReadTimeout,
		WriteTimeout: app.Config.WriteTimeout,
		IdleTimeout:  app.Config.IdleTimeout,
		ErrorLog:     log.New(io.Discard, "", 0),
	}

	go func() {
		var err error
		certPath := filepath.Join(app.Config.MasterKeyDir, app.Config.CertFilename)
		keyPath := filepath.Join(app.Config.MasterKeyDir, app.Config.KeyFilename)

		if app.fileExists(certPath) && app.fileExists(keyPath) {
			err = srv.ServeTLS(l, certPath, keyPath)
		} else {
			err = srv.Serve(l)
		}
		if err != nil && err != http.ErrServerClosed {
			app.Logger.Fatalf(msgServerErr, err)
		}
	}()

	return srv
}

func (app *App) fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (app *App) handleGracefulShutdown(sigChan chan os.Signal, srv *http.Server) {
	sig := <-sigChan
	app.Logger.Infof(msgShutdownInit, sig)

	ctx, cancel := context.WithTimeout(context.Background(), config.ShutdownTimeout)
	defer cancel()

	if app.Handlers != nil && app.Handlers.WSManager != nil {
		app.Handlers.WSManager.Shutdown()
	}

	if err := srv.Shutdown(ctx); err != nil {
		app.Logger.Warnf("Server forced to shutdown: %v", err)
	}

	app.Close(ctx)
	app.Logger.Infof("Graceful shutdown completed")
}

func (app *App) logAccessPoints(addr string) {
	_, port, _ := net.SplitHostPort(addr)

	app.Logger.Infof("Listening")
	app.Logger.Infof("  Bind Address:    %s", addr)
	app.Logger.Infof("  Configured Port: %s", port)
	app.Logger.Infof("  Assigned Port:   %s", port)

	primary := getPrimaryLocalAddress(port)
	app.Logger.Infof("  Primary Local Address: %s", primary)

	nodeAddr := "unavailable"
	// node address is only available if explicitly set via env
	if envNode := os.Getenv("NEONECT_NODE_ADDR"); envNode != "" {
		nodeAddr = envNode
	}
	app.Logger.Infof("  Node Address:    %s", nodeAddr)
}

func getPrimaryLocalAddress(port string) string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unavailable"
	}

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ip := ipnet.IP.To4(); ip != nil {
				if isPrivateIP(ip) {
					return net.JoinHostPort(ip.String(), port)
				}
			}
		}
	}
	return "unavailable"
}

func isPrivateIP(ip net.IP) bool {
	return (ip[0] == 10) ||
		(ip[0] == 172 && (ip[1] >= 16 && ip[1] <= 31)) ||
		(ip[0] == 192 && ip[1] == 168)
}
