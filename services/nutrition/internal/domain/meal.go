package domain

import (
	"errors"
	"time"
)

var ErrMealFoodNotFound = errors.New("food not found or not accessible")

type MealItem struct {
	ID              string  `json:"id"`
	FoodID          string  `json:"food_id,omitempty"`
	FoodName        string  `json:"food_name"`
	Grams           float64 `json:"grams"`
	CaloriesPer100g float64 `json:"calories_per_100g"`
	ProteinPer100g  float64 `json:"protein_per_100g"`
	FatPer100g      float64 `json:"fat_per_100g"`
	CarbsPer100g    float64 `json:"carbs_per_100g"`
	Calories        float64 `json:"calories"`
	Protein         float64 `json:"protein"`
	Fat             float64 `json:"fat"`
	Carbs           float64 `json:"carbs"`
}

type Meal struct {
	ID        string     `json:"id"`
	OwnerID   string     `json:"owner_user_id"`
	MealType  string     `json:"meal_type"`
	EatenAt   time.Time  `json:"eaten_at"`
	CreatedAt time.Time  `json:"created_at"`
	Items     []MealItem `json:"items"`
	Calories  float64    `json:"calories"`
	Protein   float64    `json:"protein"`
	Fat       float64    `json:"fat"`
	Carbs     float64    `json:"carbs"`
}

type DiarySummary struct {
	Date     string  `json:"date"`
	Meals    int     `json:"meals"`
	Items    int     `json:"items"`
	Calories float64 `json:"calories"`
	Protein  float64 `json:"protein"`
	Fat      float64 `json:"fat"`
	Carbs    float64 `json:"carbs"`
}
