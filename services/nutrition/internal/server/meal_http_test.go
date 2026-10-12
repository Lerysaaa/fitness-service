package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitness-platform/services/nutrition/internal/domain"
	postgresrepo "fitness-platform/services/nutrition/internal/repository/postgres"
)

func sendDiaryRequest(
	t *testing.T, client *http.Client, base, method, path, token, body string,
) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, base+path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("make diary request: %v", err)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("do diary request: %v", err)
	}
	defer res.Body.Close()
	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read diary response: %v", err)
	}
	return res.StatusCode, payload
}

func testMealDiaryHTTP(
	t *testing.T, ctx context.Context, pool *pgxpool.Pool,
	client *http.Client, base, tokenA, tokenB string,
) {
	t.Helper()
	foodRepo := postgresrepo.NewFoodRepo(pool, 2*time.Second)
	a, err := foodRepo.CreatePersonalFood(ctx, userA, &domain.Food{
		Name: "Diary oats", CaloriesPer100g: 180,
		ProteinPer100g: 6, FatPer100g: 4, CarbsPer100g: 30,
	})
	if err != nil {
		t.Fatalf("create user A meal food: %v", err)
	}
	b, err := foodRepo.CreatePersonalFood(ctx, userB, &domain.Food{
		Name: "Private B food", CaloriesPer100g: 100,
		ProteinPer100g: 5, FatPer100g: 2, CarbsPer100g: 15,
	})
	if err != nil {
		t.Fatalf("create user B food: %v", err)
	}
	var sharedID string
	err = pool.QueryRow(ctx, `
		INSERT INTO foods (
			name, calories_per_100g, protein_per_100g,
			fat_per_100g, carbs_per_100g
		) VALUES ('Shared diary food', 60, 3, 3, 4.8)
		RETURNING id::text
	`).Scan(&sharedID)
	if err != nil {
		t.Fatalf("create shared diary food: %v", err)
	}

	var created domain.Meal
	t.Run("reject unauthenticated and invalid meals", func(t *testing.T) {
		status, body := sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/meals?date=2026-10-12", "", "")
		checkStatus(t, status, http.StatusUnauthorized, body)
		status, body = sendDiaryRequest(t, client, base, http.MethodPost,
			"/nutrition/meals", tokenA,
			`{"meal_type":"breakfast","eaten_at":"2026-10-12T08:30:00Z","items":[{"food_id":"`+a.ID+`","grams":0}]}`)
		checkStatus(t, status, http.StatusBadRequest, body)
		status, body = sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/meals?date=not-a-date", tokenA, "")
		checkStatus(t, status, http.StatusBadRequest, body)
	})

	t.Run("reject foreign food atomically", func(t *testing.T) {
		bodyJSON := fmt.Sprintf(
			`{"meal_type":"breakfast","eaten_at":"2026-10-12T08:30:00Z","items":[{"food_id":"%s","grams":150},{"food_id":"%s","grams":100}]}`,
			a.ID, b.ID,
		)
		status, body := sendDiaryRequest(t, client, base, http.MethodPost,
			"/nutrition/meals", tokenA, bodyJSON)
		checkStatus(t, status, http.StatusNotFound, body)
		var count int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM nutrition_meals WHERE owner_user_id=$1::uuid`,
			userA).Scan(&count); err != nil || count != 0 {
			t.Fatalf("foreign food transaction not rolled back: count=%d err=%v", count, err)
		}
	})

	t.Run("create meal with personal and shared foods", func(t *testing.T) {
		bodyJSON := fmt.Sprintf(
			`{"meal_type":"breakfast","eaten_at":"2026-10-12T08:30:00Z","items":[{"food_id":"%s","grams":150},{"food_id":"%s","grams":100}]}`,
			a.ID, sharedID,
		)
		status, body := sendDiaryRequest(t, client, base, http.MethodPost,
			"/nutrition/meals", tokenA, bodyJSON)
		checkStatus(t, status, http.StatusCreated, body)
		if err := json.Unmarshal(body, &created); err != nil {
			t.Fatalf("decode meal: %v", err)
		}
		if created.ID == "" || created.OwnerID != userA || len(created.Items) != 2 {
			t.Fatal("incorrect saved meal")
		}
		if created.Calories != 330 || created.Protein != 12 ||
			created.Fat != 9 || created.Carbs != 49.8 {
			t.Fatalf("unexpected meal macros: %+v", created)
		}
	})

	t.Run("daily summary and user isolation", func(t *testing.T) {
		status, body := sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/diary/summary?date=2026-10-12", tokenA, "")
		checkStatus(t, status, http.StatusOK, body)
		var summary domain.DiarySummary
		if err := json.Unmarshal(body, &summary); err != nil {
			t.Fatalf("decode summary: %v", err)
		}
		if summary.Meals != 1 || summary.Items != 2 ||
			summary.Calories != 330 || summary.Carbs != 49.8 {
			t.Fatalf("wrong day totals: %+v", summary)
		}
		status, body = sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/meals?date=2026-10-12", tokenB, "")
		checkStatus(t, status, http.StatusOK, body)
		var foreign []domain.Meal
		if err := json.Unmarshal(body, &foreign); err != nil || len(foreign) != 0 {
			t.Fatalf("user B can read A diary: %s err=%v", body, err)
		}
		status, body = sendDiaryRequest(t, client, base, http.MethodDelete,
			"/nutrition/meals/"+created.ID, tokenB, "")
		checkStatus(t, status, http.StatusNotFound, body)
	})

	t.Run("history survives catalog update", func(t *testing.T) {
		_, err := foodRepo.UpdatePersonalFood(ctx, userA, a.ID, &domain.Food{
			Name: "Updated diary oats", CaloriesPer100g: 250,
			ProteinPer100g: 7, FatPer100g: 4, CarbsPer100g: 30,
		})
		if err != nil {
			t.Fatalf("update source food: %v", err)
		}
		status, body := sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/meals?date=2026-10-12", tokenA, "")
		checkStatus(t, status, http.StatusOK, body)
		var meals []domain.Meal
		if err := json.Unmarshal(body, &meals); err != nil {
			t.Fatalf("decode meals: %v", err)
		}
		if len(meals) != 1 || meals[0].Calories != 330 ||
			meals[0].Items[0].FoodName != "Diary oats" {
			t.Fatalf("meal snapshot changed: %+v", meals)
		}
	})

	t.Run("owner can delete meal", func(t *testing.T) {
		status, body := sendDiaryRequest(t, client, base, http.MethodDelete,
			"/nutrition/meals/"+created.ID, tokenA, "")
		checkStatus(t, status, http.StatusNoContent, body)
		status, body = sendDiaryRequest(t, client, base, http.MethodGet,
			"/nutrition/diary/summary?date=2026-10-12", tokenA, "")
		checkStatus(t, status, http.StatusOK, body)
		var summary domain.DiarySummary
		if err := json.Unmarshal(body, &summary); err != nil {
			t.Fatalf("decode empty summary: %v", err)
		}
		if summary.Meals != 0 || summary.Items != 0 || summary.Calories != 0 {
			t.Fatalf("deleted meal still counted: %+v", summary)
		}
	})
}
