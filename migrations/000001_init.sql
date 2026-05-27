CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TYPE user_role AS ENUM ('admin', 'volunteer');
CREATE TYPE user_status AS ENUM ('active', 'disabled');
CREATE TYPE audit_action AS ENUM ('create', 'update', 'delete');

CREATE TABLE users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  name text NOT NULL,
  role user_role NOT NULL DEFAULT 'volunteer',
  status user_status NOT NULL DEFAULT 'active',
  google_sub text UNIQUE,
  password_hash text,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  csrf_token_hash bytea NOT NULL UNIQUE,
  user_agent text NOT NULL DEFAULT '',
  ip inet,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  revoked_at timestamptz
);

CREATE TABLE oauth_states (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  state_hash bytea NOT NULL UNIQUE,
  redirect_path text NOT NULL DEFAULT '/',
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  consumed_at timestamptz
);

CREATE TABLE locations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE species (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  chinese_name text NOT NULL,
  english_name text NOT NULL,
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE observations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  observed_on date NOT NULL,
  location_id uuid NOT NULL REFERENCES locations(id) ON DELETE RESTRICT,
  species_id uuid NOT NULL REFERENCES species(id) ON DELETE RESTRICT,
  observer_id uuid NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
  count integer NOT NULL CHECK (count >= 0),
  notes text NOT NULL DEFAULT '',
  created_by uuid REFERENCES users(id) ON DELETE SET NULL,
  updated_by uuid REFERENCES users(id) ON DELETE SET NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE audit_logs (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  action audit_action NOT NULL,
  target_table text NOT NULL,
  target_id uuid NOT NULL,
  actor_user_id uuid REFERENCES users(id) ON DELETE SET NULL,
  actor_email text NOT NULL DEFAULT '',
  before_data jsonb,
  after_data jsonb,
  method text NOT NULL DEFAULT '',
  path text NOT NULL DEFAULT '',
  ip inet,
  user_agent text NOT NULL DEFAULT '',
  logged_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE login_attempts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL DEFAULT '',
  ip inet,
  success boolean NOT NULL,
  attempted_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);
CREATE INDEX observations_observer_id_idx ON observations(observer_id);
CREATE INDEX observations_observed_on_idx ON observations(observed_on);
CREATE INDEX audit_logs_target_idx ON audit_logs(target_table, target_id);
CREATE INDEX audit_logs_logged_at_idx ON audit_logs(logged_at DESC);
CREATE INDEX login_attempts_email_time_idx ON login_attempts(email, attempted_at DESC);
