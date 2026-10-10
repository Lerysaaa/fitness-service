package service

import "fitness-platform/services/nutrition/internal/domain"

func CalculateTDEE(
	profile *domain.NutritionProfile,
) (float64, error) {
	bmr, err := CalculateBMR(profile)
	if err != nil {
		return 0, err
	}

	multiplier, err := activityMultiplier(
		profile.ActivityLevel,
	)
	if err != nil {
		return 0, err
	}

	tdee := bmr * multiplier

	return tdee, nil
}
