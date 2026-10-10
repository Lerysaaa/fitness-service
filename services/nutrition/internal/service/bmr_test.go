package service

import (
	"errors"
	"math"
	"testing"

	"fitness-platform/services/nutrition/internal/domain"
)

func TestCalculateBMR(t *testing.T) {
	tests := []struct {
		name string
		sex  string
		want float64
	}{
		{
			name: "male BMR",
			sex:  "male",
			want: 1555,
		},
		{
			name: "female BMR",
			sex:  "female",
			want: 1389,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := validTestProfile()
			profile.Sex = tt.sex

			got, err := CalculateBMR(profile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Fatalf(
					"expected BMR %v, got %v",
					tt.want,
					got,
				)
			}
		})
	}
}

func TestCalculateBMRInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.NutritionProfile)
	}{
		{
			name: "invalid age",
			change: func(p *domain.NutritionProfile) {
				p.Age = -5
			},
		},
		{
			name: "invalid height",
			change: func(p *domain.NutritionProfile) {
				p.HeightCm = 0
			},
		},
		{
			name: "invalid weight",
			change: func(p *domain.NutritionProfile) {
				p.WeightKg = 0
			},
		},
		{
			name: "invalid sex",
			change: func(p *domain.NutritionProfile) {
				p.Sex = "unknown"
			},
		},
		{
			name: "weight is NaN",
			change: func(p *domain.NutritionProfile) {
				p.WeightKg = math.NaN()
			},
		},
		{
			name: "height is infinity",
			change: func(p *domain.NutritionProfile) {
				p.HeightCm = math.Inf(1)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := validTestProfile()
			tt.change(profile)

			_, err := CalculateBMR(profile)

			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf(
					"expected ErrInvalidInput, got %v",
					err,
				)
			}
		})
	}

	t.Run("nil profile", func(t *testing.T) {
		_, err := CalculateBMR(nil)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})
}

func TestCalculateBMRIndependentOfActivity(t *testing.T) {
	profile := validTestProfile()

	profile.ActivityLevel = "high"
	highBMR, err := CalculateBMR(profile)
	if err != nil {
		t.Fatal(err)
	}

	profile.ActivityLevel = "sedentary"
	lowBMR, err := CalculateBMR(profile)
	if err != nil {
		t.Fatal(err)
	}

	if highBMR != lowBMR {
		t.Fatal("activity level must not affect BMR")
	}
}

func TestCalculateBMRWithoutGoal(t *testing.T) {
	profile := validTestProfile()
	profile.Goal = ""
	profile.ActivityLevel = ""

	bmr, err := CalculateBMR(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if bmr != 1389 {
		t.Fatalf("expected BMR 1389, got %v", bmr)
	}
}
