-- +goose Up
CREATE TABLE resource_groups (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT resource_groups_tenant_name_unique UNIQUE (tenant_id, name)
);

ALTER TABLE resources
    ADD COLUMN group_id UUID REFERENCES resource_groups (id) ON DELETE RESTRICT,
    ADD COLUMN seats INT,
    ADD CONSTRAINT resources_seats_positive CHECK (seats IS NULL OR seats > 0);

INSERT INTO resource_groups (tenant_id, name)
SELECT DISTINCT r.tenant_id, 'Allgemein'
FROM resources r
WHERE NOT EXISTS (
    SELECT 1 FROM resource_groups g
    WHERE g.tenant_id = r.tenant_id AND g.name = 'Allgemein'
);

UPDATE resources r
SET group_id = g.id
FROM resource_groups g
WHERE r.group_id IS NULL
  AND g.tenant_id = r.tenant_id
  AND g.name = 'Allgemein';

ALTER TABLE resources
    ALTER COLUMN group_id SET NOT NULL;

ALTER TABLE resources
    DROP CONSTRAINT resources_tenant_location_name_unique;

ALTER TABLE resources
    ADD CONSTRAINT resources_group_location_name_unique UNIQUE (group_id, location_id, name);

CREATE INDEX resources_group_idx ON resources (group_id);

-- +goose Down
DROP INDEX IF EXISTS resources_group_idx;

ALTER TABLE resources
    DROP CONSTRAINT IF EXISTS resources_group_location_name_unique;

ALTER TABLE resources
    DROP CONSTRAINT IF EXISTS resources_seats_positive;

ALTER TABLE resources
    DROP COLUMN IF EXISTS seats,
    DROP COLUMN IF EXISTS group_id;

ALTER TABLE resources
    ADD CONSTRAINT resources_tenant_location_name_unique UNIQUE (tenant_id, location_id, name);

DROP TABLE IF EXISTS resource_groups;
