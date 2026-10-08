package service

import (
	"context"
	"errors"
	"testing"

	"fitness-platform/services/nutrition/internal/domain"
)

// Поддельный Repository для тестов.
type fakeNutritionRepository struct {
	profile      *domain.NutritionProfile
	upsertCalled bool
	getCalled    bool
	saveError    error
}

// Имитируем сохранение профиля в базу.
func (f *fakeNutritionRepository) UpsertProfile(
	ctx context.Context,
	profile *domain.NutritionProfile,
) error {
	f.upsertCalled = true

	if f.saveError != nil {
		return f.saveError
	}

	f.profile = profile
	return nil
}

// Имитируем получение профиля из базы.
func (f *fakeNutritionRepository) GetProfile(
	ctx context.Context,
	userID string,
) (*domain.NutritionProfile, error) {
	f.getCalled = true

	if f.profile == nil || f.profile.UserID != userID {
		return nil, nil
	}

	return f.profile, nil
}

// Создаём корректный профиль для тестов.
func validTestProfile() *domain.NutritionProfile {
	return &domain.NutritionProfile{
		UserID:        "550e8400-e29b-41d4-a716-446655440000",
		Age:           20,
		Sex:           "female",
		HeightCm:      168,
		WeightKg:      60,
		ActivityLevel: "moderate",
		Goal:          "maintain",
	}
}

// Тест 1: корректный профиль сохраняется.
func TestUpsertProfileSuccess(t *testing.T) {
	repo := &fakeNutritionRepository{}
	svc := NewNutritionService(repo)

	profile := validTestProfile()

	result, err := svc.UpsertProfile(
		context.Background(),
		profile,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.upsertCalled {
		t.Fatal("repository UpsertProfile was not called")
	}

	if !repo.getCalled {
		t.Fatal("repository GetProfile was not called")
	}

	if result == nil || result.WeightKg != 60 {
		t.Fatal("incorrect saved profile")
	}
}

// Тест 2: отрицательный возраст запрещён.
func TestUpsertProfileInvalidAge(t *testing.T) {
	repo := &fakeNutritionRepository{}
	svc := NewNutritionService(repo)

	profile := validTestProfile()
	profile.Age = -5

	_, err := svc.UpsertProfile(
		context.Background(),
		profile,
	)

	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got: %v", err)
	}

	if repo.upsertCalled {
		t.Fatal("repository must not be called")
	}
}

// Тест 3: Service возвращает ошибку Repository.
func TestUpsertProfileRepositoryError(t *testing.T) {
	dbErr := errors.New("database unavailable")

	repo := &fakeNutritionRepository{
		saveError: dbErr,
	}
	svc := NewNutritionService(repo)

	_, err := svc.UpsertProfile(
		context.Background(),
		validTestProfile(),
	)

	if !errors.Is(err, dbErr) {
		t.Fatalf("expected database error, got: %v", err)
	}

	if repo.getCalled {
		t.Fatal("GetProfile must not be called after save error")
	}
}

// Тест 4: отсутствующий профиль возвращает nil.
func TestGetProfileNotFound(t *testing.T) {
	repo := &fakeNutritionRepository{}
	svc := NewNutritionService(repo)

	profile, err := svc.GetProfile(
		context.Background(),
		"550e8400-e29b-41d4-a716-446655440000",
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if profile != nil {
		t.Fatal("expected nil profile")
	}
}
