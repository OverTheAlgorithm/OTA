-- Add type column to ct_axes
ALTER TABLE ct_axes ADD COLUMN type TEXT NOT NULL DEFAULT 'topic' CHECK (type IN ('meta', 'topic'));

-- Set type of existing meta axes
UPDATE ct_axes SET type = 'meta' WHERE key IN ('leaning', 'political', 'age');
UPDATE ct_axes SET type = 'topic' WHERE key IN ('gender_topic', 'political_topic');
