package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"fitness-platform/services/nutrition/internal/domain"
)

type fakeDiaryRepository struct {
	created    *domain.Meal
	calls      int
	weekly     []domain.DiarySummary
	weeklyUser string
}

func (f *fakeDiaryRepository) CreateMeal(_ context.Context, m *domain.Meal) (*domain.Meal, error) {
	f.calls++
	f.created = m
	copyOfMeal := *m
	copyOfMeal.Items = append([]domain.MealItem(nil), m.Items...)
	for i := range copyOfMeal.Items {
		copyOfMeal.Items[i].FoodName = "Oats"
		copyOfMeal.Items[i].CaloriesPer100g = 180
		copyOfMeal.Items[i].ProteinPer100g = 6
		copyOfMeal.Items[i].FatPer100g = 4
		copyOfMeal.Items[i].CarbsPer100g = 30
	}
	return &copyOfMeal, nil
}

func (f *fakeDiaryRepository) ListMeals(_ context.Context, _ string, _, _ time.Time) ([]domain.Meal, error) {
	return []domain.Meal{}, nil
}

func (f *fakeDiaryRepository) ListDailySummaries(
	_ context.Context, user string, _, _ time.Time,
) ([]domain.DiarySummary, error) {
	f.weeklyUser = user
	return f.weekly, nil
}

func (f *fakeDiaryRepository) DeleteMeal(_ context.Context, _, _ string) (bool, error) {
	return true, nil
}

func (f *fakeDiaryRepository) GetDiarySummary(_ context.Context, _ string, _, _ time.Time) (*domain.DiarySummary, error) {
	return &domain.DiarySummary{}, nil
}

func TestMealValidationAndCalculation(t *testing.T) {
	const owner = "550e8400-e29b-41d4-a716-446655440001"
	const foodID = "550e8400-e29b-41d4-a716-446655440002"
	ctx := context.Background()
	at := time.Date(2026, 10, 12, 8, 30, 0, 0, time.FixedZone("UTC+3", 3*3600))
	makeMeal := func() *domain.Meal {
		return &domain.Meal{
			MealType: " Breakfast ",
			EatenAt:  at,
			Items: []domain.MealItem{{FoodID: foodID, Grams: 150}},
		}
	}
	t.Run("valid meal calculation", func(t *testing.T) {
		fake := &fakeDiaryRepository{}
		got, err := NewMealService(fake).CreateMeal(ctx, owner, makeMeal())
		if err != nil {
			t.Fatalf("create meal: %v", err)
		}
		if fake.calls != 1 || fake.created.MealType != "breakfast" ||
			fake.created.EatenAt.Location() != time.UTC ||
			got.Calories != 270 || got.Protein != 9 ||
			got.Fat != 6 || got.Carbs != 45 {
			t.Fatalf("unexpected meal: %+v", got)
		}
	})

	cases := map[string]func(*domain.Meal){
		"empty type": func(m *domain.Meal) { m.MealType = "" },
		"invalid type": func(m *domain.Meal) { m.MealType = "midnight" },
		"no timestamp": func(m *domain.Meal) { m.EatenAt = time.Time{} },
		"no items": func(m *domain.Meal) { m.Items = nil },
		"zero grams": func(m *domain.Meal) { m.Items[0].Grams = 0 },
		"negative grams": func(m *domain.Meal) { m.Items[0].Grams = -1 },
		"excess grams": func(m *domain.Meal) { m.Items[0].Grams = 10001 },
		"fractional grams": func(m *domain.Meal) { m.Items[0].Grams = 2.555 },
		"invalid food id": func(m *domain.Meal) { m.Items[0].FoodID = "bad" },
		"over 50 items": func(m *domain.Meal) {
			m.Items = make([]domain.MealItem, 51)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			fake := &fakeDiaryRepository{}
			meal := makeMeal()
			mutate(meal)
			if _, err := NewMealService(fake).CreateMeal(ctx, owner, meal);
				!errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid input, got %v", err)
			}
			if fake.calls != 0 {
				t.Fatal("invalid data reached repository")
			}
		})
	}
}

func TestMealDiaryDateValidation(t *testing.T) {
	for _, date := range []string{"2026-02-30", "12-10-2026", "invalid", "2026-1-2"} {
		t.Run(date, func(t *testing.T) {
			_, _, err := parseDiaryDate(date)
			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("expected invalid date, got %v", err)
			}
		})
	}

	start, end, err := parseDiaryDate("2026-10-12")
	if err != nil || start.Format("2006-01-02") != "2026-10-12" ||
		end.Sub(start) != 24*time.Hour {
		t.Fatalf("bad UTC date boundaries: %v %v %v", start, end, err)
	}
}
