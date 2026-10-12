# Nutrition meal diary

JWT-protected meal diary backed by PostgreSQL. Requires migration `000003_create_nutrition_meals`.

## Endpoints

| Method | Path | Purpose |
|---|---|---|
| POST | `/nutrition/meals` | Create a meal with 1–50 food items |
| GET | `/nutrition/meals?date=2026-10-12` | List the signed-in user's meals for a UTC day |
| DELETE | `/nutrition/meals/{id}` | Delete an owned meal |
| GET | `/nutrition/diary/summary?date=2026-10-12` | Daily calories, protein, fat and carbs |
| GET | `/nutrition/diary/weekly?start=2026-10-12` | 7-day intake, logged-day averages and daily target comparison |

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

## Weekly diary report

`GET /nutrition/diary/weekly?start=2026-10-12` reports seven consecutive UTC dates starting with the specified `YYYY-MM-DD` day. If `start` is omitted, the current UTC week's Monday is used. The query aggregates all seven days in one PostgreSQL query, scoped to the JWT owner.

The response contains `days` (seven dates, including dates without recorded meals), `totals` (calories and macros), `tracked_days` (dates with at least one recorded meal), and `average_calories_per_tracked_day`. Missing log entries are shown as zeros but **must not** be interpreted as actual zero food intake.

When the user has a nutrition profile, `daily_targets` contains the existing calorie/macro targets. Only logged days include `calories_vs_target` (consumed minus target; negative means below target). The optional `average_calories_vs_target_tracked_day` compares the logged-day average with the target. Without a profile, target fields are omitted.

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
