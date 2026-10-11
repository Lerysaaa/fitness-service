package service

import (
	"context"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"fitness-platform/services/nutrition/internal/domain"
)

// FoodRepository описывает операции с каталогом, необходимые FoodService.
type FoodRepository interface {
	CreatePersonalFood(
		ctx context.Context,
		userID string,
		food *domain.Food,
	) (*domain.Food, error)

	GetVisibleFood(
		ctx context.Context,
		userID string,
		foodID string,
	) (*domain.Food, error)

	ListVisibleFoods(
		ctx context.Context,
		userID string,
		limit int,
		offset int,
	) ([]domain.Food, error)

	UpdatePersonalFood(
		ctx context.Context,
		userID string,
		foodID string,
		food *domain.Food,
	) (*domain.Food, error)

	DeletePersonalFood(
		ctx context.Context,
		userID string,
		foodID string,
	) (bool, error)
}

type FoodService struct {
	repo FoodRepository
}

func NewFoodService(repo FoodRepository) *FoodService {
	return &FoodService{repo: repo}
}

// CreateFood создаёт личный продукт авторизованного пользователя.
func (s *FoodService) CreateFood(
	ctx context.Context,
	userID string,
	food *domain.Food,
) (*domain.Food, error) {
	if !validUUID(userID) {
		return nil, ErrInvalidInput
	}

	normalized, err := normalizeFood(food)
	if err != nil {
		return nil, err
	}

	return s.repo.CreatePersonalFood(ctx, userID, normalized)
}

// GetFood возвращает общий или собственный продукт.
func (s *FoodService) GetFood(
	ctx context.Context,
	userID string,
	foodID string,
) (*domain.Food, error) {
	if !validUUID(userID) || !validUUID(foodID) {
		return nil, ErrInvalidInput
	}

	return s.repo.GetVisibleFood(ctx, userID, foodID)
}

// ListFoods возвращает доступные продукты с пагинацией.
func (s *FoodService) ListFoods(
	ctx context.Context,
	userID string,
	limit int,
	offset int,
) ([]domain.Food, error) {
	if !validUUID(userID) ||
		limit < 1 ||
		limit > 100 ||
		offset < 0 {
		return nil, ErrInvalidInput
	}

	return s.repo.ListVisibleFoods(ctx, userID, limit, offset)
}

// UpdateFood изменяет личный продукт пользователя.
func (s *FoodService) UpdateFood(
	ctx context.Context,
	userID string,
	foodID string,
	food *domain.Food,
) (*domain.Food, error) {
	if !validUUID(userID) || !validUUID(foodID) {
		return nil, ErrInvalidInput
	}

	normalized, err := normalizeFood(food)
	if err != nil {
		return nil, err
	}

	return s.repo.UpdatePersonalFood(
		ctx,
		userID,
		foodID,
		normalized,
	)
}

// DeleteFood мягко удаляет личный продукт пользователя.
func (s *FoodService) DeleteFood(
	ctx context.Context,
	userID string,
	foodID string,
) (bool, error) {
	if !validUUID(userID) || !validUUID(foodID) {
		return false, ErrInvalidInput
	}

	return s.repo.DeletePersonalFood(ctx, userID, foodID)
}

// normalizeFood проверяет продукт и нормализует его название и бренд.
func normalizeFood(food *domain.Food) (*domain.Food, error) {
	if food == nil {
		return nil, ErrInvalidInput
	}

	normalized := *food
	normalized.Name = strings.TrimSpace(food.Name)
	normalized.Brand = strings.TrimSpace(food.Brand)

	if !utf8.ValidString(normalized.Name) ||
		!utf8.ValidString(normalized.Brand) ||
		utf8.RuneCountInString(normalized.Name) < 1 ||
		utf8.RuneCountInString(normalized.Name) > 200 ||
		utf8.RuneCountInString(normalized.Brand) > 120 {
		return nil, ErrInvalidInput
	}

	if !validNutritionValue(normalized.CaloriesPer100g, 1000) ||
		!validNutritionValue(normalized.ProteinPer100g, 100) ||
		!validNutritionValue(normalized.FatPer100g, 100) ||
		!validNutritionValue(normalized.CarbsPer100g, 100) {
		return nil, ErrInvalidInput
	}

	// ID и владелец назначаются Repository, а не клиентом.
	normalized.ID = ""
	normalized.OwnerUserID = ""

	return &normalized, nil
}

// validNutritionValue проверяет допустимую пищевую ценность на 100 г.
func validNutritionValue(value float64, max float64) bool {
	return !math.IsNaN(value) &&
		!math.IsInf(value, 0) &&
		value >= 0 &&
		value <= max &&
		math.Round(value*100)/100 == value
}

func validUUID(value string) bool {
	_, err := uuid.Parse(value)
	return err == nil
}
