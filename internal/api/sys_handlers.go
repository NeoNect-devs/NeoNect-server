package api

import (
	"NeoNect/internal/config"
	"NeoNect/security"
	"net/http"
)

type UserProfileResponse struct {
	Username string `json:"username"`
}

func (hm *HandlerManager) GetProfile(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	username, err := hm.UserRepo.GetUsernameById(r.Context(), uid)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.sendJSONResponse(w, UserProfileResponse{Username: username})
}

func (hm *HandlerManager) VerifySecuritySystem(w http.ResponseWriter, r *http.Request) {
	_, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	if !hm.Vault.VerifyIntegrity(config.IntegrityCheckSeed) {
		hm.sendError(w, "vault integrity check failed", http.StatusForbidden)
		return
	}

	if !hm.IntegrityRepo.VerifyDatabase(r.Context()) {
		hm.sendError(w, "database integrity check failed", http.StatusForbidden)
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) HealthCheck(w http.ResponseWriter, r *http.Request) {
	if !hm.IntegrityRepo.VerifyDatabase(r.Context()) {
		hm.sendError(w, "database unavailable", http.StatusServiceUnavailable)
		return
	}
	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

type PresenceResponse struct {
	Online bool `json:"online"`
}

func (hm *HandlerManager) GetPresence(w http.ResponseWriter, r *http.Request) {
	_, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	targetUser := r.URL.Query().Get("u")
	if targetUser == "" {
		hm.sendError(w, "missing username parameter", http.StatusBadRequest)
		return
	}

	usernameHash := security.ComputeHash(targetUser)

	userID, err := hm.UserRepo.GetUserIdByUsername(r.Context(), usernameHash)
	if err != nil {
		// return offline for unknown users to prevent enumeration
		hm.sendJSONResponse(w, PresenceResponse{Online: false})
		return
	}

	devices, err := hm.DeviceService.GetDevicesByUser(r.Context(), userID)
	if err != nil {
		hm.sendJSONResponse(w, PresenceResponse{Online: false})
		return
	}

	online := false
	for _, dev := range devices {
		if hm.WSManager.IsDeviceOnline(dev) {
			online = true
			break
		}
	}

	hm.sendJSONResponse(w, PresenceResponse{Online: online})
}
