package api

import (
	"NeoNect/internal/config"
	"NeoNect/internal/service"
	"encoding/base64"
	"errors"
	"net/http"
)

type RelayMessageRequest struct {
	MessageID         string `json:"message_id,omitempty"`
	FromDeviceID      string `json:"from_device_id"`
	RecipientDeviceID string `json:"recipient_device_id,omitempty"`
	ProtocolVersion   *int   `json:"protocol_version,omitempty"`
	ToUsername        string `json:"to_username,omitempty"`
	Ciphertext        string `json:"ciphertext"`
	Timestamp         int64  `json:"timestamp,omitempty"`
}

type RelayMessageResponse struct {
	Status string `json:"status"`
}

type DeliveryQueueResponse struct {
	Messages []DeliveryMessage `json:"messages"`
}

type DeliveryMessage struct {
	ID              int64   `json:"id"`
	MessageID       *string `json:"message_id,omitempty"`
	SenderDeviceID  *string `json:"sender_device_id,omitempty"`
	ProtocolVersion *int    `json:"protocol_version,omitempty"`
	Ciphertext      string  `json:"ciphertext"`
}

type AcknowledgeRequest struct {
	DeviceID  string `json:"device_id"`
	MessageID int64  `json:"message_id"`
}

func (hm *HandlerManager) SendRelayMessage(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	var request RelayMessageRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	if request.FromDeviceID == "" || request.Ciphertext == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	if request.ProtocolVersion == nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	if *request.ProtocolVersion == 2 {
		if request.RecipientDeviceID == "" || request.MessageID == "" {
			hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
			return
		}
	} else if *request.ProtocolVersion == 1 {
		if request.ToUsername == "" {
			hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
			return
		}
	} else {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	isOwned, err := hm.DeviceService.ValidateActiveDeviceOwnership(r.Context(), uid, request.FromDeviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}
	if !isOwned {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	cipher, err := base64.StdEncoding.DecodeString(request.Ciphertext)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	if *request.ProtocolVersion == 2 {
		env := service.MessageEnvelope{
			MessageID:         request.MessageID,
			SenderDeviceID:    request.FromDeviceID,
			RecipientDeviceID: request.RecipientDeviceID,
			ProtocolVersion:   *request.ProtocolVersion,
			Ciphertext:        cipher,
		}
		if err := hm.RelayService.SendEnvelope(r.Context(), uid, env); err != nil {
			if err.Error() == "unauthorized sender" {
				hm.sendError(w, ErrUnauthorized, http.StatusForbidden)
			} else if errors.Is(err, service.ErrRecipientDeviceNotFound) {
				hm.sendError(w, ErrNotFound, http.StatusNotFound)
			} else if errors.Is(err, service.ErrDuplicateConflict) {
				hm.sendError(w, "conflict", http.StatusConflict)
			} else if err.Error() == "mailbox quota exceeded" {
				hm.sendError(w, "quota exceeded", http.StatusRequestEntityTooLarge)
			} else {
				hm.sendError(w, ErrServer, http.StatusInternalServerError)
			}
			return
		}
	} else if *request.ProtocolVersion == 1 {
		if err := hm.RelayService.Send(r.Context(), uid, request.ToUsername, cipher); err != nil {
			if err.Error() == "unauthorized sender" {
				hm.sendError(w, ErrUnauthorized, http.StatusForbidden)
			} else if errors.Is(err, service.ErrRecipientNotFound) {
				hm.sendError(w, ErrNotFound, http.StatusNotFound)
			} else if errors.Is(err, service.ErrRecipientNoDevices) {
				hm.sendError(w, "recipient has no registered devices", http.StatusUnprocessableEntity)
			} else {
				hm.sendError(w, ErrServer, http.StatusInternalServerError)
			}
			return
		}
	}

	hm.sendJSONResponseWithStatus(w, RelayMessageResponse{Status: config.StatusSuccess}, http.StatusCreated)
}

func (hm *HandlerManager) PollRelayMessages(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	isOwned, err := hm.DeviceService.ValidateActiveDeviceOwnership(r.Context(), uid, deviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}
	if !isOwned {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	items, err := hm.RelayService.GetQueue(r.Context(), deviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	resp := DeliveryQueueResponse{Messages: []DeliveryMessage{}}
	for _, item := range items {
		resp.Messages = append(resp.Messages, DeliveryMessage{
			ID:              item.ID,
			MessageID:       item.MessageID,
			SenderDeviceID:  item.SenderDeviceID,
			ProtocolVersion: item.ProtocolVersion,
			Ciphertext:      base64.StdEncoding.EncodeToString(item.Payload),
		})
	}

	hm.sendJSONResponse(w, resp)
}

func (hm *HandlerManager) AcknowledgeMessage(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	var request AcknowledgeRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	isOwned, err := hm.DeviceService.ValidateActiveDeviceOwnership(r.Context(), uid, request.DeviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}
	if !isOwned {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	if err := hm.RelayService.Acknowledge(r.Context(), request.DeviceID, request.MessageID); err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func (hm *HandlerManager) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	deviceID := r.URL.Query().Get("device_id")
	if deviceID == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	isOwned, err := hm.DeviceService.ValidateActiveDeviceOwnership(r.Context(), uid, deviceID)
	if err != nil {
		hm.sendError(w, ErrServer, http.StatusInternalServerError)
		return
	}
	if !isOwned {
		hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		return
	}

	hm.WSManager.HandleConnection(w, r, deviceID)
}
