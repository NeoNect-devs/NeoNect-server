package api

import (
	"NeoNect/internal/config"
	"NeoNect/internal/infrastructure/persistence"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

type PrekeyUploadRequest struct {
	DeviceID            string          `json:"device_id"`
	SignedCurvePrekey   *SignedPrekey   `json:"signed_curve_prekey,omitempty"`
	OneTimeCurvePrekeys []OneTimePrekey `json:"one_time_curve_prekeys,omitempty"`
	SignedPQPrekey      *SignedPrekey   `json:"signed_pq_prekey,omitempty"`
	OneTimePQPrekeys    []OneTimePrekey `json:"one_time_pq_prekeys,omitempty"`
}

type SignedPrekey struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // base64
	Signature string `json:"signature"`  // base64
}

type OneTimePrekey struct {
	KeyID     uint32 `json:"key_id"`
	PublicKey string `json:"public_key"` // base64
}

type PrekeyClaimRequest struct {
	TargetUser   string `json:"target_user"`
	TargetDevice string `json:"target_device"`
}

type PrekeyBundleResponse struct {
	IdentityKey        string         `json:"identity_key"` // base64
	SignedCurvePrekey  *SignedPrekey  `json:"signed_curve_prekey,omitempty"`
	SignedPQPrekey     *SignedPrekey  `json:"signed_pq_prekey,omitempty"`
	OneTimeCurvePrekey *OneTimePrekey `json:"one_time_curve_prekey,omitempty"`
	OneTimePQPrekey    *OneTimePrekey `json:"one_time_pq_prekey,omitempty"`
}

