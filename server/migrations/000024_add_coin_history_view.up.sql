CREATE OR REPLACE VIEW coin_history AS
    -- Topic view earnings
    SELECT cl.id::text        AS id,
           cl.user_id,
           cl.coins_earned    AS amount,
           'earn'             AS type,
           COALESCE(ci.topic, '토픽 열람') AS description,
           cl.context_item_id::text        AS link_id,
           cl.created_at
    FROM coin_logs cl
    LEFT JOIN context_items ci ON ci.id = cl.context_item_id

    UNION ALL

    -- General coin events (signup bonus, admin adjustments, etc.)
    SELECT ce.id::text              AS id,
           ce.user_id,
           ce.amount,
           ce.type,
           COALESCE(ce.memo, ce.type) AS description,
           ''                         AS link_id,
           ce.created_at
    FROM coin_events ce

    UNION ALL

    -- Withdrawal deductions (initial pending → negative amount)
    SELECT w.id::text      AS id,
           w.user_id,
           -w.amount       AS amount,
           'withdrawal'    AS type,
           '출금 신청'      AS description,
           ''              AS link_id,
           w.created_at
    FROM withdrawals w
    INNER JOIN withdrawal_transitions wt ON wt.withdrawal_id = w.id AND wt.status = 'pending'

    UNION ALL

    -- Withdrawal refunds (rejected / cancelled → coins restored)
    SELECT wt.id::text AS id,
           w.user_id,
           w.amount    AS amount,
           'refund'    AS type,
           CASE wt.status
               WHEN 'rejected'  THEN '출금 거절 (환불)'
               WHEN 'cancelled' THEN '출금 취소 (환불)'
           END         AS description,
           ''          AS link_id,
           wt.created_at
    FROM withdrawal_transitions wt
    INNER JOIN withdrawals w ON w.id = wt.withdrawal_id
    WHERE wt.status IN ('rejected', 'cancelled');
