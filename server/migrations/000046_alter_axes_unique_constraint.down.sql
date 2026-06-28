ALTER TABLE ct_axes DROP CONSTRAINT ct_axes_key_type_unique;
ALTER TABLE ct_axes ADD CONSTRAINT ct_axes_key_key UNIQUE (key);
