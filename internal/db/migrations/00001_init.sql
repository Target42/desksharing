-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TYPE tenant_status AS ENUM ('pending', 'active', 'disabled');
CREATE TYPE user_role AS ENUM ('platform_admin', 'tenant_admin', 'employee');
CREATE TYPE identity_provider AS ENUM ('local', 'ldap');
CREATE TYPE booking_mode AS ENUM ('daily', 'hourly');
CREATE TYPE booking_status AS ENUM (
    'booked',
    'checked_in',
    'checked_out',
    'cancelled',
    'possibly_free'
);

CREATE TABLE tenants (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT NOT NULL,
    slug            TEXT NOT NULL,
    status          tenant_status NOT NULL DEFAULT 'pending',
    ldap_enabled    BOOLEAN NOT NULL DEFAULT FALSE,
    ldap_config     JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    confirmed_at    TIMESTAMPTZ,
    CONSTRAINT tenants_slug_unique UNIQUE (slug)
);

CREATE TABLE users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id           UUID REFERENCES tenants (id) ON DELETE CASCADE,
    email               TEXT NOT NULL,
    display_name        TEXT NOT NULL DEFAULT '',
    role                user_role NOT NULL DEFAULT 'employee',
    identity_provider   identity_provider NOT NULL DEFAULT 'local',
    password_hash       TEXT,
    external_subject    TEXT,
    can_block           BOOLEAN NOT NULL DEFAULT TRUE,
    email_verified_at   TIMESTAMPTZ,
    disabled_at         TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT users_platform_admin_tenant CHECK (
        (role = 'platform_admin' AND tenant_id IS NULL)
        OR (role <> 'platform_admin' AND tenant_id IS NOT NULL)
    ),
    CONSTRAINT users_ldap_external CHECK (
        identity_provider <> 'ldap' OR external_subject IS NOT NULL
    )
);

CREATE UNIQUE INDEX users_platform_email_uidx
    ON users (lower(email))
    WHERE tenant_id IS NULL;

CREATE UNIQUE INDEX users_tenant_email_uidx
    ON users (tenant_id, lower(email))
    WHERE tenant_id IS NOT NULL;

CREATE UNIQUE INDEX users_tenant_external_uidx
    ON users (tenant_id, identity_provider, external_subject)
    WHERE external_subject IS NOT NULL;

CREATE TABLE auth_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users (id) ON DELETE CASCADE,
    tenant_id   UUID REFERENCES tenants (id) ON DELETE CASCADE,
    purpose     TEXT NOT NULL,
    token_hash  TEXT NOT NULL,
    expires_at  TIMESTAMPTZ NOT NULL,
    used_at     TIMESTAMPTZ,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT auth_tokens_subject CHECK (
        user_id IS NOT NULL OR tenant_id IS NOT NULL
    ),
    CONSTRAINT auth_tokens_token_hash_unique UNIQUE (token_hash)
);

CREATE INDEX auth_tokens_user_idx ON auth_tokens (user_id);
CREATE INDEX auth_tokens_tenant_idx ON auth_tokens (tenant_id);

