package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func testNutritionMealSchema(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	const ownerID = "550e8400-e29b-41d4-a716-446655440071"

	var foodID string
	err := pool.QueryRow(ctx, `
		INSERT INTO foods (
			owner_user_id, name, calories_per_100g,
			protein_per_100g, fat_per_100g, carbs_per_100g
		) VALUES ($1, 'Oats', 180, 6, 4, 30)
		RETURNING id::text
	`, ownerID).Scan(&foodID)
	if err != nil {
		t.Fatalf("create food for meal: %v", err)
	}

	var mealID string
	t.Run("create meal", func(t *testing.T) {
		err := pool.QueryRow(ctx, `
			INSERT INTO nutrition_meals (owner_user_id, meal_type, eaten_at)
			VALUES ($1, 'breakfast', '2026-10-12T08:30:00Z')
			RETURNING id::text
		`, ownerID).Scan(&mealID)
		if err != nil {
			t.Fatalf("create meal: %v", err)
		}

		_, err = pool.Exec(ctx, `
			INSERT INTO nutrition_meal_items (
				meal_id, food_id, food_name, grams,
				calories_per_100g, protein_per_100g,
				fat_per_100g, carbs_per_100g
			)
			VALUES ($1, $2, 'Oats', 150, 180, 6, 4, 30)
		`, mealID, foodID)
		if err != nil {
			t.Fatalf("create meal item: %v", err)
		}
	})

	t.Run("snapshot survives food changes", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			UPDATE foods SET name = 'Updated oats', calories_per_100g = 250
			WHERE id = $1::uuid
		`, foodID)
		if err != nil {
			t.Fatalf("update catalog food: %v", err)
		}

		var name string
		var calories, protein, fat, carbs float64
		err = pool.QueryRow(ctx, `
			SELECT food_name,
				ROUND(calories_per_100g * grams / 100, 2)::float8,
				ROUND(protein_per_100g * grams / 100, 2)::float8,
				ROUND(fat_per_100g * grams / 100, 2)::float8,
				ROUND(carbs_per_100g * grams / 100, 2)::float8
			FROM nutrition_meal_items
			WHERE meal_id = $1::uuid
		`, mealID).Scan(&name, &calories, &protein, &fat, &carbs)
		if err != nil {
			t.Fatalf("read historical meal item: %v", err)
		}
		if name != "Oats" || calories != 270 || protein != 9 || fat != 6 || carbs != 45 {
			t.Fatalf("meal snapshot changed: %s %.2f %.2f %.2f %.2f", name, calories, protein, fat, carbs)
		}
	})

	t.Run("invalid grams rejected", func(t *testing.T) {
		_, err := pool.Exec(ctx, `
			INSERT INTO nutrition_meal_items (
				meal_id, food_id, food_name, grams,
				calories_per_100g, protein_per_100g,
				fat_per_100g, carbs_per_100g
			)
			VALUES ($1, $2, 'Oats', 0, 180, 6, 4, 30)
		`, mealID, foodID)
		if err == nil {
			t.Fatal("zero grams must violate database check constraint")
		}
	})

	t.Run("delete food preserves diary entry", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM foods WHERE id = $1::uuid`, foodID)
		if err != nil {
			t.Fatalf("delete catalog food: %v", err)
		}

		var count int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM nutrition_meal_items
			WHERE meal_id = $1::uuid AND food_id IS NULL AND food_name = 'Oats'
		`, mealID).Scan(&count)
		if err != nil || count != 1 {
			t.Fatalf("historical meal item was lost: count=%d err=%v", count, err)
		}
	})

	t.Run("delete meal cascades to items", func(t *testing.T) {
		_, err := pool.Exec(ctx, `DELETE FROM nutrition_meals WHERE id = $1::uuid`, mealID)
		if err != nil {
			t.Fatalf("delete meal: %v", err)
		}

		var count int
		err = pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM nutrition_meal_items WHERE meal_id = $1::uuid
		`, mealID).Scan(&count)
		if err != nil || count != 0 {
			t.Fatalf("meal items not deleted: count=%d err=%v", count, err)
		}
	})

	t.Run("weekly SQL aggregation uses UTC and isolates users", func(t *testing.T) {
		const anotherUser = "550e8400-e29b-41d4-a716-446655440072"
		insertMeal := func(owner, when string, grams int) {
			t.Helper()
			var createdID string
			err := pool.QueryRow(ctx, `
				INSERT INTO nutrition_meals (owner_user_id, meal_type, eaten_at)
				VALUES ($1::uuid, 'snack', $2::timestamptz)
				RETURNING id::text
			`, owner, when).Scan(&createdID)
			if err != nil {
				t.Fatalf("insert weekly meal: %v", err)
			}
			if grams > 0 {
				_, err = pool.Exec(ctx, `
					INSERT INTO nutrition_meal_items (
						meal_id, food_name, grams, calories_per_100g,
						protein_per_100g, fat_per_100g, carbs_per_100g
					) VALUES ($1::uuid, 'Weekly test food', $2, 180, 6, 4, 30)
				`, createdID, grams)
				if err != nil {
					t.Fatalf("insert weekly meal item: %v", err)
				}
			}
		}

		insertMeal(ownerID, "2026-10-12T00:30:00+03:00", 100)
		insertMeal(ownerID, "2026-10-12T10:00:00+00:00", 150)
		insertMeal(ownerID, "2026-10-14T18:00:00+00:00", 50)
		insertMeal(anotherUser, "2026-10-12T12:00:00+00:00", 100)

		repo := NewMealRepo(pool, 2*time.Second)
		start, err := time.Parse("2006-01-02", "2026-10-12")
		if err != nil {
			t.Fatalf("parse start date: %v", err)
		}
		week, err := repo.ListDailySummaries(ctx, ownerID, start, start.AddDate(0, 0, 7))
		if err != nil {
			t.Fatalf("weekly aggregation: %v", err)
		}
		if len(week) != 2 ||
			week[0].Date != "2026-10-12" || week[0].Meals != 1 ||
			week[0].Items != 1 || week[0].Calories != 270 ||
			week[1].Date != "2026-10-14" || week[1].Meals != 1 ||
			week[1].Calories != 90 {
			t.Fatalf("wrong UTC grouping or user isolation: %+v", week)
		}
	})

}
