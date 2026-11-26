-- migrate:up

-- Goéland POC — marker of a synthetic demo database (GLD-085).
--
-- `goeland-server demo-seed` writes this single row once it has generated the
-- fictional demo (GLD-079) into a brand-new database. The demo authentication
-- mode (GOELAND_AUTH_MODE=demo) lets anyone sign in as a demo user without a
-- password, so the server refuses to start in that mode unless this row
-- exists: a database holding real data (an import, a production copy) never
-- has it, since demo-seed only writes into a database without any subject.

CREATE TABLE demo_dataset (
    singleton         BOOLEAN PRIMARY KEY DEFAULT true,
    profile           TEXT NOT NULL,
    language          TEXT NOT NULL,
    generator_version TEXT NOT NULL,
    generated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT demo_dataset_single_row CHECK (singleton),
    CONSTRAINT demo_dataset_profile_not_blank CHECK (length(btrim(profile)) > 0),
    CONSTRAINT demo_dataset_language_not_blank CHECK (length(btrim(language)) > 0)
);

-- migrate:down

DROP TABLE IF EXISTS demo_dataset;
