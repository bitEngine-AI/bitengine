-- BitEngine MVP — 6 张表，够用就行
CREATE SCHEMA IF NOT EXISTS platform;
CREATE SCHEMA IF NOT EXISTS runtime;

CREATE TABLE platform.users (
    id TEXT PRIMARY KEY, username TEXT UNIQUE NOT NULL,
    password TEXT NOT NULL, role TEXT DEFAULT 'owner',
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE platform.config (
    key TEXT PRIMARY KEY, value JSONB NOT NULL, updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE platform.setup_state (
    id INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    completed BOOLEAN DEFAULT false, step INTEGER DEFAULT 0,
    data JSONB DEFAULT '{}', updated_at TIMESTAMPTZ DEFAULT NOW()
);
INSERT INTO platform.setup_state (completed, step) VALUES (false, 0) ON CONFLICT DO NOTHING;

CREATE TABLE runtime.apps (
    id TEXT PRIMARY KEY, name TEXT NOT NULL, slug TEXT UNIQUE NOT NULL,
    status TEXT DEFAULT 'creating', container_id TEXT, image_tag TEXT,
    domain TEXT, port INTEGER, prompt TEXT, source_code TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(), updated_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE runtime.templates (
    id TEXT PRIMARY KEY, slug TEXT UNIQUE NOT NULL, name TEXT NOT NULL,
    description TEXT, category TEXT, source_path TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW()
);
CREATE TABLE runtime.generation_logs (
    id TEXT PRIMARY KEY, app_id TEXT REFERENCES runtime.apps(id),
    step TEXT NOT NULL, status TEXT DEFAULT 'running',
    detail TEXT, created_at TIMESTAMPTZ DEFAULT NOW()
);
