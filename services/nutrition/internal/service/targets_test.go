package service

import (
	"errors"
	"math"
	"testing"
)

func TestCalculateTargets(t *testing.T) {
	tests := []struct {
		name         string
		goal         string
		wantCalories float64
	}{
		{
			name:         "lose weight",
			goal:         "lose",
			wantCalories: 1937.655,
		},
		{
			name:         "maintain weight",
			goal:         "maintain",
			wantCalories: 2152.95,
		},
		{
			name:         "gain weight",
			goal:         "gain",
			wantCalories: 2368.245,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			profile := validTestProfile()
			profile.Goal = tt.goal

			got, err := CalculateTargets(profile)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if math.Abs(got.DailyCalories-tt.wantCalories) > 0.000001 {
				t.Fatalf(
					"expected calories %v, got %v",
					tt.wantCalories,
					got.DailyCalories,
				)
			}

			if math.Abs(got.BMR-1389) > 0.000001 {
				t.Fatalf("unexpected BMR: %v", got.BMR)
			}

			if math.Abs(got.TDEE-2152.95) > 0.000001 {
				t.Fatalf("unexpected TDEE: %v", got.TDEE)
			}

			totalCalories :=
				got.ProteinGrams*4 +
					got.FatGrams*9 +
					got.CarbsGrams*4

			if math.Abs(totalCalories-got.DailyCalories) > 0.000001 {
				t.Fatalf(
					"macros contain %v calories, expected %v",
					totalCalories,
					got.DailyCalories,
				)
			}
		})
	}
}

func TestCalculateTargetsInvalidInput(t *testing.T) {
	t.Run("nil profile", func(t *testing.T) {
		_, err := CalculateTargets(nil)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})

	t.Run("invalid goal", func(t *testing.T) {
		profile := validTestProfile()
		profile.Goal = "pizza"

		_, err := CalculateTargets(profile)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})

	t.Run("invalid activity", func(t *testing.T) {
		profile := validTestProfile()
		profile.ActivityLevel = "extreme"

		_, err := CalculateTargets(profile)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})

	t.Run("invalid weight", func(t *testing.T) {
		profile := validTestProfile()
		profile.WeightKg = -10

		_, err := CalculateTargets(profile)

		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf(
				"expected ErrInvalidInput, got %v",
				err,
			)
		}
	})
}

func TestCalculateTargetsWithoutUserID(t *testing.T) {
	profile := validTestProfile()
	profile.UserID = ""

	result, err := CalculateTargets(profile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.DailyCalories <= 0 {
		t.Fatal("daily calories must be positive")
	}
}
