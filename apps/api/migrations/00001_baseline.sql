-- +goose Up
-- baseline: the production schema as GORM AutoMigrate and schemas.Migrate built it, captured 2026-11-13 with pg_dump --schema-only --no-owner --no-privileges --no-comments --no-tablespaces --schema=public

CREATE TABLE call_participants (
    id bigint NOT NULL,
    call_id uuid NOT NULL,
    identity text NOT NULL,
    sid text DEFAULT ''::text NOT NULL,
    name text DEFAULT ''::text NOT NULL,
    joined_at timestamp with time zone NOT NULL,
    left_at timestamp with time zone,
    user_id bigint
);

CREATE SEQUENCE call_participants_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE call_participants_id_seq OWNED BY call_participants.id;

CREATE TABLE calls (
    id uuid NOT NULL,
    room_id uuid NOT NULL,
    started_at timestamp with time zone NOT NULL,
    ended_at timestamp with time zone,
    livekit_room_name text NOT NULL,
    livekit_room_sid text DEFAULT ''::text NOT NULL,
    egress_id text DEFAULT ''::text NOT NULL,
    recording_path text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    transcribe_on boolean DEFAULT false NOT NULL,
    record_on boolean DEFAULT false NOT NULL
);

CREATE TABLE capture_consents (
    id bigint NOT NULL,
    call_id uuid NOT NULL,
    identity text NOT NULL,
    transcribe boolean,
    record boolean,
    updated_at timestamp with time zone
);

CREATE SEQUENCE capture_consents_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE capture_consents_id_seq OWNED BY capture_consents.id;

