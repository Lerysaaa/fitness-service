package service

func goalMultiplier(goal string) (float64, error) {
	switch goal {
	case "lose":
		return 0.9, nil

	case "maintain":
		return 1.0, nil

	case "gain":
		return 1.1, nil

	default:
		return 0, ErrInvalidInput
	}
}
