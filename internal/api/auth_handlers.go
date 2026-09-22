package api

import (
	"NeoNect/internal/config"
	"net/http"
	"strings"
)

type UserSignupRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserSignupResponse struct {
	Status string `json:"status"`
	UserID int64  `json:"user_id"`
}

type UserLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type UserLoginResponse struct {
	Status string `json:"status"`
	Token  string `json:"token"`
}

type AccountAvailabilityResponse struct {
	Available bool `json:"available"`
}

func (hm *HandlerManager) CheckUsername(w http.ResponseWriter, r *http.Request) {
	ip, ok := r.Context().Value(clientIPKey).(string)
	if !ok {
		ip = GetClientIP(r, hm.TrustedProxies)
	}
	if !hm.RateLimiter.AllowAuthAttempt(ip) {
		hm.sendError(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("u"))
	if username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	available, err := hm.AuthService.IsUsernameAvailable(r.Context(), username)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.sendJSONResponse(w, AccountAvailabilityResponse{Available: available})
}

func (hm *HandlerManager) RegisterUser(w http.ResponseWriter, r *http.Request) {
	ip, ok := r.Context().Value(clientIPKey).(string)
	if !ok {
		ip = GetClientIP(r, hm.TrustedProxies)
	}
	if !hm.RateLimiter.AllowAuthAttempt(ip) {
		hm.sendError(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	var request UserSignupRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	uid, err := hm.AuthService.Register(r.Context(), request.Username, request.Password)
	if err != nil {
		if err.Error() == "username is already taken" {
			hm.sendError(w, "username is already taken", http.StatusConflict)
		} else {
			hm.sendError(w, err.Error(), http.StatusBadRequest)
		}
		return
	}

	hm.sendJSONResponseWithStatus(w, UserSignupResponse{
		Status: config.StatusSuccess,
		UserID: uid,
	}, http.StatusCreated)
}

func (hm *HandlerManager) LoginUser(w http.ResponseWriter, r *http.Request) {
	ip, ok := r.Context().Value(clientIPKey).(string)
	if !ok {
		ip = GetClientIP(r, hm.TrustedProxies)
	}
	if !hm.RateLimiter.AllowAuthAttempt(ip) {
		hm.sendError(w, "too many requests", http.StatusTooManyRequests)
		return
	}

	var request UserLoginRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	token, err := hm.AuthService.Login(r.Context(), request.Username, request.Password)
	if err != nil {
		hm.sendError(w, err.Error(), http.StatusUnauthorized)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     config.SessionCookieName,
		Value:    token,
		Path:     config.CookiePath,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	hm.sendJSONResponse(w, UserLoginResponse{
		Status: config.StatusSuccess,
		Token:  token,
	})
}

func (hm *HandlerManager) LogoutUser(w http.ResponseWriter, r *http.Request) {
	token, ok := hm.getToken(r)
	if !ok {
		hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
		return
	}

	_ = hm.AuthService.Logout(r.Context(), token)

	http.SetCookie(w, &http.Cookie{
		Name:     config.SessionCookieName,
		Value:    "",
		Path:     config.CookiePath,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	})

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) HandleAuthV1(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		hm.LoginUser(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		hm.LogoutUser(w, r)
		return
	}
	hm.sendError(w, ErrMethodNotAllowed, http.StatusMethodNotAllowed)
}