CREATE TABLE directory_deletion_state (
    id integer NOT NULL,
    state bytea NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE forfait_state (
    id bigint NOT NULL,
    document text NOT NULL,
    updated_at timestamp with time zone
);

CREATE TABLE mcp_actions (
    id text NOT NULL,
    user_id text NOT NULL,
    kind text NOT NULL,
    target text NOT NULL,
    digest text NOT NULL,
    summary text DEFAULT ''::text NOT NULL,
    client_id text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    resolved_at timestamp with time zone,
    outcome text DEFAULT ''::text NOT NULL
);

CREATE TABLE porte_identities (
    user_id bigint NOT NULL,
    provider text NOT NULL,
    subject text NOT NULL,
    password_hash text DEFAULT ''::text NOT NULL,
    access_token text DEFAULT ''::text NOT NULL,
    refresh_token text DEFAULT ''::text NOT NULL,
    token_expiry timestamp with time zone,
    roles jsonb,
    roles_synced_at timestamp with time zone,
    synced_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now()
);

CREATE TABLE porte_login_codes (
    code_hash text NOT NULL,
    user_id bigint NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    consumed_at timestamp with time zone,
    provider text DEFAULT ''::text NOT NULL,
    subject text DEFAULT ''::text NOT NULL
);

CREATE TABLE porte_sessions (
    id bigint NOT NULL,
    token_hash text NOT NULL,
    user_id bigint NOT NULL,
    label text DEFAULT ''::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone
);

CREATE SEQUENCE porte_sessions_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE porte_sessions_id_seq OWNED BY porte_sessions.id;

CREATE TABLE recordings (
    id uuid NOT NULL,
    call_id uuid NOT NULL,
    egress_id text DEFAULT ''::text NOT NULL,
    filename text NOT NULL,
    shared boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone,
    content_type text DEFAULT 'video/mp4'::text NOT NULL,
    status text DEFAULT 'complete'::text NOT NULL,
    size bigint DEFAULT 0 NOT NULL,
    chunks bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone
);

CREATE TABLE rooms (
    id uuid NOT NULL,
    slug text NOT NULL,
    name text NOT NULL,
    owner_id bigint,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    integration_app text DEFAULT ''::text NOT NULL,
    external_id text DEFAULT ''::text NOT NULL,
    callback_url text DEFAULT ''::text NOT NULL
);

CREATE TABLE summaries (
    id uuid NOT NULL,
    call_id uuid NOT NULL,
    content text,
    model text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE transcript_lines (
    id bigint NOT NULL,
    call_id uuid NOT NULL,
    speaker text NOT NULL,
    text text NOT NULL,
    offset_ms bigint DEFAULT 0 NOT NULL
);

CREATE SEQUENCE transcript_lines_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE transcript_lines_id_seq OWNED BY transcript_lines.id;

CREATE TABLE transcripts (
    id uuid NOT NULL,
    call_id uuid NOT NULL,
    content text,
    language text DEFAULT 'fr'::text NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone
);

CREATE TABLE users (
    id bigint NOT NULL,
    email text NOT NULL,
    name text,
    password_hash text DEFAULT ''::text NOT NULL,
    is_admin boolean DEFAULT false,
    created_at timestamp with time zone,
    picture text DEFAULT ''::text NOT NULL,
    locale text DEFAULT 'en'::text NOT NULL
);

CREATE SEQUENCE users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE users_id_seq OWNED BY users.id;

ALTER TABLE ONLY call_participants ALTER COLUMN id SET DEFAULT nextval('call_participants_id_seq'::regclass);

ALTER TABLE ONLY capture_consents ALTER COLUMN id SET DEFAULT nextval('capture_consents_id_seq'::regclass);

ALTER TABLE ONLY porte_sessions ALTER COLUMN id SET DEFAULT nextval('porte_sessions_id_seq'::regclass);

ALTER TABLE ONLY transcript_lines ALTER COLUMN id SET DEFAULT nextval('transcript_lines_id_seq'::regclass);

ALTER TABLE ONLY users ALTER COLUMN id SET DEFAULT nextval('users_id_seq'::regclass);

ALTER TABLE ONLY call_participants
    ADD CONSTRAINT call_participants_pkey PRIMARY KEY (id);

ALTER TABLE ONLY calls
    ADD CONSTRAINT calls_pkey PRIMARY KEY (id);

ALTER TABLE ONLY capture_consents
    ADD CONSTRAINT capture_consents_pkey PRIMARY KEY (id);

ALTER TABLE ONLY directory_deletion_state
    ADD CONSTRAINT directory_deletion_state_pkey PRIMARY KEY (id);

ALTER TABLE ONLY forfait_state
    ADD CONSTRAINT forfait_state_pkey PRIMARY KEY (id);

ALTER TABLE ONLY mcp_actions
    ADD CONSTRAINT mcp_actions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY porte_identities
    ADD CONSTRAINT porte_identities_pkey PRIMARY KEY (provider, subject);

ALTER TABLE ONLY porte_login_codes
    ADD CONSTRAINT porte_login_codes_pkey PRIMARY KEY (code_hash);

ALTER TABLE ONLY porte_sessions
    ADD CONSTRAINT porte_sessions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY porte_sessions
    ADD CONSTRAINT porte_sessions_token_hash_key UNIQUE (token_hash);

ALTER TABLE ONLY recordings
    ADD CONSTRAINT recordings_pkey PRIMARY KEY (id);

ALTER TABLE ONLY rooms
    ADD CONSTRAINT rooms_pkey PRIMARY KEY (id);

ALTER TABLE ONLY summaries
    ADD CONSTRAINT summaries_pkey PRIMARY KEY (id);

ALTER TABLE ONLY transcript_lines
    ADD CONSTRAINT transcript_lines_pkey PRIMARY KEY (id);

ALTER TABLE ONLY transcripts
    ADD CONSTRAINT transcripts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

CREATE UNIQUE INDEX calls_livekit_room_sid_key ON calls USING btree (livekit_room_sid) WHERE (livekit_room_sid <> ''::text);

CREATE UNIQUE INDEX idx_call_participant_identity ON call_participants USING btree (call_id, identity);

CREATE INDEX idx_call_participants_user_id ON call_participants USING btree (user_id);

CREATE INDEX idx_calls_livekit_room_name ON calls USING btree (livekit_room_name);

CREATE INDEX idx_calls_room_id ON calls USING btree (room_id);

CREATE UNIQUE INDEX idx_capture_consent_identity ON capture_consents USING btree (call_id, identity);

CREATE INDEX idx_recordings_call_id ON recordings USING btree (call_id);

CREATE INDEX idx_rooms_owner_id ON rooms USING btree (owner_id);

CREATE UNIQUE INDEX idx_rooms_slug ON rooms USING btree (slug);

CREATE UNIQUE INDEX idx_summaries_call_id ON summaries USING btree (call_id);

CREATE INDEX idx_transcript_lines_call_id ON transcript_lines USING btree (call_id);

CREATE UNIQUE INDEX idx_transcripts_call_id ON transcripts USING btree (call_id);

CREATE UNIQUE INDEX idx_users_email ON users USING btree (email);

CREATE INDEX mcp_actions_open_idx ON mcp_actions USING btree (user_id, created_at DESC) WHERE (resolved_at IS NULL);

CREATE INDEX porte_identities_user_idx ON porte_identities USING btree (user_id);

CREATE INDEX porte_sessions_expiry_idx ON porte_sessions USING btree (expires_at) WHERE (expires_at IS NOT NULL);

CREATE INDEX porte_sessions_user_idx ON porte_sessions USING btree (user_id);

CREATE UNIQUE INDEX recordings_egress_key ON recordings USING btree (egress_id) WHERE (egress_id <> ''::text);

CREATE UNIQUE INDEX rooms_integration_external_key ON rooms USING btree (integration_app, external_id) WHERE (integration_app <> ''::text);

ALTER TABLE ONLY call_participants
    ADD CONSTRAINT fk_call_participants_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY calls
    ADD CONSTRAINT fk_calls_room FOREIGN KEY (room_id) REFERENCES rooms(id) ON DELETE CASCADE;

ALTER TABLE ONLY capture_consents
    ADD CONSTRAINT fk_capture_consents_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY recordings
    ADD CONSTRAINT fk_recordings_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY summaries
    ADD CONSTRAINT fk_summaries_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY transcript_lines
    ADD CONSTRAINT fk_transcript_lines_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY transcripts
    ADD CONSTRAINT fk_transcripts_call FOREIGN KEY (call_id) REFERENCES calls(id) ON DELETE CASCADE;

ALTER TABLE ONLY porte_identities
    ADD CONSTRAINT porte_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY porte_login_codes
    ADD CONSTRAINT porte_login_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

ALTER TABLE ONLY porte_sessions
    ADD CONSTRAINT porte_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE;

-- +goose Down
SELECT 'baseline migration is not reversible' WHERE false;
