package service

import "fitness-platform/services/nutrition/internal/domain"

func CalculateBMR(
	profile *domain.NutritionProfile,
) (float64, error) {
	if err := validateBodyMeasurements(profile); err != nil {
		return 0, err
	}

	base := 10*profile.WeightKg +
		6.25*profile.HeightCm -
		5*float64(profile.Age)

	if profile.Sex == "male" {
		return base + 5, nil
	}

	return base - 161, nil
}
