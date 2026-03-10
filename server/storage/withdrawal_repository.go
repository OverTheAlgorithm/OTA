package storage

import (
	"context"
	"errors"
	"fmt"

	"ota/domain/withdrawal"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WithdrawalRepository struct {
	pool *pgxpool.Pool
}

func NewWithdrawalRepository(pool *pgxpool.Pool) *WithdrawalRepository {
	return &WithdrawalRepository{pool: pool}
}

// ── Bank Account ────────────────────────────────────────────────────────────

func (r *WithdrawalRepository) GetBankAccount(ctx context.Context, userID string) (*withdrawal.BankAccount, error) {
	var a withdrawal.BankAccount
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, bank_name, account_number, account_holder
		 FROM user_bank_accounts WHERE user_id = $1`, userID,
	).Scan(&a.UserID, &a.BankName, &a.AccountNumber, &a.AccountHolder)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get bank account: %w", err)
	}
	return &a, nil
}

func (r *WithdrawalRepository) UpsertBankAccount(ctx context.Context, a withdrawal.BankAccount) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO user_bank_accounts (user_id, bank_name, account_number, account_holder, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			bank_name      = $2,
			account_number = $3,
			account_holder = $4,
			updated_at     = NOW()
	`, a.UserID, a.BankName, a.AccountNumber, a.AccountHolder)
	if err != nil {
		return fmt.Errorf("upsert bank account: %w", err)
	}
	return nil
}

// ── Withdrawal CRUD ─────────────────────────────────────────────────────────

func (r *WithdrawalRepository) CreateWithdrawal(ctx context.Context, w withdrawal.Withdrawal, actorID string) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO withdrawals (user_id, amount, bank_name, account_number, account_holder)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, w.UserID, w.Amount, w.BankName, w.AccountNumber, w.AccountHolder).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert withdrawal: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO withdrawal_transitions (withdrawal_id, status, note, actor_id)
		VALUES ($1, $2, '', $3)
	`, id, withdrawal.StatusPending, actorID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert initial transition: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, fmt.Errorf("commit: %w", err)
	}
	return id, nil
}

func (r *WithdrawalRepository) GetByID(ctx context.Context, id uuid.UUID) (*withdrawal.WithdrawalDetail, error) {
	var d withdrawal.WithdrawalDetail
	err := r.pool.QueryRow(ctx, `
		SELECT w.id, w.user_id, w.amount, w.bank_name, w.account_number, w.account_holder, w.created_at,
		       (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1)
		FROM withdrawals w
		WHERE w.id = $1
	`, id).Scan(
		&d.ID, &d.UserID, &d.Amount, &d.BankName, &d.AccountNumber, &d.AccountHolder, &d.CreatedAt,
		&d.CurrentStatus,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get withdrawal by id: %w", err)
	}

	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.withdrawal_id, t.status, t.note, COALESCE(t.actor_id::text, ''),
		       COALESCE(u.nickname, ''), t.created_at, t.updated_at
		FROM withdrawal_transitions t
		LEFT JOIN users u ON u.id = t.actor_id
		WHERE t.withdrawal_id = $1
		ORDER BY t.created_at ASC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("get transitions: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var t withdrawal.Transition
		if err := rows.Scan(&t.ID, &t.WithdrawalID, &t.Status, &t.Note, &t.ActorID, &t.ActorName, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan transition: %w", err)
		}
		d.Transitions = append(d.Transitions, t)
	}

	return &d, nil
}

func (r *WithdrawalRepository) GetByUser(ctx context.Context, userID string, limit, offset int) ([]withdrawal.WithdrawalDetail, bool, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT w.id, w.user_id, w.amount, w.bank_name, w.account_number, w.account_holder, w.created_at,
		       (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1)
		FROM withdrawals w
		WHERE w.user_id = $1
		ORDER BY w.created_at DESC
		LIMIT $2 OFFSET $3
	`, userID, limit+1, offset)
	if err != nil {
		return nil, false, fmt.Errorf("get user withdrawals: %w", err)
	}
	defer rows.Close()

	var items []withdrawal.WithdrawalDetail
	for rows.Next() {
		var d withdrawal.WithdrawalDetail
		if err := rows.Scan(&d.ID, &d.UserID, &d.Amount, &d.BankName, &d.AccountNumber, &d.AccountHolder, &d.CreatedAt, &d.CurrentStatus); err != nil {
			return nil, false, fmt.Errorf("scan withdrawal: %w", err)
		}
		items = append(items, d)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	// Load transitions for each withdrawal
	for i := range items {
		tRows, err := r.pool.Query(ctx, `
			SELECT t.id, t.withdrawal_id, t.status, t.note, COALESCE(t.actor_id::text, ''),
			       COALESCE(u.nickname, ''), t.created_at, t.updated_at
			FROM withdrawal_transitions t
			LEFT JOIN users u ON u.id = t.actor_id
			WHERE t.withdrawal_id = $1
			ORDER BY t.created_at ASC
		`, items[i].ID)
		if err != nil {
			return nil, false, fmt.Errorf("get transitions for %s: %w", items[i].ID, err)
		}
		for tRows.Next() {
			var t withdrawal.Transition
			if err := tRows.Scan(&t.ID, &t.WithdrawalID, &t.Status, &t.Note, &t.ActorID, &t.ActorName, &t.CreatedAt, &t.UpdatedAt); err != nil {
				tRows.Close()
				return nil, false, fmt.Errorf("scan transition: %w", err)
			}
			items[i].Transitions = append(items[i].Transitions, t)
		}
		tRows.Close()
	}

	return items, hasMore, nil
}

