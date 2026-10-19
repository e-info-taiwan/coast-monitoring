ALTER TABLE public_content DROP CONSTRAINT public_content_kind_check;
ALTER TABLE public_content ADD CONSTRAINT public_content_kind_check CHECK (kind IN ('article','activity','development','profile','annotation','chart','settings'));
CREATE UNIQUE INDEX public_content_single_settings ON public_content(kind) WHERE kind='settings';
CREATE TABLE reef_import_attempt (
 id bigserial PRIMARY KEY,source_sha256 text NOT NULL,source_snapshot jsonb NOT NULL,source_format text NOT NULL,
 status text NOT NULL CHECK (status IN ('validating','validated','completed','failed')),
 report jsonb NOT NULL DEFAULT '{}',created_at timestamptz NOT NULL DEFAULT now(),updated_at timestamptz NOT NULL DEFAULT now()
);
