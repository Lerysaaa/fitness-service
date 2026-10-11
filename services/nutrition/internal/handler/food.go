package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"fitness-platform/pkg/logger"
	"fitness-platform/pkg/middleware"
	"fitness-platform/services/nutrition/internal/domain"
	"fitness-platform/services/nutrition/internal/service"
)

type FoodHandler struct {
	foodService *service.FoodService
}

func NewFoodHandler(foodService *service.FoodService) *FoodHandler {
	return &FoodHandler{foodService: foodService}
}

type createFoodRequest struct {
	Name            string   `json:"name"`
	Brand           string   `json:"brand"`
	CaloriesPer100g *float64 `json:"calories_per_100g"`
	ProteinPer100g  *float64 `json:"protein_per_100g"`
	FatPer100g      *float64 `json:"fat_per_100g"`
	CarbsPer100g    *float64 `json:"carbs_per_100g"`
}

// CreateFood обрабатывает создание личного продукта.
func (h *FoodHandler) CreateFood(
	w http.ResponseWriter,
	r *http.Request,
) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	var req createFoodRequest

	if err := decoder.Decode(&req); err != nil {
		var sizeErr *http.MaxBytesError
		if errors.As(err, &sizeErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}

		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.CaloriesPer100g == nil ||
		req.ProteinPer100g == nil ||
		req.FatPer100g == nil ||
		req.CarbsPer100g == nil {
		writeError(w, http.StatusBadRequest, "missing nutrition values")
		return
	}

	food := &domain.Food{
		Name:            req.Name,
		Brand:           req.Brand,
		CaloriesPer100g: *req.CaloriesPer100g,
		ProteinPer100g:  *req.ProteinPer100g,
		FatPer100g:      *req.FatPer100g,
		CarbsPer100g:    *req.CarbsPer100g,
	}

	created, err := h.foodService.CreateFood(
		r.Context(),
		userID,
		food,
	)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, "invalid food data")
			return
		}

		logger.FromContext(r.Context()).
			Error().
			Err(err).
			Msg("failed to create food")

		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	w.Header().Set("Location", "/nutrition/foods/"+created.ID)
	writeJSON(w, http.StatusCreated, created)
}
