package app

import (
	"NeoNect/internal/api"
	"NeoNect/internal/config"
	"context"
	"encoding/json"
	"net/http"
)

const (
	headerContentTypeOptions   = "X-Content-Type-Options"
	headerFrameOptions         = "X-Frame-Options"
	headerXSSProtection        = "X-XSS-Protection"
	headerDownloadOptions      = "X-Download-Options"
	headerReferrerPolicy       = "Referrer-Policy"
	headerAccessControlOrigin  = "Access-Control-Allow-Origin"
	headerAccessControlMethods = "Access-Control-Allow-Methods"
	headerAccessControlHeaders = "Access-Control-Allow-Headers"
	headerAccessControlCreds   = "Access-Control-Allow-Credentials"
	headerHSTS                 = "Strict-Transport-Security"
	headerOrigin               = "Origin"

	valueNoSniff      = "nosniff"
	valueDeny         = "DENY"
	valueXSSBlock     = "1; mode=block"
	valueNoOpen       = "noopen"
	valueSameOrigin   = "same-origin"
	valueMethods      = "GET, POST, PUT, DELETE, OPTIONS"
	valueHeaders      = "Content-Type, Authorization"
	valueTrue         = "true"
	valueHSTS         = "max-age=31536000; includeSubDomains"
	errInternalServer = "Internal Server Error"
)

const (
	apiV1Prefix = "/api/v1"
)

func (app *App) RecoverMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				http.Error(w, errInternalServer, http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (app *App) TimeoutMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), config.RequestContextTimeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (app *App) HeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set(headerContentTypeOptions, valueNoSniff)
		h.Set(headerFrameOptions, valueDeny)
		h.Set(headerXSSProtection, valueXSSBlock)
		h.Set(headerDownloadOptions, valueNoOpen)
		h.Set(headerReferrerPolicy, valueSameOrigin)

		next.ServeHTTP(w, r)
	})
}

func (app *App) HSTSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if api.IsHTTPS(r) {
			w.Header().Set(headerHSTS, valueHSTS)
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) CorsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		if origin := r.Header.Get(headerOrigin); origin != "" {
			allowed := false
			for _, o := range app.Config.AllowedOrigins {
				if o == origin {
					allowed = true
					break
				}
			}
			if allowed {
				h.Set(headerAccessControlOrigin, origin)
			}
		}
		h.Set(headerAccessControlMethods, valueMethods)
		h.Set(headerAccessControlHeaders, valueHeaders)
		h.Set(headerAccessControlCreds, valueTrue)

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) BodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch || r.Method == http.MethodDelete {
			r.Body = http.MaxBytesReader(w, r.Body, config.GlobalMaxBodySize)
		}
		next.ServeHTTP(w, r)
	})
}

func (app *App) withMethod(method string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Content-Type", config.ContentTypeJSON)
			w.WriteHeader(http.StatusMethodNotAllowed)
			_ = json.NewEncoder(w).Encode(api.ErrorResponse{Error: api.ErrMethodNotAllowed})
			return
		}
		handler(w, r)
	}
}

func (app *App) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc(apiV1Prefix+"/auth", app.Handlers.HandleAuthV1)
	mux.HandleFunc(apiV1Prefix+"/users/availability", app.withMethod(http.MethodGet, app.Handlers.CheckUsername))
	mux.HandleFunc(apiV1Prefix+"/users", app.withMethod(http.MethodPost, app.Handlers.RegisterUser))
	mux.HandleFunc(apiV1Prefix+"/users/me", app.withMethod(http.MethodGet, app.Handlers.GetProfile))
	mux.HandleFunc(apiV1Prefix+"/security/verify", app.withMethod(http.MethodGet, app.Handlers.VerifySecuritySystem))

	mux.HandleFunc(apiV1Prefix+"/device/register", app.withMethod(http.MethodPost, app.Handlers.RegisterDevice))
	mux.HandleFunc(apiV1Prefix+"/device/key", app.withMethod(http.MethodGet, app.Handlers.GetDevicePublicKey))
	mux.HandleFunc(apiV1Prefix+"/device", app.withMethod(http.MethodDelete, app.Handlers.RevokeDevice))
	mux.HandleFunc(apiV1Prefix+"/devices", app.withMethod(http.MethodGet, app.Handlers.ListDevices))

	mux.HandleFunc(apiV1Prefix+"/health", app.withMethod(http.MethodGet, app.Handlers.HealthCheck))
	mux.HandleFunc(apiV1Prefix+"/presence", app.withMethod(http.MethodGet, app.Handlers.GetPresence))

	mux.HandleFunc(apiV1Prefix+"/relay/send", app.withMethod(http.MethodPost, app.Handlers.SendRelayMessage))
	mux.HandleFunc(apiV1Prefix+"/relay/poll", app.withMethod(http.MethodGet, app.Handlers.PollRelayMessages))
	mux.HandleFunc(apiV1Prefix+"/relay/ack", app.withMethod(http.MethodPost, app.Handlers.AcknowledgeMessage))
	mux.HandleFunc(apiV1Prefix+"/relay/ws", app.withMethod(http.MethodGet, app.Handlers.HandleWebSocket))

	mux.HandleFunc(apiV1Prefix+"/relay/keys", app.withMethod(http.MethodGet, app.Handlers.GetRecipientKeys))

	mux.HandleFunc(apiV1Prefix+"/keys/upload", app.withMethod(http.MethodPost, app.Handlers.UploadPrekeys))
	mux.HandleFunc(apiV1Prefix+"/keys/claim", app.withMethod(http.MethodPost, app.Handlers.ClaimPrekeys))

	mux.HandleFunc(apiV1Prefix+"/friends", app.Handlers.HandleFriendsV1)
}
