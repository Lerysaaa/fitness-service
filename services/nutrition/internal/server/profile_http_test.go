package server

import (
	"encoding/json"
	"io"
	"math"
	"net/http"
	"strings"
	"testing"
	"time"

	"fitness-platform/services/nutrition/internal/domain"
	"fitness-platform/services/nutrition/internal/service"
)

// Запускает сценарии профиля на HTTP-сервере из родительского теста.
func testProfileHTTP(
	t *testing.T,
	client *http.Client,
	baseURL string,
	tokenA string,
	tokenB string,
) {
	initialBody := `{
		"age": 20,
		"sex": "female",
		"height_cm": 168,
		"weight_kg": 60,
		"activity_level": "moderate",
		"goal": "maintain"
	}`

	// Тест 1. Нельзя обращаться без JWT.
	t.Run("unauthorized", func(t *testing.T) {
		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, "", "",
		)

		checkStatus(t, status, http.StatusUnauthorized, body)

		status, body = sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, "", initialBody,
		)

		checkStatus(t, status, http.StatusUnauthorized, body)
	})

	// Тест 2. Несуществующий профиль возвращает 404.
	t.Run("profile not found", func(t *testing.T) {
		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, tokenA, "",
		)

		checkStatus(t, status, http.StatusNotFound, body)
	})

	var createdAt time.Time

	// Тест 3. Создание профиля через HTTP.
	t.Run("create profile", func(t *testing.T) {
		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, tokenA, initialBody,
		)

		checkStatus(t, status, http.StatusOK, body)

		profile := decodeProfile(t, body)

		if profile.UserID != userA {
			t.Fatalf(
				"expected user %s, got %s",
				userA,
				profile.UserID,
			)
		}

		if profile.WeightKg != 60 {
			t.Fatalf(
				"expected weight 60, got %v",
				profile.WeightKg,
			)
		}

		if profile.CreatedAt.IsZero() {
			t.Fatal("created_at is empty")
		}

		createdAt = profile.CreatedAt
	})

	// Тест 4. Получение сохранённого профиля.
	t.Run("get profile", func(t *testing.T) {
		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, tokenA, "",
		)

		checkStatus(t, status, http.StatusOK, body)

		profile := decodeProfile(t, body)

		if profile.UserID != userA {
			t.Fatal("incorrect user ID")
		}

		if profile.WeightKg != 60 {
			t.Fatal("incorrect profile weight")
		}
	})

	// Тест 5. Обновление веса через POST.
	t.Run("update profile", func(t *testing.T) {
		updatedBody := `{
			"age": 20,
			"sex": "female",
			"height_cm": 168,
			"weight_kg": 58.5,
			"activity_level": "moderate",
			"goal": "maintain"
		}`

		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, tokenA, updatedBody,
		)

		checkStatus(t, status, http.StatusOK, body)

		profile := decodeProfile(t, body)

		if profile.WeightKg != 58.5 {
			t.Fatalf(
				"expected weight 58.5, got %v",
				profile.WeightKg,
			)
		}

		if !profile.CreatedAt.Equal(createdAt) {
			t.Fatal("created_at changed during update")
		}
	})

	// Тест 6. Некорректный возраст возвращает 400.
	t.Run("invalid age", func(t *testing.T) {
		badBody := `{
			"age": -5,
			"sex": "female",
			"height_cm": 168,
			"weight_kg": 60,
			"activity_level": "moderate",
			"goal": "maintain"
		}`

		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, tokenA, badBody,
		)

		checkStatus(t, status, http.StatusBadRequest, body)

		// Тест 7. Некорректая цель питания не проходит
		t.Run("invalid goal", func(t *testing.T) {
			badBody := `{
			"age": 20,
			"sex": "female",
			"height_cm": 168,
			"weight_kg": 60,
			"activity_level": "moderate",
			"goal": "pizza"
		}`

			status, body := sendProfileRequest(
				t,
				client,
				baseURL,
				http.MethodPost,
				tokenA,
				badBody,
			)

			checkStatus(
				t,
				status,
				http.StatusBadRequest,
				body,
			)
		})

		// Некорректный POST не должен менять вес.
		status, body = sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, tokenA, "",
		)

		checkStatus(t, status, http.StatusOK, body)

		profile := decodeProfile(t, body)

		if profile.WeightKg != 58.5 {
			t.Fatal("invalid request changed stored profile")
		}
	})

	// Тест 7. Пользователи не видят чужие профили.
	t.Run("user isolation", func(t *testing.T) {
		// Пользователь B ещё не создавал профиль.
		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, tokenB, "",
		)

		checkStatus(t, status, http.StatusNotFound, body)

		// B пытается указать чужой user_id в JSON.
		spoofedBody := `{
			"user_id": "550e8400-e29b-41d4-a716-446655440001",
			"age": 25,
			"sex": "male",
			"height_cm": 180,
			"weight_kg": 80,
			"activity_level": "high",
			"goal": "maintain"
		}`

		status, body = sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, tokenB, spoofedBody,
		)

		checkStatus(t, status, http.StatusOK, body)

		profileB := decodeProfile(t, body)

		// Профиль должен принадлежать B, а не A.
		if profileB.UserID != userB {
			t.Fatalf(
				"expected user B, got %s",
				profileB.UserID,
			)
		}

		// Проверяем, что данные пользователя A не изменились.
		status, body = sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, tokenA, "",
		)

		checkStatus(t, status, http.StatusOK, body)

		profileA := decodeProfile(t, body)

		if profileA.WeightKg != 58.5 {
			t.Fatal("user A profile was changed by user B")
		}
	})

	t.Run("targets without JWT", func(t *testing.T) {
		status, body := sendTargetsRequest(
			t, client, baseURL, "",
		)

		checkStatus(
			t,
			status,
			http.StatusUnauthorized,
			body,
		)
	})

	t.Run("targets without profile", func(t *testing.T) {
		tokenC := makeTestJWT(
			t,
			"550e8400-e29b-41d4-a716-446655440003",
		)

		status, body := sendTargetsRequest(
			t, client, baseURL, tokenC,
		)

		checkStatus(
			t,
			status,
			http.StatusNotFound,
			body,
		)
	})

	t.Run("targets for user A", func(t *testing.T) {
		status, body := sendTargetsRequest(
			t, client, baseURL, tokenA,
		)

		checkStatus(
			t,
			status,
			http.StatusOK,
			body,
		)

		var targets service.NutritionTargets
		if err := json.Unmarshal(body, &targets); err != nil {
			t.Fatalf("decode targets: %v", err)
		}

		// В предыдущем сценарии пользователь A
		// обновил вес с 60 до 58.5 кг.
		// Поэтому BMR и TDEE должны пересчитаться.
		if math.Abs(targets.BMR-1374) > 0.000001 {
			t.Fatalf("unexpected BMR: %v", targets.BMR)
		}

		if math.Abs(targets.TDEE-2129.7) > 0.000001 {
			t.Fatalf("unexpected TDEE: %v", targets.TDEE)
		}

		if math.Abs(targets.DailyCalories-2129.7) > 0.000001 {
			t.Fatalf(
				"unexpected daily calories: %v",
				targets.DailyCalories,
			)
		}

		// Проверяем, что количество калорий,
		// рассчитанное по БЖУ, совпадает с целью.
		totalCalories :=
			targets.ProteinGrams*4 +
				targets.FatGrams*9 +
				targets.CarbsGrams*4

		if math.Abs(totalCalories-targets.DailyCalories) > 0.000001 {
			t.Fatal("macros do not match daily calories")
		}
	})

	t.Run("targets for user B", func(t *testing.T) {
		status, body := sendTargetsRequest(
			t, client, baseURL, tokenB,
		)

		checkStatus(
			t,
			status,
			http.StatusOK,
			body,
		)

		var targets service.NutritionTargets
		if err := json.Unmarshal(body, &targets); err != nil {
			t.Fatalf("decode targets: %v", err)
		}

		// Пользователь B имеет другой профиль:
		// male, 25 лет, 180 см, 80 кг, high.
		// Проверяем, что расчёт выполнен по его данным.
		if math.Abs(targets.BMR-1805) > 0.000001 {
			t.Fatalf("unexpected BMR: %v", targets.BMR)
		}

		if math.Abs(targets.TDEE-3113.625) > 0.000001 {
			t.Fatalf("unexpected TDEE: %v", targets.TDEE)
		}

		if math.Abs(targets.DailyCalories-3113.625) > 0.000001 {
			t.Fatalf(
				"unexpected daily calories: %v",
				targets.DailyCalories,
			)
		}
	})

	t.Run("profile JSON validation", func(t *testing.T) {
		valid := `{"age":20,"sex":"female","height_cm":168,"weight_kg":60,"activity_level":"moderate","goal":"maintain"}`

		tests := []struct {
			name string
			body string
			want int
		}{
			{"unknown field", `{"age":20,"sex":"female","height_cm":168,"weight_kg":60,"activity_level":"moderate","goal":"maintain","unexpected":true}`, http.StatusBadRequest},
			{"trailing JSON", valid + `{}`, http.StatusBadRequest},
			{"oversized body", valid + strings.Repeat(" ", (1<<20)+1), http.StatusRequestEntityTooLarge},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				status, body := sendProfileRequest(t, client, baseURL, http.MethodPost, tokenA, tc.body)
				checkStatus(t, status, tc.want, body)
			})
		}
	})

	t.Run("invalid profile user UUID", func(t *testing.T) {
		invalidToken := makeTestJWT(t, "not-a-uuid")

		status, body := sendProfileRequest(
			t, client, baseURL,
			http.MethodGet, invalidToken, "",
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		status, body = sendProfileRequest(
			t, client, baseURL,
			http.MethodPost, invalidToken, initialBody,
		)
		checkStatus(t, status, http.StatusBadRequest, body)

		status, body = sendTargetsRequest(t, client, baseURL, invalidToken)
		checkStatus(t, status, http.StatusBadRequest, body)
	})
}