// ── Admin Listing ───────────────────────────────────────────────────────────

func (r *WithdrawalRepository) ListAll(ctx context.Context, filter withdrawal.ListFilter) ([]withdrawal.WithdrawalListItem, int, error) {
	// Count query
	countQuery := `
		SELECT COUNT(*)
		FROM withdrawals w
		WHERE 1=1
	`
	args := []interface{}{}
	argIdx := 1

	if filter.Status != "" {
		countQuery += fmt.Sprintf(` AND (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1) = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}

	var total int
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count withdrawals: %w", err)
	}

	// Data query
	dataQuery := `
		SELECT w.id, w.user_id, w.amount, w.bank_name, w.account_number, w.account_holder, w.created_at,
		       (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1),
		       COALESCE(u.nickname, ''), COALESCE(u.email, '')
		FROM withdrawals w
		LEFT JOIN users u ON u.id = w.user_id
		WHERE 1=1
	`
	dataArgs := []interface{}{}
	dataArgIdx := 1

	if filter.Status != "" {
		dataQuery += fmt.Sprintf(` AND (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1) = $%d`, dataArgIdx)
		dataArgs = append(dataArgs, filter.Status)
		dataArgIdx++
	}

	dataQuery += ` ORDER BY w.created_at DESC`
	dataQuery += fmt.Sprintf(` LIMIT $%d OFFSET $%d`, dataArgIdx, dataArgIdx+1)
	dataArgs = append(dataArgs, filter.Limit, filter.Offset)

	rows, err := r.pool.Query(ctx, dataQuery, dataArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list withdrawals: %w", err)
	}
	defer rows.Close()

	var items []withdrawal.WithdrawalListItem
	for rows.Next() {
		var item withdrawal.WithdrawalListItem
		if err := rows.Scan(
			&item.ID, &item.UserID, &item.Amount, &item.BankName, &item.AccountNumber, &item.AccountHolder, &item.CreatedAt,
			&item.CurrentStatus, &item.UserNickname, &item.UserEmail,
		); err != nil {
			return nil, 0, fmt.Errorf("scan withdrawal list item: %w", err)
		}
		items = append(items, item)
	}

	return items, total, nil
}

// ── Transitions ─────────────────────────────────────────────────────────────

func (r *WithdrawalRepository) AddTransition(ctx context.Context, withdrawalID uuid.UUID, status, note, actorID string) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO withdrawal_transitions (withdrawal_id, status, note, actor_id)
		VALUES ($1, $2, $3, $4)
	`, withdrawalID, status, note, actorID)
	if err != nil {
		return fmt.Errorf("add transition: %w", err)
	}
	return nil
}

func (r *WithdrawalRepository) GetLatestStatus(ctx context.Context, withdrawalID uuid.UUID) (string, error) {
	var status string
	err := r.pool.QueryRow(ctx, `
		SELECT status FROM withdrawal_transitions
		WHERE withdrawal_id = $1
		ORDER BY created_at DESC LIMIT 1
	`, withdrawalID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("no transitions found")
	}
	if err != nil {
		return "", fmt.Errorf("get latest status: %w", err)
	}
	return status, nil
}

func (r *WithdrawalRepository) GetTransitionByID(ctx context.Context, transitionID uuid.UUID) (*withdrawal.Transition, error) {
	var t withdrawal.Transition
	err := r.pool.QueryRow(ctx, `
		SELECT id, withdrawal_id, status, note, COALESCE(actor_id::text, ''), created_at, updated_at
		FROM withdrawal_transitions WHERE id = $1
	`, transitionID).Scan(&t.ID, &t.WithdrawalID, &t.Status, &t.Note, &t.ActorID, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get transition: %w", err)
	}
	return &t, nil
}

func (r *WithdrawalRepository) UpdateTransitionNote(ctx context.Context, transitionID uuid.UUID, note string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE withdrawal_transitions SET note = $2, updated_at = NOW()
		WHERE id = $1
	`, transitionID, note)
	if err != nil {
		return fmt.Errorf("update transition note: %w", err)
	}
	return nil
}

// ── Ownership ───────────────────────────────────────────────────────────────

func (r *WithdrawalRepository) GetWithdrawalOwner(ctx context.Context, withdrawalID uuid.UUID) (string, error) {
	var userID string
	err := r.pool.QueryRow(ctx,
		`SELECT user_id FROM withdrawals WHERE id = $1`, withdrawalID,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", fmt.Errorf("withdrawal not found")
	}
	if err != nil {
		return "", fmt.Errorf("get withdrawal owner: %w", err)
	}
	return userID, nil
}

func (r *WithdrawalRepository) HasPendingWithdrawals(ctx context.Context, userID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM withdrawals w
			WHERE w.user_id = $1
			AND (
				SELECT wt.status FROM withdrawal_transitions wt
				WHERE wt.withdrawal_id = w.id
				ORDER BY wt.created_at DESC LIMIT 1
			) = 'pending'
		)`, userID,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check pending withdrawals: %w", err)
	}
	return exists, nil
}
