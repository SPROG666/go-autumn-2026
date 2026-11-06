package create

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/SPROG666/go-autumn-2026/api"
	"github.com/SPROG666/go-autumn-2026/internal/model"
)

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	var body api.CreateTripJSONRequestBody
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		h.response.InvalidRequest(w, r)
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		h.response.InvalidRequest(w, r)
		return
	}

	value, replay, err := h.usecase.Execute(r.Context(), model.CreateParams{
		UserID: body.UserId, DriverID: body.DriverId,
		StartLatitude: body.StartPoint.Latitude, StartLongitude: body.StartPoint.Longitude,
		EndLatitude: body.EndPoint.Latitude, EndLongitude: body.EndPoint.Longitude,
		Price: body.Price,
	}, params.IdempotencyKey)
	if err != nil {
		h.response.Error(w, r, err)
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+value.ID.String())
	status := http.StatusCreated
	if replay {
		status = http.StatusOK
	}
	h.response.Trip(w, r, status, value)
}
