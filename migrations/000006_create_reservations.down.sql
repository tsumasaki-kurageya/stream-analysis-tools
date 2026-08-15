ALTER TABLE collection.collection_jobs
    DROP CONSTRAINT IF EXISTS collection_jobs_reservation_fkey;

DROP INDEX IF EXISTS collection.collection_jobs_reservation_uidx;

ALTER TABLE collection.collection_jobs
    DROP COLUMN IF EXISTS reservation_id;

DROP SCHEMA IF EXISTS reservation CASCADE;