func (hm *HandlerManager) UploadPrekeys(w http.ResponseWriter, r *http.Request) {
	uid, ok := hm.requireSession(w, r)
	if !ok {
		return
	}
	if !hm.checkMessageLimit(w, uid) {
		return
	}

	var request PrekeyUploadRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	if request.DeviceID == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	signedCurve, err := decodeSignedPrekey(request.SignedCurvePrekey)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	signedPQ, err := decodeSignedPrekey(request.SignedPQPrekey)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	oneTimeCurve, err := decodeOneTimePrekeys(request.OneTimeCurvePrekeys)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	oneTimePQ, err := decodeOneTimePrekeys(request.OneTimePQPrekeys)
	if err != nil {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	err = hm.PrekeyService.UploadPrekeys(
		r.Context(),
		uid,
		request.DeviceID,
		signedCurve,
		oneTimeCurve,
		signedPQ,
		oneTimePQ,
	)

	if err != nil {
		if err.Error() == "unauthorized device access" {
			hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		} else if err.Error() == "prekey limit exceeded" || err.Error() == "invalid prekey batch" {
			hm.sendError(w, err.Error(), http.StatusBadRequest)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	hm.sendJSONResponse(w, GenericResponse{Status: config.StatusSuccess})
}

func decodeSignedPrekey(spk *SignedPrekey) (*persistence.SignedPrekeyRecord, error) {
	if spk == nil {
		return nil, nil
	}
	pub, err := base64.StdEncoding.DecodeString(spk.PublicKey)
	if err != nil {
		return nil, err
	}
	sig, err := base64.StdEncoding.DecodeString(spk.Signature)
	if err != nil {
		return nil, err
	}
	return &persistence.SignedPrekeyRecord{
		KeyID:     spk.KeyID,
		PublicKey: pub,
		Signature: sig,
	}, nil
}

func decodeOneTimePrekeys(opks []OneTimePrekey) ([]persistence.OneTimePrekeyRecord, error) {
	var records []persistence.OneTimePrekeyRecord
	for _, p := range opks {
		pub, err := base64.StdEncoding.DecodeString(p.PublicKey)
		if err != nil {
			return nil, err
		}
		records = append(records, persistence.OneTimePrekeyRecord{
			KeyID:     p.KeyID,
			PublicKey: pub,
		})
	}
	return records, nil
}

func (hm *HandlerManager) ClaimPrekeys(w http.ResponseWriter, r *http.Request) {
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

	var request PrekeyClaimRequest
	if !hm.parseJSON(w, r, &request) {
		return
	}

	if request.TargetDevice == "" {
		hm.sendError(w, ErrBadRequest, http.StatusBadRequest)
		return
	}

	idempotencyKey := r.Header.Get("Idempotency-Key")

	if idempotencyKey != "" {
		reqBytes, _ := json.Marshal(request)
		fingerprintHash := sha256.Sum256(reqBytes)
		fingerprintStr := hex.EncodeToString(fingerprintHash[:])

		payloadBytes, status, err := hm.PrekeyService.ClaimPrekeysIdempotent(
			r.Context(),
			uid,
			request.TargetDevice,
			idempotencyKey,
			fingerprintStr,
			func(bundle *persistence.PrekeyBundle) ([]byte, int, error) {
				response := PrekeyBundleResponse{
					IdentityKey: base64.StdEncoding.EncodeToString(bundle.IdentityKey),
				}

				if bundle.SignedCurvePrekey != nil {
					response.SignedCurvePrekey = &SignedPrekey{
						KeyID:     bundle.SignedCurvePrekey.KeyID,
						PublicKey: base64.StdEncoding.EncodeToString(bundle.SignedCurvePrekey.PublicKey),
						Signature: base64.StdEncoding.EncodeToString(bundle.SignedCurvePrekey.Signature),
					}
				}

				if bundle.SignedPQPrekey != nil {
					response.SignedPQPrekey = &SignedPrekey{
						KeyID:     bundle.SignedPQPrekey.KeyID,
						PublicKey: base64.StdEncoding.EncodeToString(bundle.SignedPQPrekey.PublicKey),
						Signature: base64.StdEncoding.EncodeToString(bundle.SignedPQPrekey.Signature),
					}
				}

				if bundle.OneTimeCurvePrekey != nil {
					response.OneTimeCurvePrekey = &OneTimePrekey{
						KeyID:     bundle.OneTimeCurvePrekey.KeyID,
						PublicKey: base64.StdEncoding.EncodeToString(bundle.OneTimeCurvePrekey.PublicKey),
					}
				}

				if bundle.OneTimePQPrekey != nil {
					response.OneTimePQPrekey = &OneTimePrekey{
						KeyID:     bundle.OneTimePQPrekey.KeyID,
						PublicKey: base64.StdEncoding.EncodeToString(bundle.OneTimePQPrekey.PublicKey),
					}
				}

				b, err := json.Marshal(response)
				return b, http.StatusOK, err
			},
		)

		if err != nil {
			if strings.Contains(err.Error(), "unauthorized prekey claim") {
				hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
			} else if strings.Contains(err.Error(), "device not found or inactive") {
				hm.sendError(w, ErrNotFound, http.StatusNotFound)
			} else if strings.Contains(err.Error(), "idempotency conflict") {
				hm.sendError(w, "Conflict", http.StatusConflict)
			} else {
				hm.sendError(w, ErrServer, http.StatusInternalServerError)
			}
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write(payloadBytes)
		return
	}

	bundle, err := hm.PrekeyService.ClaimPrekeys(r.Context(), uid, request.TargetDevice)
	if err != nil {
		if err.Error() == "unauthorized prekey claim" {
			hm.sendError(w, ErrUnauthorized, http.StatusUnauthorized)
		} else if err.Error() == "device not found or inactive" {
			hm.sendError(w, ErrNotFound, http.StatusNotFound)
		} else {
			hm.sendError(w, ErrServer, http.StatusInternalServerError)
		}
		return
	}

	response := PrekeyBundleResponse{
		IdentityKey: base64.StdEncoding.EncodeToString(bundle.IdentityKey),
	}

	if bundle.SignedCurvePrekey != nil {
		response.SignedCurvePrekey = &SignedPrekey{
			KeyID:     bundle.SignedCurvePrekey.KeyID,
			PublicKey: base64.StdEncoding.EncodeToString(bundle.SignedCurvePrekey.PublicKey),
			Signature: base64.StdEncoding.EncodeToString(bundle.SignedCurvePrekey.Signature),
		}
	}

	if bundle.SignedPQPrekey != nil {
		response.SignedPQPrekey = &SignedPrekey{
			KeyID:     bundle.SignedPQPrekey.KeyID,
			PublicKey: base64.StdEncoding.EncodeToString(bundle.SignedPQPrekey.PublicKey),
			Signature: base64.StdEncoding.EncodeToString(bundle.SignedPQPrekey.Signature),
		}
	}

	if bundle.OneTimeCurvePrekey != nil {
		response.OneTimeCurvePrekey = &OneTimePrekey{
			KeyID:     bundle.OneTimeCurvePrekey.KeyID,
			PublicKey: base64.StdEncoding.EncodeToString(bundle.OneTimeCurvePrekey.PublicKey),
		}
	}

	if bundle.OneTimePQPrekey != nil {
		response.OneTimePQPrekey = &OneTimePrekey{
			KeyID:     bundle.OneTimePQPrekey.KeyID,
			PublicKey: base64.StdEncoding.EncodeToString(bundle.OneTimePQPrekey.PublicKey),
		}
	}

	hm.sendJSONResponse(w, response)
}
