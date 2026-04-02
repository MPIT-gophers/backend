ALTER TABLE event_locations
    DROP COLUMN IF EXISTS cuisine,
    DROP COLUMN IF EXISTS avg_bill,
    DROP COLUMN IF EXISTS working_hours,
    DROP COLUMN IF EXISTS rating,
    DROP COLUMN IF EXISTS description,
    DROP COLUMN IF EXISTS image_url;
