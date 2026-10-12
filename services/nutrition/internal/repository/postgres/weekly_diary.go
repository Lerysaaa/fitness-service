package postgres

import (
	"context"
	"fmt"
	"time"

	"fitness-platform/services/nutrition/internal/domain"
)

func (r *MealRepo) ListDailySummaries(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]domain.DiarySummary, error) {
	ctx, cancel := r.operationCtx(ctx)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT TO_CHAR((m.eaten_at AT TIME ZONE 'UTC')::date, 'YYYY-MM-DD'),
			COUNT(DISTINCT m.id)::int, COUNT(i.id)::int,
			COALESCE(ROUND(SUM(i.grams * i.calories_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.protein_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.fat_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.carbs_per_100g / 100), 2), 0)::float8
		FROM nutrition_meals m
		LEFT JOIN nutrition_meal_items i ON i.meal_id = m.id
		WHERE m.owner_user_id = $1::uuid
			AND m.eaten_at >= $2
			AND m.eaten_at < $3
		GROUP BY (m.eaten_at AT TIME ZONE 'UTC')::date
		ORDER BY (m.eaten_at AT TIME ZONE 'UTC')::date
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list daily summaries: %w", err)
	}
	defer rows.Close()

	summaries := make([]domain.DiarySummary, 0, 7)
	for rows.Next() {
		var day domain.DiarySummary
		if err := rows.Scan(
			&day.Date, &day.Meals, &day.Items, &day.Calories,
			&day.Protein, &day.Fat, &day.Carbs,
		); err != nil {
			return nil, fmt.Errorf("scan daily summary: %w", err)
		}
		summaries = append(summaries, day)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate daily summaries: %w", err)
	}
	return summaries, nil
}
