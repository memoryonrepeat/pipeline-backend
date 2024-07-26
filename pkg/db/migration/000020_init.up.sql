BEGIN;

CREATE extension pg_trgm;
ALTER TABLE public.pipeline ADD COLUMN IF NOT EXISTS namespace_id VARCHAR(255) DEFAULT '';
ALTER TABLE public.pipeline ADD COLUMN IF NOT EXISTS namespace_type VARCHAR(255) DEFAULT '';
ALTER TABLE public.secret ADD COLUMN IF NOT EXISTS namespace_id VARCHAR(255) DEFAULT '';
ALTER TABLE public.secret ADD COLUMN IF NOT EXISTS namespace_type VARCHAR(255) DEFAULT '';

COMMIT;