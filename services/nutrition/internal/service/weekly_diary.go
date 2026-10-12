package service

import (
	"context"
	"time"

	"fitness-platform/services/nutrition/internal/domain"
)

type WeeklyDiaryDay struct {
	domain.DiarySummary
	CaloriesVsTarget *float64 `json:"calories_vs_target,omitempty"`
}

type WeeklyDiaryTotals struct {
	Meals    int     `json:"meals"`
	Items    int     `json:"items"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carbs    float64 `json:"carbs"`
}

type WeeklyDiaryReport struct {
	StartDate                         string           `json:"start_date"`
	EndDate                           string           `json:"end_date"`
	Days                              []WeeklyDiaryDay `json:"days"`
	Totals                            WeeklyDiaryTotals `json:"totals"`
	TrackedDays                       int              `json:"tracked_days"`
	AverageCaloriesPerTrackedDay      float64          `json:"average_calories_per_tracked_day"`
	DailyTargets                      *NutritionTargets `json:"daily_targets,omitempty"`
	AverageCaloriesVsTargetTrackedDay *float64         `json:"average_calories_vs_target_tracked_day,omitempty"`
}

func (s *MealService) GetWeeklyReport(
	ctx context.Context,
	userID string,
	startDate string,
) (*WeeklyDiaryReport, error) {
	ownerID, err := normalizeUUID(userID)
	if err != nil {
		return nil, ErrInvalidInput
	}

	start, err := weeklyStart(startDate)
	if err != nil {
		return nil, err
	}
	end := start.AddDate(0, 0, 7)

	summaries, err := s.repo.ListDailySummaries(ctx, ownerID, start, end)
	if err != nil {
		return nil, err
	}

	byDate := make(map[string]domain.DiarySummary, len(summaries))
	for _, summary := range summaries {
		byDate[summary.Date] = summary
	}

	report := &WeeklyDiaryReport{
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.AddDate(0, 0, -1).Format("2006-01-02"),
		Days:      make([]WeeklyDiaryDay, 0, 7),
	}

	if s.nutrition != nil {
		report.DailyTargets, err = s.nutrition.GetTargets(ctx, ownerID)
		if err != nil {
			return nil, err
		}
	}

	for offset := 0; offset < 7; offset++ {
		date := start.AddDate(0, 0, offset).Format("2006-01-02")
		summary, ok := byDate[date]
		if !ok {
			summary = domain.DiarySummary{Date: date}
		}

		day := WeeklyDiaryDay{DiarySummary: summary}
		report.Totals.Meals += summary.Meals
		report.Totals.Items += summary.Items
		report.Totals.Calories += summary.Calories
		report.Totals.Protein += summary.Protein
		report.Totals.Fat += summary.Fat
		report.Totals.Carbs += summary.Carbs

		if summary.Meals > 0 {
			report.TrackedDays++
			if report.DailyTargets != nil {
				delta := roundMealValue(summary.Calories - report.DailyTargets.DailyCalories)
				day.CaloriesVsTarget = &delta
			}
		}

		report.Days = append(report.Days, day)
	}

	report.Totals.Calories = roundMealValue(report.Totals.Calories)
	report.Totals.Protein = roundMealValue(report.Totals.Protein)
	report.Totals.Fat = roundMealValue(report.Totals.Fat)
	report.Totals.Carbs = roundMealValue(report.Totals.Carbs)

	if report.TrackedDays > 0 {
		report.AverageCaloriesPerTrackedDay = roundMealValue(
			report.Totals.Calories / float64(report.TrackedDays),
		)
		if report.DailyTargets != nil {
			delta := roundMealValue(
				report.Totals.Calories/float64(report.TrackedDays) -
					report.DailyTargets.DailyCalories,
			)
			report.AverageCaloriesVsTargetTrackedDay = &delta
		}
	}

	return report, nil
}

func weeklyStart(date string) (time.Time, error) {
	if date != "" {
		start, _, err := parseDiaryDate(date)
		return start, err
	}

	now := time.Now().UTC()
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	daysFromMonday := (int(now.Weekday()) + 6) % 7
	return midnight.AddDate(0, 0, -daysFromMonday), nil
}
