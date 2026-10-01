-- +goose Up
-- Anonymous occupancy for resources, same rule as place_occupancy_days: a day was taken, not by whom.
CREATE TABLE resource_occupancy_days (
    resource_id UUID NOT NULL REFERENCES resources (id) ON DELETE CASCADE,
    day         DATE NOT NULL,
    occupied    BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (resource_id, day)
);

CREATE INDEX resource_occupancy_days_day_idx ON resource_occupancy_days (day);

-- +goose Down
DROP TABLE IF EXISTS resource_occupancy_days;
