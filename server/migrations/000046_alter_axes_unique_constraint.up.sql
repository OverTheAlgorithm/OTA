-- 기존 단일 key UNIQUE 제약조건 제거 (Postgres 기본 이름: ct_axes_key_key)
ALTER TABLE ct_axes DROP CONSTRAINT ct_axes_key_key;

-- (key, type) 복합 UNIQUE 제약조건 추가
ALTER TABLE ct_axes ADD CONSTRAINT ct_axes_key_type_unique UNIQUE (key, type);
