-- Copyright 2021 Changkun Ou. All rights reserved.
-- Use of this source code is governed by a MIT
-- license that can be found in the LICENSE file.

-- links holds one row per short alias. The identity of a link is the pair
-- (host, alias): one process serves several hosts, and the same alias may
-- mean different things on each.
CREATE TABLE links (
    id         BIGSERIAL PRIMARY KEY,
    host       TEXT NOT NULL,
    alias      TEXT NOT NULL,
    url        TEXT NOT NULL,
    private    BOOLEAN NOT NULL DEFAULT FALSE,
    trust      BOOLEAN NOT NULL DEFAULT FALSE,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by TEXT NOT NULL DEFAULT '',
    updated_by TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (host, alias)
);

-- visits holds one row per redirect served.
--
-- There is deliberately no foreign key from alias to links. A visit is
-- recorded before the alias is resolved, so 404s are counted; the index
-- page records a visit with an empty alias; and deleting a link keeps its
-- history. On the production data that is 21% of the rows, which a foreign
-- key would reject. Queries that must match a link therefore join links
-- explicitly rather than relying on the schema to guarantee it.
CREATE TABLE visits (
    id           BIGSERIAL PRIMARY KEY,
    host         TEXT NOT NULL,
    alias        TEXT NOT NULL,
    -- Null when the value recorded before this column was validated was
    -- not a UUID. Historical rows are not given invented identifiers.
    visitor_id   UUID,
    -- Text, not inet: with gdpr.hide_ip the address is replaced by a
    -- SHA-1 hex digest, or by the literal "unknown" when hashing fails.
    ip           TEXT NOT NULL,
    ua           TEXT NOT NULL DEFAULT '',
    referer      TEXT NOT NULL DEFAULT '',
    -- Derived once at write time from ua and referer.
    referer_host TEXT NOT NULL DEFAULT '',
    browser      TEXT NOT NULL DEFAULT '',
    os           TEXT NOT NULL DEFAULT '',
    device       TEXT NOT NULL DEFAULT '',
    is_bot       BOOLEAN NOT NULL DEFAULT FALSE,
    time         TIMESTAMPTZ NOT NULL
);

-- Time-ranged stats for one alias.
CREATE INDEX idx_visits_host_alias_time ON visits (host, alias, time DESC);
-- PV/UV for one alias: uv is a distinct count over ip.
CREATE INDEX idx_visits_host_alias_ip ON visits (host, alias, ip);
-- The index listing orders by updated_at within a host.
CREATE INDEX idx_links_host_updated ON links (host, updated_at DESC);
