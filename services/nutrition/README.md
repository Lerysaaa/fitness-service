# Nutrition meal diary

JWT-protected meal diary backed by PostgreSQL. Requires migration `000003_create_nutrition_meals`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/nutrition/meals` | Create a meal with 1–50 food items |
| GET | `/nutrition/meals?date=2026-10-12` | List the signed-in user's meals for a UTC day |
| DELETE | `/nutrition/meals/{id}` | Delete an owned meal |
| GET | `/nutrition/diary/summary?date=2026-10-12` | Daily calories, protein, fat and carbs |

All endpoints require `Authorization: Bearer <access_token>`. If `date` is omitted, the current UTC day is used. Dates are `YYYY-MM-DD`. Meal timestamps are RFC3339 with timezone, and daily boundaries use UTC (a future feature may support user time zones).

### Create a meal

```json
{
  "meal_type": "breakfast",
  "eaten_at": "2026-10-12T08:30:00+03:00",
  "items": [
    { "food_id": "550e8400-e29b-41d4-a716-446655440001", "grams": 150 }
  ]
}
```

Replace `food_id` with an actual shared food or a personal food owned by the authenticated user. Supported meal types: `breakfast`, `lunch`, `dinner`, `snack`. Grams must be greater than zero and at most 10,000 with two decimal places.

The server looks up product values itself and atomically saves a snapshot of food name and nutrients per 100 g. A later product edit or deletion does not rewrite the meal history. Users cannot access another user's meals or private foods.

Responses: POST `201`; list and summary `200`; successful DELETE `204`; invalid input `400`; missing or forbidden resource `404`; absent/invalid JWT `401`.

## Testing

From repository root with Docker Desktop running:

```powershell
go test -count=1 ./services/nutrition/...
```

The HTTP and repository integration suites use Testcontainers PostgreSQL and run the real migrations. A sample request in PowerShell after obtaining a JWT:

```powershell
$headers = @{ Authorization = "Bearer $token" }
$today = [DateTimeOffset]::UtcNow.ToString("yyyy-MM-dd")
Invoke-RestMethod -Uri "http://localhost:8084/nutrition/diary/summary?date=$today" -Headers $headers
```
