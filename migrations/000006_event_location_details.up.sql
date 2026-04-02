ALTER TABLE event_locations
    ADD COLUMN IF NOT EXISTS image_url TEXT,
    ADD COLUMN IF NOT EXISTS description TEXT,
    ADD COLUMN IF NOT EXISTS rating NUMERIC(3,2),
    ADD COLUMN IF NOT EXISTS working_hours TEXT,
    ADD COLUMN IF NOT EXISTS avg_bill TEXT,
    ADD COLUMN IF NOT EXISTS cuisine TEXT;

TRUNCATE TABLE
    event_locations,
    event_variants,
    event_guests,
    event_invites,
    event_users,
    events
RESTART IDENTITY CASCADE;
