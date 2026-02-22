-- Add details (JSON array) to replace single detail text field
ALTER TABLE context_items ADD COLUMN details JSONB DEFAULT '[]';

-- Migrate existing detail values into single-element arrays
UPDATE context_items SET details = jsonb_build_array(detail) WHERE detail IS NOT NULL AND detail != '';

-- Add buzz_score for topic popularity ranking
ALTER TABLE context_items ADD COLUMN buzz_score INT DEFAULT 0;
