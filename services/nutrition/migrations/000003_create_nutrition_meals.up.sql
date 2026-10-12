CREATE TABLE nutrition_meals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    owner_user_id UUID NOT NULL,
    meal_type TEXT NOT NULL,
    eaten_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT nutrition_meals_type_valid
        CHECK (LENGTH(BTRIM(meal_type)) BETWEEN 1 AND 60)
);

CREATE INDEX idx_nutrition_meals_owner_date
    ON nutrition_meals (owner_user_id, eaten_at DESC);

CREATE TABLE nutrition_meal_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    meal_id UUID NOT NULL REFERENCES nutrition_meals(id) ON DELETE CASCADE,
    food_id UUID REFERENCES foods(id) ON DELETE SET NULL,
    food_name TEXT NOT NULL,
    grams NUMERIC(9, 2) NOT NULL,
    calories_per_100g NUMERIC(7, 2) NOT NULL,
    protein_per_100g NUMERIC(6, 2) NOT NULL,
    fat_per_100g NUMERIC(6, 2) NOT NULL,
    carbs_per_100g NUMERIC(6, 2) NOT NULL,

    CONSTRAINT nutrition_meal_items_name_valid
        CHECK (LENGTH(BTRIM(food_name)) BETWEEN 1 AND 200),
    CONSTRAINT nutrition_meal_items_grams_valid
        CHECK (grams > 0 AND grams <= 10000),
    CONSTRAINT nutrition_meal_items_calories_valid
        CHECK (calories_per_100g BETWEEN 0 AND 1000),
    CONSTRAINT nutrition_meal_items_protein_valid
        CHECK (protein_per_100g BETWEEN 0 AND 100),
    CONSTRAINT nutrition_meal_items_fat_valid
        CHECK (fat_per_100g BETWEEN 0 AND 100),
    CONSTRAINT nutrition_meal_items_carbs_valid
        CHECK (carbs_per_100g BETWEEN 0 AND 100)
);

CREATE INDEX idx_nutrition_meal_items_meal_id
    ON nutrition_meal_items (meal_id);
