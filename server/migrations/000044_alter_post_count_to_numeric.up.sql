-- ct_tag_daily.post_count 컬럼의 타입을 INT에서 NUMERIC(10,2)로 변경하여 가중치 소수점 값을 표현할 수 있도록 승격
ALTER TABLE ct_tag_daily ALTER COLUMN post_count TYPE NUMERIC(10, 2);
