package service

import "fitness-platform/services/nutrition/internal/domain"

type NutritionTargets struct {
	BMR           float64 `json:"bmr"`
	TDEE          float64 `json:"tdee"`
	DailyCalories float64 `json:"daily_calories"`
	ProteinGrams  float64 `json:"protein_grams"`
	FatGrams      float64 `json:"fat_grams"`
	CarbsGrams    float64 `json:"carbs_grams"`
}

func CalculateTargets(
	profile *domain.NutritionProfile,
) (*NutritionTargets, error) {
	bmr, err := CalculateBMR(profile)
	if err != nil {
		return nil, err
	}

	tdee, err := CalculateTDEE(profile)
	if err != nil {
		return nil, err
	}

	multiplier, err := goalMultiplier(profile.Goal)
	if err != nil {
		return nil, err
	}

	calories := tdee * multiplier

	protein := calories * 0.25 / 4
	fat := calories * 0.30 / 9
	carbs := calories * 0.45 / 4

	return &NutritionTargets{
		BMR:           bmr,
		TDEE:          tdee,
		DailyCalories: calories,
		ProteinGrams:  protein,
		FatGrams:      fat,
		CarbsGrams:    carbs,
	}, nil
}