// Отправляет реальный HTTP-запрос тестовому серверу.
func sendProfileRequest(
	t *testing.T,
	client *http.Client,
	baseURL string,
	method string,
	token string,
	body string,
) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(
		method,
		baseURL+"/nutrition/profile",
		strings.NewReader(body),
	)
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}

	if body != "" {
		req.Header.Set(
			"Content-Type",
			"application/json",
		)
	}

	if token != "" {
		req.Header.Set(
			"Authorization",
			"Bearer "+token,
		)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read HTTP response: %v", err)
	}

	return resp.StatusCode, responseBody
}

// Превращает JSON-ответ обратно в Go-структуру.
func decodeProfile(
	t *testing.T,
	body []byte,
) domain.NutritionProfile {
	t.Helper()

	var profile domain.NutritionProfile

	if err := json.Unmarshal(body, &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}

	return profile
}

// sendTargetsRequest отправляет GET-запрос к API дневных норм.
// Используется только в HTTP-тестах.
func sendTargetsRequest(
	t *testing.T,
	client *http.Client,
	baseURL string,
	token string,
) (int, []byte) {
	t.Helper()

	req, err := http.NewRequest(
		http.MethodGet,
		baseURL+"/nutrition/targets",
		nil,
	)
	if err != nil {
		t.Fatalf("create HTTP request: %v", err)
	}

	if token != "" {
		req.Header.Set(
			"Authorization",
			"Bearer "+token,
		)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("send HTTP request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	return resp.StatusCode, body
}
