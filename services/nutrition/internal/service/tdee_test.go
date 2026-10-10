package service

import (
	"errors"
	"math"
	"testing"

	"fitness-platform/services/nutrition/internal/domain"
)

func TestCalculateTDEE(t *testing.T) {
	tests := []struct {
		name          string
		sex           string
		activityLevel string
		want          float64
	}{
		{
			name:          "female sedentary",
			sex:           "female",
			activityLevel: "sedentary",
			want:          1666.8,
		},
		{
			name:          "female light",
			sex:           "female",
			activityLevel: "light",
			want:          1909.875,
		},
		{
			name:          "female moderate",
			sex:           "female",
			activityLevel: "moderate",
			want:          2152.95,
		},
		{
			name:          "female high",
			sex:           "female",
			activityLevel: "high",
			want:          2396.025,
		},
		{
			name:          "female very high",
			sex:           "female",
			activityLevel: "very_high",
			want:          2639.1,
		},
		{
			name:          "male moderate",
			sex:           "male",
			activityLevel: "moderate",
			want:          2410.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := validTestProfile()
			profile.Sex = tt.sex
			profile.ActivityLevel = tt.activityLevel

			got, err := CalculateTDEE(profile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got-tt.want) > 0.000001 {
				t.Fatalf(
					"expected TDEE %v, got %v",
					tt.want,
					got,
				)
			}
		})
	}
}

func TestCalculateTDEEInvalidInput(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.NutritionProfile)
	}{
		{
			name: "unknown activity level",
			change: func(p *domain.NutritionProfile) {
				p.ActivityLevel = "extreme"
			},
		},
		{
			name: "empty activity level",
			change: func(p *domain.NutritionProfile) {
				p.ActivityLevel = ""
			},
		},
		{
			name: "invalid age",
			change: func(p *domain.NutritionProfile) {
				p.Age = -5
			},
		},
		{
			name: "invalid weight",
			change: func(p *domain.NutritionProfile) {
				p.WeightKg = 0
			},
		},
		{
			name: "invalid height",
			change: func(p *domain.NutritionProfile) {
				p.HeightCm = 0
			},
		},
		{
			name: "invalid sex",
			change: func(p *domain.NutritionProfile) {
				p.Sex = "unknown"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := validTestProfile()
			tt.change(profile)

			_, err := CalculateTDEE(profile)

			if !errors.Is(err, ErrInvalidInput) {
				t.Fatalf(
					"expected ErrInvalidInput, got %v",
					err,
				)
			}
		})
	}

	t.Run("nil profile", func(t *testing.T) {
		_, err := CalculateTDEE(nil)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})
}

func TestCalculateTDEEWithoutGoal(t *testing.T) {
	profile := validTestProfile()

	profile.Goal = ""
	profile.UserID = ""

	got, err := CalculateTDEE(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := 2152.95

	if math.Abs(got-want) > 0.000001 {
		t.Fatalf(
			"expected TDEE %v, got %v",
			want,
			got,
		)
	}
}
