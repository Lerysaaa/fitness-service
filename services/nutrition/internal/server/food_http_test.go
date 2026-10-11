package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"fitness-platform/services/nutrition/internal/domain"
	postgresrepo "fitness-platform/services/nutrition/internal/repository/postgres"
)

// Все HTTP-проверки продуктов используют PostgreSQL и сервер,
// уже запущенные родительским тестом.
func testFoodHTTP(
	t *testing.T,
	ctx context.Context,
	httpServer *httptest.Server,
	client *http.Client,
	tokenA string,
	tokenB string,
	foodRepo *postgresrepo.FoodRepo,
	pool *pgxpool.Pool,
) {
	t.Run("create food without JWT", func(t *testing.T) {
		status, body := sendCreateFoodRequest(
			t, client, httpServer.URL, "",
			`{"name":"Oatmeal","calories_per_100g":180,"protein_per_100g":6,"fat_per_100g":4,"carbs_per_100g":30}`,
		)

		checkStatus(t, status, http.StatusUnauthorized, body)
	})

	t.Run("create food with invalid data", func(t *testing.T) {
		status, body := sendCreateFoodRequest(
			t, client, httpServer.URL, tokenA,
			`{"name":"Invalid","calories_per_100g":200,"protein_per_100g":80,"fat_per_100g":80,"carbs_per_100g":80}`,
		)

		checkStatus(t, status, http.StatusBadRequest, body)
	})

	t.Run("create personal food", func(t *testing.T) {
		status, body := sendCreateFoodRequest(
			t, client, httpServer.URL, tokenA,
			`{"name":"  Oatmeal  ","brand":"Test","calories_per_100g":180,"protein_per_100g":6,"fat_per_100g":4,"carbs_per_100g":30}`,
		)

		checkStatus(t, status, http.StatusCreated, body)

		var created domain.Food
		if err := json.Unmarshal(body, &created); err != nil {
			t.Fatalf("decode created food: %v", err)
		}

		if created.ID == "" || created.OwnerUserID != userA {
			t.Fatal("incorrect food ID or owner")
		}

		if created.Name != "Oatmeal" {
			t.Fatal("food name was not normalized")
		}

		saved, err := foodRepo.GetVisibleFood(ctx, userA, created.ID)
		if err != nil || saved == nil {
			t.Fatalf("food was not saved: %v", err)
		}

		foreign, err := foodRepo.GetVisibleFood(ctx, userB, created.ID)
		if err != nil {
			t.Fatalf("check foreign access: %v", err)
		}

		if foreign != nil {
			t.Fatal("user B can access user A's food")
		}
	})

	t.Run("list and get foods", func(t *testing.T) {
		own, err := foodRepo.CreatePersonalFood(
			ctx,
			userA,
			&domain.Food{
				Name:            "GET Test Own Food",
				CaloriesPer100g: 180,
				ProteinPer100g:  6,
				FatPer100g:      4,
				CarbsPer100g:    30,
			},
		)
		if err != nil {
			t.Fatalf("create own food: %v", err)
		}

		foreign, err := foodRepo.CreatePersonalFood(
			ctx,
			userB,
			&domain.Food{
				Name:            "GET Test Foreign Food",
				CaloriesPer100g: 100,
				ProteinPer100g:  5,
				FatPer100g:      2,
				CarbsPer100g:    15,
			},
		)
		if err != nil {
			t.Fatalf("create foreign food: %v", err)
		}

		var sharedID string
		err = pool.QueryRow(ctx, `
			INSERT INTO foods (
				name,
				calories_per_100g,
				protein_per_100g,
				fat_per_100g,
				carbs_per_100g
			)
			VALUES ('GET Test Shared Food', 60, 3, 3.2, 4.8)
			RETURNING id::text
		`).Scan(&sharedID)
		if err != nil {
			t.Fatalf("create shared food: %v", err)
		}

		status, body := sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods", tokenA,
		)
		checkStatus(t, status, http.StatusOK, body)

		var foods []domain.Food
		if err := json.Unmarshal(body, &foods); err != nil {
			t.Fatalf("decode foods: %v", err)
		}

		foundOwn := false
		foundShared := false

		for _, food := range foods {
			if food.ID == own.ID {
				foundOwn = true
			}
			if food.ID == sharedID {
				foundShared = true
			}
			if food.ID == foreign.ID {
				t.Fatal("foreign food is visible in the list")
			}
		}

		if !foundOwn || !foundShared {
			t.Fatal("own or shared food missing from list")
		}

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods/"+own.ID, tokenA,
		)
		checkStatus(t, status, http.StatusOK, body)

		var result domain.Food
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("decode food: %v", err)
		}
		if result.ID != own.ID {
			t.Fatal("returned incorrect food")
		}

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods/"+own.ID, tokenB,
		)
		checkStatus(t, status, http.StatusNotFound, body)

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods/"+sharedID, tokenB,
		)
		checkStatus(t, status, http.StatusOK, body)

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods/invalid-id", tokenA,
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods?limit=101", tokenA,
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods?limit=1", tokenA,
		)
		checkStatus(t, status, http.StatusOK, body)

		var page []domain.Food
		if err := json.Unmarshal(body, &page); err != nil {
			t.Fatalf("decode food page: %v", err)
		}
		if len(page) != 1 {
			t.Fatalf("expected 1 food, got %d", len(page))
		}

		status, body = sendFoodGetRequest(
			t, client, httpServer.URL,
			"/nutrition/foods", "",
		)
		checkStatus(t, status, http.StatusUnauthorized, body)
	})

	t.Run("update and delete foods", func(t *testing.T) {
		own, err := foodRepo.CreatePersonalFood(
			ctx,
			userA,
			&domain.Food{
				Name:            "Editable Food",
				CaloriesPer100g: 180,
				ProteinPer100g:  6,
				FatPer100g:      4,
				CarbsPer100g:    30,
			},
		)
		if err != nil {
			t.Fatalf("create test food: %v", err)
		}

		var sharedID string
		err = pool.QueryRow(ctx, `
			INSERT INTO foods (
				name,
				calories_per_100g,
				protein_per_100g,
				fat_per_100g,
				carbs_per_100g
			)
			VALUES ('Protected Shared Food', 60, 3, 3, 5)
			RETURNING id::text
		`).Scan(&sharedID)
		if err != nil {
			t.Fatalf("create shared food: %v", err)
		}

		path := "/nutrition/foods/" + own.ID
		sharedPath := "/nutrition/foods/" + sharedID

		validBody := `{
			"name":"  Updated Food  ",
			"brand":"Test",
			"calories_per_100g":210,
			"protein_per_100g":8,
			"fat_per_100g":5,
			"carbs_per_100g":32
		}`

		// Без авторизации изменять продукт нельзя.
		for _, method := range []string{http.MethodPut, http.MethodDelete} {
			status, body := sendFoodMutationRequest(
				t, client, httpServer.URL,
				method, path, "", validBody,
			)
			checkStatus(t, status, http.StatusUnauthorized, body)
		}

		// Пользователь B не может изменить или удалить продукт A.
		status, body := sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, path, tokenB, validBody,
		)
		checkStatus(t, status, http.StatusNotFound, body)

		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodDelete, path, tokenB, "",
		)
		checkStatus(t, status, http.StatusNotFound, body)

		// Общие продукты запрещено изменять и удалять.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, sharedPath, tokenA, validBody,
		)
		checkStatus(t, status, http.StatusNotFound, body)

		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodDelete, sharedPath, tokenA, "",
		)
		checkStatus(t, status, http.StatusNotFound, body)

		// Некорректные данные не должны обновлять продукт.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, path, tokenA,
			`{"name":"Incomplete Food","calories_per_100g":100}`,
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, "/nutrition/foods/invalid-id",
			tokenA, validBody,
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		// Владелец успешно изменяет продукт.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, path, tokenA, validBody,
		)
		checkStatus(t, status, http.StatusOK, body)

		var updated domain.Food
		if err := json.Unmarshal(body, &updated); err != nil {
			t.Fatalf("decode updated food: %v", err)
		}

		if updated.ID != own.ID ||
			updated.OwnerUserID != userA ||
			updated.Name != "Updated Food" ||
			updated.CaloriesPer100g != 210 {
			t.Fatal("incorrect updated food")
		}

		saved, err := foodRepo.GetVisibleFood(ctx, userA, own.ID)
		if err != nil || saved == nil {
			t.Fatalf("get updated food: %v", err)
		}
		if saved.Name != "Updated Food" {
			t.Fatal("updated food was not saved")
		}

		// Некорректный ID при удалении возвращает 400.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodDelete, "/nutrition/foods/invalid-id",
			tokenA, "",
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		// Владелец успешно удаляет продукт.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodDelete, path, tokenA, "",
		)
		checkStatus(t, status, http.StatusNoContent, body)

		if len(body) != 0 {
			t.Fatal("DELETE response must have an empty body")
		}

		// Удалённый продукт больше не доступен через GET.
		status, body = sendFoodGetRequest(
			t, client, httpServer.URL, path, tokenA,
		)
		checkStatus(t, status, http.StatusNotFound, body)

		// Повторное удаление возвращает 404.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodDelete, path, tokenA, "",
		)
		checkStatus(t, status, http.StatusNotFound, body)

		// После удаления PUT также должен возвращать 404.
		status, body = sendFoodMutationRequest(
			t, client, httpServer.URL,
			http.MethodPut, path, tokenA, validBody,
		)
		checkStatus(t, status, http.StatusNotFound, body)

		// Запись сохраняется в базе с заполненным deleted_at.
		var softDeleted bool
		err = pool.QueryRow(ctx, `
			SELECT deleted_at IS NOT NULL
			FROM foods
			WHERE id = $1::uuid
		`, own.ID).Scan(&softDeleted)

		if err != nil {
			t.Fatalf("check soft delete: %v", err)
		}
		if !softDeleted {
			t.Fatal("food was not soft deleted")
		}
	})

	t.Run("invalid food JSON", func(t *testing.T) {
		valid := `{"name":"Water","calories_per_100g":0,"protein_per_100g":0,"fat_per_100g":0,"carbs_per_100g":0}`

		cases := []struct {
			name string
			body string
			want int
		}{
			{"unknown field", `{"name":"Water","owner_user_id":"` + userB + `","calories_per_100g":0,"protein_per_100g":0,"fat_per_100g":0,"carbs_per_100g":0}`, http.StatusBadRequest},
			{"trailing JSON", valid + `{}`, http.StatusBadRequest},
			{"missing values", `{"name":"Water"}`, http.StatusBadRequest},
			{"oversized body", valid + strings.Repeat(" ", (1<<20)+1), http.StatusRequestEntityTooLarge},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				status, body := sendCreateFoodRequest(t, client, httpServer.URL, tokenA, tc.body)
				checkStatus(t, status, tc.want, body)
			})
		}
	})

	t.Run("invalid food pagination", func(t *testing.T) {
		for _, suffix := range []string{"?limit=abc", "?offset=-1", "?limit=0"} {
			status, body := sendFoodGetRequest(t, client, httpServer.URL, "/nutrition/foods"+suffix, tokenA)
			checkStatus(t, status, http.StatusBadRequest, body)
		}
	})
}

func sendCreateFoodRequest(
	t *testing.T,
	client *http.Client,
	baseURL string,
	token string,
	body string,
) (int, []byte) {
	return sendTestRequest(t, client, baseURL, http.MethodPost, "/nutrition/foods", token, body)
}

// sendFoodGetRequest отправляет GET-запрос к каталогу продуктов.
func sendFoodGetRequest(
	t *testing.T,
	client *http.Client,
	baseURL string,
	path string,
	token string,
) (int, []byte) {
	return sendTestRequest(t, client, baseURL, http.MethodGet, path, token, "")
}

// sendFoodMutationRequest отправляет PUT или DELETE в HTTP-тестах.
func sendFoodMutationRequest(
	t *testing.T,
	client *http.Client,
	baseURL string,
	method string,
	path string,
	token string,
	body string,
) (int, []byte) {
	return sendTestRequest(t, client, baseURL, method, path, token, body)
}
