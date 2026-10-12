package service

import (
	"context"
	"math"
	"strings"
	"time"

	"fitness-platform/services/nutrition/internal/domain"
)

type MealRepository interface {
	CreateMeal(context.Context, *domain.Meal) (*domain.Meal, error)
	ListMeals(context.Context, string, time.Time, time.Time) ([]domain.Meal, error)
	DeleteMeal(context.Context, string, string) (bool, error)
	GetDiarySummary(context.Context, string, time.Time, time.Time) (*domain.DiarySummary, error)
	ListDailySummaries(context.Context, string, time.Time, time.Time) ([]domain.DiarySummary, error)
}

type MealService struct {
	repo      MealRepository
	nutrition *NutritionService
}

func NewMealService(repo MealRepository, nutrition ...*NutritionService) *MealService {
	result := &MealService{repo: repo}
	if len(nutrition) > 0 {
		result.nutrition = nutrition[0]
	}
	return result
}

func (s *MealService) CreateMeal(ctx context.Context, userID string, meal *domain.Meal) (*domain.Meal, error) {
	ownerID, err := normalizeUUID(userID)
	if err != nil || meal == nil {
		return nil, ErrInvalidInput
	}
	mealType := strings.ToLower(strings.TrimSpace(meal.MealType))
	switch mealType {
	case "breakfast", "lunch", "dinner", "snack":
	default:
		return nil, ErrInvalidInput
	}
	if meal.EatenAt.IsZero() || len(meal.Items) == 0 || len(meal.Items) > 50 {
		return nil, ErrInvalidInput
	}

	normalized := &domain.Meal{
		OwnerID:  ownerID,
		MealType: mealType,
		EatenAt:  meal.EatenAt.UTC(),
		Items:    make([]domain.MealItem, 0, len(meal.Items)),
	}
	for _, item := range meal.Items {
		foodID, err := normalizeUUID(item.FoodID)
		if err != nil || !validNutritionValue(item.Grams, 10000) || item.Grams <= 0 {
			return nil, ErrInvalidInput
		}
		normalized.Items = append(normalized.Items, domain.MealItem{
			FoodID: foodID,
			Grams: item.Grams,
		})
	}
	created, err := s.repo.CreateMeal(ctx, normalized)
	if err != nil {
		return nil, err
	}
	calculateMealTotals(created)
	return created, nil
}

func (s *MealService) ListMeals(ctx context.Context, userID, date string) ([]domain.Meal, error) {
	ownerID, err := normalizeUUID(userID)
	if err != nil {
		return nil, ErrInvalidInput
	}
	start, end, err := parseDiaryDate(date)
	if err != nil {
		return nil, err
	}
	meals, err := s.repo.ListMeals(ctx, ownerID, start, end)
	if err != nil {
		return nil, err
	}
	for i := range meals {
		calculateMealTotals(&meals[i])
	}
	return meals, nil
}

func (s *MealService) DeleteMeal(ctx context.Context, userID, mealID string) (bool, error) {
	ownerID, err := normalizeUUID(userID)
	if err != nil {
		return false, ErrInvalidInput
	}
	id, err := normalizeUUID(mealID)
	if err != nil {
		return false, ErrInvalidInput
	}
	return s.repo.DeleteMeal(ctx, ownerID, id)
}

func (s *MealService) GetDiarySummary(ctx context.Context, userID, date string) (*domain.DiarySummary, error) {
	ownerID, err := normalizeUUID(userID)
	if err != nil {
		return nil, ErrInvalidInput
	}
	start, end, err := parseDiaryDate(date)
	if err != nil {
		return nil, err
	}
	summary, err := s.repo.GetDiarySummary(ctx, ownerID, start, end)
	if err != nil {
		return nil, err
	}
	summary.Date = start.Format("2006-01-02")
	return summary, nil
}

func parseDiaryDate(date string) (time.Time, time.Time, error) {
	if date == "" {
		date = time.Now().UTC().Format("2006-01-02")
	}
	start, err := time.Parse("2006-01-02", date)
	if err != nil || start.Format("2006-01-02") != date {
		return time.Time{}, time.Time{}, ErrInvalidInput
	}
	return start, start.AddDate(0, 0, 1), nil
}

func roundMealValue(value float64) float64 {
	return math.Round(value*100) / 100
}

func calculateMealTotals(meal *domain.Meal) {
	var calories, protein, fat, carbs float64
	for i := range meal.Items {
		item := &meal.Items[i]
		factor := item.Grams / 100
		item.Calories = roundMealValue(item.CaloriesPer100g * factor)
		item.Protein = roundMealValue(item.ProteinPer100g * factor)
		item.Fat = roundMealValue(item.FatPer100g * factor)
		item.Carbs = roundMealValue(item.CarbsPer100g * factor)
		calories += item.Calories
		protein += item.Protein
		fat += item.Fat
		carbs += item.Carbs
	}
	meal.Calories = roundMealValue(calories)
	meal.Protein = roundMealValue(protein)
	meal.Fat = roundMealValue(fat)
	meal.Carbs = roundMealValue(carbs)
}
