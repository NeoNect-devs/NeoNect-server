package api

import (
	"NeoNect/internal/config"
	"NeoNect/security"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"strings"
)

type DeviceEnrollmentRequest struct {
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

type DeviceKeyResponse struct {
	DeviceID  string `json:"device_id"`
	PublicKey string `json:"public_key"`
}

func (hm *HandlerManager) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	var request DeviceEnrollmentRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	if request.DeviceID == "" || request.PublicKey == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	pubKey, err := base64.StdEncoding.DecodeString(request.PublicKey)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	if err := hm.DeviceService.RegisterDevice(r.Context(), uid, request.DeviceID, pubKey); err != nil {
		if err.Error() == "maximum device limit reached" {
			hm.sendError(w, err.Error(), http.StatusBadRequest)
		} else if err.Error() == "conflict" {
			hm.sendError(w, err.Error(), http.StatusConflict)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponseWithStatus(w, GenericResponse{Status: config.StatusSuccess}, http.StatusCreated)
}

func (hm *HandlerManager) GetDevicePublicKey(w http.ResponseWriter, r *http.Request) {
	_, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	pubKey, err := hm.DeviceService.GetDevicePublicKey(r.Context(), deviceID)
	if err != nil {
		hm.sendError(w, ErrNotFound, http.StatusNotFound)
		return
	}

	hm.sendJSONResponse(w, DeviceKeyResponse{
		DeviceID:  deviceID,
		PublicKey: base64.StdEncoding.EncodeToString(pubKey),
	})
}

type RecipientKeysResponse struct {
	Devices []DeviceKeyResponse `json:"devices"`
}

type DeviceRevocationRequest struct {
	DeviceID string `json:"device_id"`
}

func (hm *HandlerManager) GetRecipientKeys(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	if !hm.checkDiscoveryLimit(w, uid) {
		return
	}

	username := strings.TrimSpace(r.URL.Query().Get("u"))
	if username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	targetHash := security.ComputeHash(username)
	targetUserID, err := hm.UserRepo.GetUserIdByUsername(r.Context(), targetHash)
	if err != nil {
		if err.Error() == "user not found" || errors.Is(err, sql.ErrNoRows) {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	if targetUserID != uid {
		isFriend, err := hm.FriendService.AreFriends(r.Context(), uid, targetUserID)
		if err != nil || !isFriend {
			hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
			return
		}
	}

	devices, err := hm.DeviceService.GetRecipientKeys(r.Context(), username)
	if err != nil {
		if err.Error() == "user not found" || errors.Is(err, sql.ErrNoRows) {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	resp := RecipientKeysResponse{
		Devices: make([]DeviceKeyResponse, 0),
	}

	for _, d := range devices {
		resp.Devices = append(resp.Devices, DeviceKeyResponse{
			DeviceID:  d.DeviceID,
			PublicKey: base64.StdEncoding.EncodeToString(d.PublicKey),
		})
	}

	hm.sendJSONResponse(w, resp)
}

func (hm *HandlerManager) RevokeDevice(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	var request DeviceRevocationRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	if request.DeviceID == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	isOwned, err := hm.DeviceService.ValidateDeviceOwnership(r.Context(), uid, request.DeviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}
	if !isOwned {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	if err := hm.DeviceService.DeleteDevice(r.Context(), uid, request.DeviceID); err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.WSManager.DisconnectDevice(request.DeviceID)

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

type ListDevicesResponse struct {
	Status  string   `json:"status"`
	Devices []string `json:"devices"`
}

func (hm *HandlerManager) ListDevices(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	devices, err := hm.DeviceService.GetDevicesByUser(r.Context(), uid)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	if devices == nil {
		devices = []string{}
	}

	hm.sendJSONResponse(w, ListDevicesResponse{
		Status:  config.StatusSuccess,
		Devices: devices,
	})
}
