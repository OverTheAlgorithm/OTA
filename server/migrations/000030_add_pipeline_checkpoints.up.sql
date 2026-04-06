ALTER TABLE collection_runs
    ADD COLUMN last_completed_stage SMALLINT,
    ADD COLUMN checkpoint_data JSONB;

COMMENT ON COLUMN collection_runs.last_completed_stage IS 'Last successfully completed pipeline stage (0-3). NULL means no checkpoint.';
COMMENT ON COLUMN collection_runs.checkpoint_data IS 'Serialized intermediate data from the last completed stage for resume capability.';