CREATE TABLE teams (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT teams_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE TABLE team_members (
    team_id     UUID NOT NULL REFERENCES teams (id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (team_id, user_id)
);

CREATE TABLE locations (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    map_path    TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT locations_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE TABLE buildings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    location_id     UUID NOT NULL REFERENCES locations (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    code            TEXT NOT NULL,
    address         TEXT,
    latitude        DOUBLE PRECISION,
    longitude       DOUBLE PRECISION,
    polygon         JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT buildings_code_unique UNIQUE (location_id, code)
);

CREATE TABLE floors (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    building_id     UUID NOT NULL REFERENCES buildings (id) ON DELETE CASCADE,
    name            TEXT NOT NULL,
    level           INT NOT NULL DEFAULT 0,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT floors_building_level_unique UNIQUE (building_id, level)
);

CREATE TABLE floor_plans (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    floor_id        UUID NOT NULL REFERENCES floors (id) ON DELETE CASCADE,
    storage_path    TEXT NOT NULL,
    sort_order      INT NOT NULL DEFAULT 0,
    width_px        INT,
    height_px       INT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE rooms (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    floor_id        UUID NOT NULL REFERENCES floors (id) ON DELETE CASCADE,
    number          TEXT NOT NULL,
    name            TEXT NOT NULL DEFAULT '',
    polygon         JSONB,
    is_bookable     BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT rooms_floor_number_unique UNIQUE (floor_id, number)
);

CREATE TABLE place_types (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT place_types_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE TABLE equipments (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id   UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    name        TEXT NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT equipments_tenant_name_unique UNIQUE (tenant_id, name)
);

CREATE TABLE places (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    room_id         UUID NOT NULL REFERENCES rooms (id) ON DELETE CASCADE,
    number          TEXT NOT NULL,
    place_type_id   UUID NOT NULL REFERENCES place_types (id),
    booking_mode    booking_mode NOT NULL DEFAULT 'daily',
    rectangle       JSONB,
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT places_room_number_unique UNIQUE (room_id, number)
);

CREATE TABLE place_equipments (
    place_id        UUID NOT NULL REFERENCES places (id) ON DELETE CASCADE,
    equipment_id    UUID NOT NULL REFERENCES equipments (id) ON DELETE CASCADE,
    PRIMARY KEY (place_id, equipment_id)
);

CREATE TABLE resources (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    location_id     UUID NOT NULL REFERENCES locations (id) ON DELETE CASCADE,
    building_id     UUID REFERENCES buildings (id) ON DELETE SET NULL,
    room_id         UUID REFERENCES rooms (id) ON DELETE SET NULL,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    is_active       BOOLEAN NOT NULL DEFAULT TRUE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT resources_tenant_location_name_unique UNIQUE (tenant_id, location_id, name)
);

CREATE TABLE bookings (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    user_id         UUID NOT NULL REFERENCES users (id),
    team_id         UUID REFERENCES teams (id) ON DELETE SET NULL,
    place_id        UUID REFERENCES places (id) ON DELETE CASCADE,
    resource_id     UUID REFERENCES resources (id) ON DELETE CASCADE,
    group_id        UUID,
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    status          booking_status NOT NULL DEFAULT 'booked',
    checked_in_at   TIMESTAMPTZ,
    checked_out_at  TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT bookings_time_valid CHECK (ends_at > starts_at),
    CONSTRAINT bookings_target CHECK (
        place_id IS NOT NULL OR resource_id IS NOT NULL
    ),
    CONSTRAINT bookings_single_target CHECK (
        NOT (place_id IS NOT NULL AND resource_id IS NOT NULL)
    )
);

CREATE INDEX bookings_tenant_time_idx ON bookings (tenant_id, starts_at, ends_at);
CREATE INDEX bookings_user_idx ON bookings (user_id);
CREATE INDEX bookings_group_idx ON bookings (group_id) WHERE group_id IS NOT NULL;

-- Active bookings must not overlap on the same place.
ALTER TABLE bookings ADD CONSTRAINT bookings_place_no_overlap
    EXCLUDE USING gist (
        place_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
    WHERE (
        place_id IS NOT NULL
        AND status IN ('booked', 'checked_in')
    );

ALTER TABLE bookings ADD CONSTRAINT bookings_resource_no_overlap
    EXCLUDE USING gist (
        resource_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
    WHERE (
        resource_id IS NOT NULL
        AND status IN ('booked', 'checked_in')
    );

CREATE TABLE blocks (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id       UUID NOT NULL REFERENCES tenants (id) ON DELETE CASCADE,
    created_by      UUID NOT NULL REFERENCES users (id),
    place_id        UUID REFERENCES places (id) ON DELETE CASCADE,
    room_id         UUID REFERENCES rooms (id) ON DELETE CASCADE,
    starts_at       TIMESTAMPTZ NOT NULL,
    ends_at         TIMESTAMPTZ NOT NULL,
    reason          TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT blocks_time_valid CHECK (ends_at > starts_at),
    CONSTRAINT blocks_target CHECK (
        (place_id IS NOT NULL AND room_id IS NULL)
        OR (place_id IS NULL AND room_id IS NOT NULL)
    )
);

CREATE INDEX blocks_tenant_time_idx ON blocks (tenant_id, starts_at, ends_at);

ALTER TABLE blocks ADD CONSTRAINT blocks_place_no_overlap
    EXCLUDE USING gist (
        place_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
    WHERE (place_id IS NOT NULL);

ALTER TABLE blocks ADD CONSTRAINT blocks_room_no_overlap
    EXCLUDE USING gist (
        room_id WITH =,
        tstzrange(starts_at, ends_at, '[)') WITH &&
    )
    WHERE (room_id IS NOT NULL);

-- Privacy-friendly occupancy: day was occupied, not by whom.
CREATE TABLE place_occupancy_days (
    place_id    UUID NOT NULL REFERENCES places (id) ON DELETE CASCADE,
    day         DATE NOT NULL,
    occupied    BOOLEAN NOT NULL DEFAULT TRUE,
    PRIMARY KEY (place_id, day)
);

CREATE INDEX place_occupancy_days_day_idx ON place_occupancy_days (day);

-- +goose Down
DROP TABLE IF EXISTS place_occupancy_days;
DROP TABLE IF EXISTS blocks;
DROP TABLE IF EXISTS bookings;
DROP TABLE IF EXISTS resources;
DROP TABLE IF EXISTS place_equipments;
DROP TABLE IF EXISTS places;
DROP TABLE IF EXISTS equipments;
DROP TABLE IF EXISTS place_types;
DROP TABLE IF EXISTS rooms;
DROP TABLE IF EXISTS floor_plans;
DROP TABLE IF EXISTS floors;
DROP TABLE IF EXISTS buildings;
DROP TABLE IF EXISTS locations;
DROP TABLE IF EXISTS team_members;
DROP TABLE IF EXISTS teams;
DROP TABLE IF EXISTS auth_tokens;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS tenants;

DROP TYPE IF EXISTS booking_status;
DROP TYPE IF EXISTS booking_mode;
DROP TYPE IF EXISTS identity_provider;
DROP TYPE IF EXISTS user_role;
DROP TYPE IF EXISTS tenant_status;
