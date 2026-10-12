package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"fitness-platform/pkg/logger"
	"fitness-platform/pkg/middleware"
	"fitness-platform/services/nutrition/internal/domain"
	"fitness-platform/services/nutrition/internal/service"
)

type MealHandler struct {
	service *service.MealService
}

func NewMealHandler(s *service.MealService) *MealHandler {
	return &MealHandler{service: s}
}

type mealItemRequest struct {
	FoodID string  `json:"food_id"`
	Grams  float64 `json:"grams"`
}

type createMealRequest struct {
	MealType string            `json:"meal_type"`
	EatenAt  string            `json:"eaten_at"`
	Items    []mealItemRequest `json:"items"`
}

func (h *MealHandler) CreateMeal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var req createMealRequest
	if err := decoder.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid meal JSON")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		writeError(w, http.StatusBadRequest, "invalid meal JSON")
		return
	}

	eatenAt, err := time.Parse(time.RFC3339, req.EatenAt)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid eaten_at timestamp")
		return
	}
	meal := &domain.Meal{
		MealType: req.MealType,
		EatenAt:  eatenAt,
		Items:    make([]domain.MealItem, 0, len(req.Items)),
	}
	for _, item := range req.Items {
		meal.Items = append(meal.Items, domain.MealItem{
			FoodID: item.FoodID,
			Grams:  item.Grams,
		})
	}

	created, err := h.service.CreateMeal(r.Context(), userID, meal)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidInput):
			writeError(w, http.StatusBadRequest, "invalid meal data")
		case errors.Is(err, domain.ErrMealFoodNotFound):
			writeError(w, http.StatusNotFound, "food not found")
		default:
			logger.FromContext(r.Context()).Error().Err(err).Msg("failed to create meal")
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	w.Header().Set("Location", "/nutrition/meals/"+created.ID)
	writeJSON(w, http.StatusCreated, created)
}

func (h *MealHandler) ListMeals(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}
	meals, err := h.service.ListMeals(r.Context(), userID, r.URL.Query().Get("date"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, "invalid date")
		} else {
			logger.FromContext(r.Context()).Error().Err(err).Msg("failed to list meals")
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	writeJSON(w, http.StatusOK, meals)
}

func (h *MealHandler) DeleteMeal(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}
	deleted, err := h.service.DeleteMeal(r.Context(), userID, chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, "invalid meal id")
		} else {
			logger.FromContext(r.Context()).Error().Err(err).Msg("failed to delete meal")
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	if !deleted {
		writeError(w, http.StatusNotFound, "meal not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *MealHandler) GetDiarySummary(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user not authenticated")
		return
	}
	summary, err := h.service.GetDiarySummary(r.Context(), userID, r.URL.Query().Get("date"))
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, http.StatusBadRequest, "invalid date")
		} else {
			logger.FromContext(r.Context()).Error().Err(err).Msg("failed to summarize diary")
			writeError(w, http.StatusInternalServerError, "internal server error")
		}
		return
	}
	writeJSON(w, http.StatusOK, summary)
}
