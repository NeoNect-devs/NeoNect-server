package api

import (
	"NeoNect/internal/config"
	"NeoNect/internal/service"
	"errors"
	"net/http"
	"strings"
)

type AddFriendRequest struct {
	Username string `json:"username"`
}

func (hm *HandlerManager) AddFriend(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	if !hm.checkDiscoveryLimit(w, uid) {
		return
	}

	var req AddFriendRequest
	if !hm.parseJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err := hm.FriendService.AddFriend(r.Context(), uid, req.Username)
	if err != nil {
		if err == service.ErrSelfFriendship {
			hm.sendError(w, err.Error(), http.StatusBadRequest)
		} else if err == service.ErrUserNotFound {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else if err == service.ErrDuplicateFriendship || err == service.ErrRequestAlreadySent || err == service.ErrRequestAlreadyReceived {
			hm.sendError(w, err.Error(), http.StatusConflict)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponseWithStatus(w, GenericResponse{Status: config.StatusSuccess}, http.StatusCreated)
}

type GetFriendsResponse struct {
	Status  string   `json:"status"`
	Friends []string `json:"friends"`
}

func (hm *HandlerManager) ListFriends(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	friends, err := hm.FriendService.GetFriends(r.Context(), uid)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.sendJSONResponse(w, GetFriendsResponse{
		Status:  config.StatusSuccess,
		Friends: friends,
	})
}

func (hm *HandlerManager) RemoveFriend(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	var req AddFriendRequest
	if !hm.parseJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err := hm.FriendService.RemoveFriend(r.Context(), uid, req.Username)
	if err != nil {
		if err == service.ErrUserNotFound {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) HandleFriendsV1(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		hm.ListFriends(w, r)
		return
	}
	if r.Method == http.MethodPost {
		hm.AddFriend(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		hm.RemoveFriend(w, r)
		return
	}
	hm.sendError(w, ErrMethodNotAllowed, http.StatusMethodNotAllowed)
}

type GetRequestsResponse struct {
	Status string                    `json:"status"`
	Data   service.FriendRequestList `json:"data"`
}

func (hm *HandlerManager) ListRequests(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	list, err := hm.FriendService.GetRequests(r.Context(), uid)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.sendJSONResponse(w, GetRequestsResponse{
		Status: config.StatusSuccess,
		Data:   list,
	})
}

func (hm *HandlerManager) AcceptRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	var req AddFriendRequest
	if !hm.parseJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err := hm.FriendService.AcceptRequest(r.Context(), uid, req.Username)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrRequestNotFound) {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else if errors.Is(err, service.ErrUnauthorizedAction) {
			hm.sendError(w, err.Error(), http.StatusForbidden)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) DeclineRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	var req AddFriendRequest
	if !hm.parseJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err := hm.FriendService.DeclineRequest(r.Context(), uid, req.Username)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrRequestNotFound) {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else if errors.Is(err, service.ErrUnauthorizedAction) {
			hm.sendError(w, err.Error(), http.StatusForbidden)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) CancelRequest(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}

	var req AddFriendRequest
	if !hm.parseJSON(w, r, &req) {
		return
	}

	req.Username = strings.TrimSpace(req.Username)
	if req.Username == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err := hm.FriendService.CancelRequest(r.Context(), uid, req.Username)
	if err != nil {
		if errors.Is(err, service.ErrUserNotFound) || errors.Is(err, service.ErrRequestNotFound) {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else if errors.Is(err, service.ErrUnauthorizedAction) {
			hm.sendError(w, err.Error(), http.StatusForbidden)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) HandleRequestsV1(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		hm.ListRequests(w, r)
		return
	}
	if r.Method == http.MethodDelete {
		hm.CancelRequest(w, r)
		return
	}
	hm.sendError(w, ErrMethodNotAllowed, http.StatusMethodNotAllowed)
}
