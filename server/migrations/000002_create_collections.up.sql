CREATE TABLE collection_runs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    status VARCHAR(20) NOT NULL DEFAULT 'running',
    error_message TEXT,
    raw_response TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE context_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    collection_run_id UUID NOT NULL REFERENCES collection_runs(id),
    category VARCHAR(50) NOT NULL,
    rank INT NOT NULL,
    topic TEXT NOT NULL,
    summary TEXT NOT NULL,
    sources JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_context_items_run_id ON context_items(collection_run_id);
CREATE INDEX idx_context_items_category ON context_items(category);
CREATE INDEX idx_collection_runs_started ON collection_runs(started_at DESC);
