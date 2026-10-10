package service

func activityMultiplier(level string) (float64, error) {
	switch level {
	case "sedentary":
		return 1.2, nil

	case "light":
		return 1.375, nil

	case "moderate":
		return 1.55, nil

	case "high":
		return 1.725, nil

	case "very_high":
		return 1.9, nil

	default:
		return 0, ErrInvalidInput
	}
}
