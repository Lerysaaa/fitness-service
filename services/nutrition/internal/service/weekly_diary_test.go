package service

import (
	"context"
	"errors"
	"testing"

	"fitness-platform/services/nutrition/internal/domain"
)

func TestWeeklyDiaryReport(t *testing.T) {
	const owner = "550e8400-e29b-41d4-a716-446655440001"

	fake := &fakeDiaryRepository{
		weekly: []domain.DiarySummary{
			{Date: "2026-10-12", Meals: 2, Items: 3, Calories: 330, Protein: 12, Fat: 9, Carbs: 49.8},
			{Date: "2026-10-14", Meals: 1, Items: 1, Calories: 270, Protein: 9, Fat: 6, Carbs: 45},
		},
	}
	report, err := NewMealService(fake).GetWeeklyReport(context.Background(), owner, "2026-10-12")
	if err != nil {
		t.Fatalf("weekly report: %v", err)
	}

	if fake.weeklyUser != owner {
		t.Fatalf("wrong repository user: %s", fake.weeklyUser)
	}
	if report.StartDate != "2026-10-12" || report.EndDate != "2026-10-18" ||
		len(report.Days) != 7 || report.TrackedDays != 2 {
		t.Fatalf("wrong calendar range: %+v", report)
	}
	if report.Totals.Meals != 3 || report.Totals.Items != 4 ||
		report.Totals.Calories != 600 || report.Totals.Protein != 21 ||
		report.Totals.Fat != 15 || report.Totals.Carbs != 94.8 ||
		report.AverageCaloriesPerTrackedDay != 300 {
		t.Fatalf("wrong totals: %+v", report)
	}
	if report.Days[1].Date != "2026-10-13" ||
		report.Days[1].Meals != 0 || report.Days[1].Calories != 0 ||
		report.Days[2].Date != "2026-10-14" {
		t.Fatalf("missing empty calendar days: %+v", report.Days)
	}
	if report.DailyTargets != nil || report.Days[0].CaloriesVsTarget != nil {
		t.Fatal("target comparison must be absent without a profile service")
	}
}

func TestWeeklyDiaryValidationAndEmptyWeek(t *testing.T) {
	const owner = "550e8400-e29b-41d4-a716-446655440001"
	svc := NewMealService(&fakeDiaryRepository{})

	for _, date := range []string{"2026-02-30", "not-a-date", "2026-10-1"} {
		_, err := svc.GetWeeklyReport(context.Background(), owner, date)
		if !errors.Is(err, ErrInvalidInput) {
			t.Fatalf("date %q expected invalid input; got %v", date, err)
		}
	}
	if _, err := svc.GetWeeklyReport(context.Background(), "not-a-uuid", "2026-10-12"); !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected bad user id error, got %v", err)
	}

	report, err := svc.GetWeeklyReport(context.Background(), owner, "2026-10-12")
	if err != nil {
		t.Fatalf("empty week: %v", err)
	}
	if report.TrackedDays != 0 || report.AverageCaloriesPerTrackedDay != 0 ||
		report.AverageCaloriesVsTargetTrackedDay != nil || len(report.Days) != 7 {
		t.Fatalf("unexpected empty week: %+v", report)
	}
}
