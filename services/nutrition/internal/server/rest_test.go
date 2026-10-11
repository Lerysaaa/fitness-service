package server

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"fitness-platform/services/nutrition/internal/handler"
	postgresrepo "fitness-platform/services/nutrition/internal/repository/postgres"
	"fitness-platform/services/nutrition/internal/service"
)

const testJWTSecret = "nutrition-http-test-secret"

const userA = "550e8400-e29b-41d4-a716-446655440001"
const userB = "550e8400-e29b-41d4-a716-446655440002"

func TestNutritionProfileHTTP(t *testing.T) {
	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Minute,
	)
	defer cancel()

	// 1. Запускаем PostgreSQL через Testcontainers.
	container, err := tcpostgres.Run(
		ctx,
		"postgres:15-alpine",
		tcpostgres.WithDatabase("nutrition_http_test"),
		tcpostgres.WithUsername("admin"),
		tcpostgres.WithPassword("secret"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		t.Fatalf("start PostgreSQL: %v", err)
	}

	testcontainers.CleanupContainer(t, container)

	dbURL, err := container.ConnectionString(
		ctx,
		"sslmode=disable",
	)
	if err != nil {
		t.Fatalf("get database URL: %v", err)
	}

	// 2. Применяем настоящие миграции.
	source, err := iofs.New(
		os.DirFS("../../migrations"),
		".",
	)
	if err != nil {
		t.Fatalf("open migrations: %v", err)
	}

	migrationURL := strings.Replace(
		dbURL,
		"postgres://",
		"pgx5://",
		1,
	)

	migrator, err := migrate.NewWithSourceInstance(
		"iofs",
		source,
		migrationURL,
	)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}

	t.Cleanup(func() {
		sourceErr, databaseErr := migrator.Close()

		if sourceErr != nil {
			t.Errorf("close migration source: %v", sourceErr)
		}
		if databaseErr != nil {
			t.Errorf("close migration database: %v", databaseErr)
		}
	})

	if err := migrator.Up(); err != nil &&
		!errors.Is(err, migrate.ErrNoChange) {
		t.Fatalf("apply migrations: %v", err)
	}

	// 3. Подключаем настоящий PostgreSQL Repository.
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("create database pool: %v", err)
	}

	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping PostgreSQL: %v", err)
	}

	repo := postgresrepo.NewNutritionRepo(
		pool,
		2*time.Second,
	)

	// 4. Создаём настоящий Service и Handler.
	nutritionService := service.NewNutritionService(repo)
	nutritionHandler := handler.NewNutritionHandler(
		nutritionService,
	)

	foodRepo := postgresrepo.NewFoodRepo(
		pool,
		2*time.Second,
	)

	foodService := service.NewFoodService(foodRepo)
	foodHandler := handler.NewFoodHandler(foodService)

	// 5. Запускаем временный HTTP-сервер.
	httpServer := httptest.NewServer(
		NewRouter(
			nutritionHandler,
			foodHandler,
			testJWTSecret,
			time.Second,
			func(ctx context.Context) error {
				return pool.Ping(ctx)
			},
		),
	)
	t.Cleanup(httpServer.Close)

	client := httpServer.Client()
	client.Timeout = 5 * time.Second

	// Создаём тестовые JWT для двух пользователей.
	tokenA := makeTestJWT(t, userA)
	tokenB := makeTestJWT(t, userB)

	t.Run("profiles", func(t *testing.T) {
		testProfileHTTP(t, client, httpServer.URL, tokenA, tokenB)
	})

	t.Run("foods", func(t *testing.T) {
		testFoodHTTP(t, ctx, httpServer, client, tokenA, tokenB, foodRepo, pool)
	})

}

// Создаёт JWT с указанным userID.
func makeTestJWT(t *testing.T, userID string) string {
	t.Helper()

	token := jwt.NewWithClaims(
		jwt.SigningMethodHS256,
		jwt.MapClaims{
			"sub": userID,
			"exp": time.Now().Add(time.Hour).Unix(),
		},
	)

	signed, err := token.SignedString(
		[]byte(testJWTSecret),
	)
	if err != nil {
		t.Fatalf("sign JWT: %v", err)
	}

	return signed
}

// Проверяет HTTP-статус.
func checkStatus(
	t *testing.T,
	got int,
	want int,
	body []byte,
) {
	t.Helper()

	if got != want {
		t.Fatalf(
			"expected HTTP %d, got %d; body: %s",
			want,
			got,
			body,
		)
	}
}

