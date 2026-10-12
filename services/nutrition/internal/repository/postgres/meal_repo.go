package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"fitness-platform/services/nutrition/internal/domain"
)

type MealRepo struct {
	pool             *pgxpool.Pool
	operationTimeout time.Duration
}

func NewMealRepo(pool *pgxpool.Pool, timeout time.Duration) *MealRepo {
	return &MealRepo{pool: pool, operationTimeout: timeout}
}

func (r *MealRepo) operationCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if r.operationTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, r.operationTimeout)
}

func (r *MealRepo) CreateMeal(ctx context.Context, meal *domain.Meal) (*domain.Meal, error) {
	ctx, cancel := r.operationCtx(ctx)
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin meal transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	created := &domain.Meal{
		OwnerID: meal.OwnerID,
		MealType: meal.MealType,
		EatenAt: meal.EatenAt,
		Items: make([]domain.MealItem, 0, len(meal.Items)),
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO nutrition_meals (owner_user_id, meal_type, eaten_at)
		VALUES ($1::uuid, $2, $3)
		RETURNING id::text, created_at
	`, meal.OwnerID, meal.MealType, meal.EatenAt).Scan(&created.ID, &created.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("create meal: %w", err)
	}

	for _, entry := range meal.Items {
		var food domain.Food
		err = tx.QueryRow(ctx, `
			SELECT name,
				calories_per_100g::float8, protein_per_100g::float8,
				fat_per_100g::float8, carbs_per_100g::float8
			FROM foods
			WHERE id = $1::uuid
				AND deleted_at IS NULL
				AND (owner_user_id IS NULL OR owner_user_id = $2::uuid)
			FOR SHARE
		`, entry.FoodID, meal.OwnerID).Scan(
			&food.Name, &food.CaloriesPer100g,
			&food.ProteinPer100g, &food.FatPer100g, &food.CarbsPer100g,
		)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, domain.ErrMealFoodNotFound
		}
		if err != nil {
			return nil, fmt.Errorf("get meal food: %w", err)
		}

		item := domain.MealItem{
			FoodID: entry.FoodID,
			FoodName: food.Name,
			Grams: entry.Grams,
			CaloriesPer100g: food.CaloriesPer100g,
			ProteinPer100g: food.ProteinPer100g,
			FatPer100g: food.FatPer100g,
			CarbsPer100g: food.CarbsPer100g,
		}
		err = tx.QueryRow(ctx, `
			INSERT INTO nutrition_meal_items (
				meal_id, food_id, food_name, grams,
				calories_per_100g, protein_per_100g,
				fat_per_100g, carbs_per_100g
			)
			VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
			RETURNING id::text
		`, created.ID, item.FoodID, item.FoodName, item.Grams,
			item.CaloriesPer100g, item.ProteinPer100g,
			item.FatPer100g, item.CarbsPer100g,
		).Scan(&item.ID)
		if err != nil {
			return nil, fmt.Errorf("create meal item: %w", err)
		}
		created.Items = append(created.Items, item)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit meal: %w", err)
	}
	return created, nil
}

func (r *MealRepo) ListMeals(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) ([]domain.Meal, error) {
	ctx, cancel := r.operationCtx(ctx)
	defer cancel()

	rows, err := r.pool.Query(ctx, `
		SELECT id::text, owner_user_id::text, meal_type, eaten_at, created_at
		FROM nutrition_meals
		WHERE owner_user_id = $1::uuid AND eaten_at >= $2 AND eaten_at < $3
		ORDER BY eaten_at DESC, id
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list meals: %w", err)
	}
	meals := make([]domain.Meal, 0)
	indices := make(map[string]int)
	for rows.Next() {
		var meal domain.Meal
		if err := rows.Scan(&meal.ID, &meal.OwnerID, &meal.MealType, &meal.EatenAt, &meal.CreatedAt); err != nil {
			rows.Close()
			return nil, fmt.Errorf("scan meal: %w", err)
		}
		meal.Items = make([]domain.MealItem, 0)
		indices[meal.ID] = len(meals)
		meals = append(meals, meal)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, fmt.Errorf("iterate meals: %w", err)
	}
	if len(meals) == 0 {
		return meals, nil
	}

	items, err := r.pool.Query(ctx, `
		SELECT i.meal_id::text, i.id::text, COALESCE(i.food_id::text, ''),
			i.food_name, i.grams::float8,
			i.calories_per_100g::float8, i.protein_per_100g::float8,
			i.fat_per_100g::float8, i.carbs_per_100g::float8
		FROM nutrition_meal_items i
		JOIN nutrition_meals m ON m.id = i.meal_id
		WHERE m.owner_user_id = $1::uuid AND m.eaten_at >= $2 AND m.eaten_at < $3
		ORDER BY m.eaten_at DESC, m.id, i.id
	`, userID, start, end)
	if err != nil {
		return nil, fmt.Errorf("list meal items: %w", err)
	}
	defer items.Close()

	for items.Next() {
		var mealID string
		var item domain.MealItem
		if err := items.Scan(&mealID, &item.ID, &item.FoodID,
			&item.FoodName, &item.Grams, &item.CaloriesPer100g,
			&item.ProteinPer100g, &item.FatPer100g,
			&item.CarbsPer100g); err != nil {
			return nil, fmt.Errorf("scan meal item: %w", err)
		}
		if i, ok := indices[mealID]; ok {
			meals[i].Items = append(meals[i].Items, item)
		}
	}
	if err := items.Err(); err != nil {
		return nil, fmt.Errorf("iterate meal items: %w", err)
	}
	return meals, nil
}

func (r *MealRepo) DeleteMeal(ctx context.Context, userID, mealID string) (bool, error) {
	ctx, cancel := r.operationCtx(ctx)
	defer cancel()

	tag, err := r.pool.Exec(ctx, `
		DELETE FROM nutrition_meals
		WHERE id = $1::uuid AND owner_user_id = $2::uuid
	`, mealID, userID)
	if err != nil {
		return false, fmt.Errorf("delete meal: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (r *MealRepo) GetDiarySummary(
	ctx context.Context,
	userID string,
	start time.Time,
	end time.Time,
) (*domain.DiarySummary, error) {
	ctx, cancel := r.operationCtx(ctx)
	defer cancel()

	summary := &domain.DiarySummary{}
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT m.id)::int, COUNT(i.id)::int,
			COALESCE(ROUND(SUM(i.grams * i.calories_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.protein_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.fat_per_100g / 100), 2), 0)::float8,
			COALESCE(ROUND(SUM(i.grams * i.carbs_per_100g / 100), 2), 0)::float8
		FROM nutrition_meals m
		LEFT JOIN nutrition_meal_items i ON i.meal_id = m.id
		WHERE m.owner_user_id = $1::uuid AND m.eaten_at >= $2 AND m.eaten_at < $3
	`, userID, start, end).Scan(
		&summary.Meals, &summary.Items, &summary.Calories,
		&summary.Protein, &summary.Fat, &summary.Carbs,
	)
	if err != nil {
		return nil, fmt.Errorf("get diary summary: %w", err)
	}
	return summary, nil
}
