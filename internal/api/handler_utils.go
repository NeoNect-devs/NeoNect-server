package api

import (
	"net"

	"NeoNect/internal/config"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

type contextKey string

const clientIPKey contextKey = "clientIP"

func (hm *HandlerManager) RateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := GetClientIP(r, hm.TrustedProxies)

		if !hm.RateLimiter.AllowConnection(ip) {
			hm.sendError(w, "too many connections", http.StatusTooManyRequests)
			return
		}
		defer hm.RateLimiter.ReleaseConnection(ip)

		if !hm.RateLimiter.AllowIPRequest(ip) {
			hm.sendError(w, "too many requests", http.StatusTooManyRequests)
			return
		}

		ctx := context.WithValue(r.Context(), clientIPKey, ip)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (hm *HandlerManager) checkMessageLimit(w http.ResponseWriter, uid int64) bool {
	if !hm.RateLimiter.AllowMessage(uid) {
		hm.sendError(w, "too many messages", http.StatusTooManyRequests)
		return false
	}
	return true
}

func (hm *HandlerManager) requireSession(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid, err := hm.getSessionUser(r.Context(), r)
	if err != nil {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return 0, false
	}
	return uid, true
}

func (hm *HandlerManager) getSessionUser(ctx context.Context, r *http.Request) (int64, error) {
	token, ok := hm.getToken(r)
	if !ok {
		return 0, http.ErrNoCookie
	}
	return hm.AuthService.GetUserIdByToken(ctx, token)
}

func (hm *HandlerManager) getToken(r *http.Request) (string, bool) {
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer "), true
	}

	cookie, err := r.Cookie(config.SessionCookieName)
	if err == nil {
		return cookie.Value, true
	}

	return "", false
}

func (hm *HandlerManager) sendJSONResponse(w http.ResponseWriter, data any) {
	hm.sendJSONResponseWithStatus(w, data, http.StatusOK)
}

func (hm *HandlerManager) sendJSONResponseWithStatus(w http.ResponseWriter, data any, code int) {
	payload, err := json.Marshal(data)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", config.ContentTypeJSON)
	w.WriteHeader(code)
	_, _ = w.Write(payload)
}

func (hm *HandlerManager) sendError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", config.ContentTypeJSON)
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func (hm *HandlerManager) checkDiscoveryLimit(w http.ResponseWriter, uid int64) bool {
	if !hm.RateLimiter.AllowDiscovery(uid) {
		hm.sendError(w, "too many discovery requests", http.StatusTooManyRequests)
		return false
	}
	return true
}

func ParseTrustedProxies(proxies []string) []*net.IPNet {
	var parsed []*net.IPNet
	for _, p := range proxies {
		if !strings.Contains(p, "/") {
			if strings.Contains(p, ":") {
				p = p + "/128"
			} else {
				p = p + "/32"
			}
		}
		_, ipNet, err := net.ParseCIDR(p)
		if err == nil {
			parsed = append(parsed, ipNet)
		}
	}
	return parsed
}

func isTrustedProxy(ip string, trustedProxies []*net.IPNet) bool {
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	for _, network := range trustedProxies {
		if network.Contains(parsedIP) {
			return true
		}
	}
	return false
}

func GetClientIP(r *http.Request, trustedProxies []*net.IPNet) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		ip = r.RemoteAddr
	}

	ip = strings.TrimSpace(ip)

	if len(trustedProxies) == 0 || !isTrustedProxy(ip, trustedProxies) {
		return ip
	}

	xff := r.Header.Get("X-Forwarded-For")
	if xff == "" {
		xRealIP := r.Header.Get("X-Real-IP")
		if xRealIP != "" {
			realIP := strings.TrimSpace(xRealIP)
			if net.ParseIP(realIP) != nil {
				return realIP
			}
		}
		return ip
	}

	parts := strings.Split(xff, ",")

	for i := len(parts) - 1; i >= 0; i-- {
		part := strings.TrimSpace(parts[i])
		if net.ParseIP(part) == nil {
			continue
		}
		if isTrustedProxy(part, trustedProxies) {
			continue
		}
		return part
	}

	// return the leftmost valid ip if all proxies are trusted
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if net.ParseIP(part) != nil {
			return part
		}
	}

	return ip
}

func IsHTTPS(r *http.Request) bool {
	return r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
}

func (hm *HandlerManager) parseJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			hm.sendError(w, "request entity too large", http.StatusRequestEntityTooLarge)
		} else {
			hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		}
		return false
	}
	return true
}
