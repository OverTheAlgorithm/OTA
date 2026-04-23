package storage

import (
	"context"
	"errors"
	"fmt"

	"ota/crypto"
	"ota/domain/apperr"
	"ota/domain/withdrawal"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type WithdrawalRepository struct {
	pool          *pgxpool.Pool
	encryptionKey string // hex-encoded 32-byte AES-256 key; empty = no encryption
}

func NewWithdrawalRepository(pool *pgxpool.Pool) *WithdrawalRepository {
	return &WithdrawalRepository{pool: pool}
}

// WithEncryptionKey sets the AES-256-GCM encryption key for bank account numbers.
// key must be a 64-character hex string (32 bytes). Empty string disables encryption.
func (r *WithdrawalRepository) WithEncryptionKey(key string) *WithdrawalRepository {
	r.encryptionKey = key
	return r
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
	decrypted, err := crypto.Decrypt(r.encryptionKey, a.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("decrypt account number: %w", err)
	}
	a.AccountNumber = decrypted
	return &a, nil
}

func (r *WithdrawalRepository) UpsertBankAccount(ctx context.Context, a withdrawal.BankAccount) error {
	encrypted, err := crypto.Encrypt(r.encryptionKey, a.AccountNumber)
	if err != nil {
		return fmt.Errorf("encrypt account number: %w", err)
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO user_bank_accounts (user_id, bank_name, account_number, account_holder, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id) DO UPDATE SET
			bank_name      = $2,
			account_number = $3,
			account_holder = $4,
			updated_at     = NOW()
	`, a.UserID, a.BankName, encrypted, a.AccountHolder)
	if err != nil {
		return fmt.Errorf("upsert bank account: %w", err)
	}
	return nil
}

// ── Withdrawal CRUD ─────────────────────────────────────────────────────────

// CreateWithdrawalWithDeduction atomically locks the user's balance row,
// verifies sufficient funds, deducts coins, and creates the withdrawal +
// initial pending transition — all in a single transaction.
func (r *WithdrawalRepository) CreateWithdrawalWithDeduction(ctx context.Context, w withdrawal.Withdrawal, actorID string, amount int) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock the user's balance row and read current points
	var currentPoints int
	err = tx.QueryRow(ctx,
		`SELECT points FROM user_points WHERE user_id = $1 FOR UPDATE`, w.UserID,
	).Scan(&currentPoints)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, fmt.Errorf("%w: have 0, need %d", apperr.ErrInsufficientBalance, amount)
	}
	if err != nil {
		return uuid.Nil, fmt.Errorf("lock balance: %w", err)
	}

	if currentPoints < amount {
		return uuid.Nil, fmt.Errorf("%w: have %d, need %d", apperr.ErrInsufficientBalance, currentPoints, amount)
	}

	// Deduct coins
	_, err = tx.Exec(ctx,
		`UPDATE user_points SET points = points - $2, updated_at = NOW() WHERE user_id = $1`,
		w.UserID, amount,
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("deduct coins: %w", err)
	}

	// Encrypt account number before storing
	encAcct, err := crypto.Encrypt(r.encryptionKey, w.AccountNumber)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt account number: %w", err)
	}

	// Create withdrawal record
	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO withdrawals (user_id, amount, bank_name, account_number, account_holder)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, w.UserID, w.Amount, w.BankName, encAcct, w.AccountHolder).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert withdrawal: %w", err)
	}

	// Create initial pending transition
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

func (r *WithdrawalRepository) CreateWithdrawal(ctx context.Context, w withdrawal.Withdrawal, actorID string) (uuid.UUID, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return uuid.Nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	encAcct2, err := crypto.Encrypt(r.encryptionKey, w.AccountNumber)
	if err != nil {
		return uuid.Nil, fmt.Errorf("encrypt account number: %w", err)
	}

	var id uuid.UUID
	err = tx.QueryRow(ctx, `
		INSERT INTO withdrawals (user_id, amount, bank_name, account_number, account_holder)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, w.UserID, w.Amount, w.BankName, encAcct2, w.AccountHolder).Scan(&id)
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
		       (SELECT t.status FROM withdrawal_transitions t WHERE t.withdrawal_id = w.id ORDER BY t.created_at DESC LIMIT 1),
		       u.adblock_detected_at, u.adblock_not_detected_at
		FROM withdrawals w
		LEFT JOIN users u ON u.id = w.user_id
		WHERE w.id = $1
	`, id).Scan(
		&d.ID, &d.UserID, &d.Amount, &d.BankName, &d.AccountNumber, &d.AccountHolder, &d.CreatedAt,
		&d.CurrentStatus, &d.AdblockDetectedAt, &d.AdblockNotDetectedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get withdrawal by id: %w", err)
	}
	decrypted, err := crypto.Decrypt(r.encryptionKey, d.AccountNumber)
	if err != nil {
		return nil, fmt.Errorf("decrypt withdrawal account number: %w", err)
	}
	d.AccountNumber = decrypted

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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate transitions: %w", err)
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
		dec, err := crypto.Decrypt(r.encryptionKey, d.AccountNumber)
		if err != nil {
			return nil, false, fmt.Errorf("decrypt withdrawal account number: %w", err)
		}
		d.AccountNumber = dec
		items = append(items, d)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate withdrawals: %w", err)
	}

	hasMore := len(items) > limit
	if hasMore {
		items = items[:limit]
	}

	// Batch-load all transitions for the fetched withdrawals in a single query.
	if len(items) > 0 {
		ids := make([]uuid.UUID, len(items))
		indexByID := make(map[uuid.UUID]int, len(items))
		for i, item := range items {
			ids[i] = item.ID
			indexByID[item.ID] = i
		}

		tRows, err := r.pool.Query(ctx, `
			SELECT t.id, t.withdrawal_id, t.status, t.note, COALESCE(t.actor_id::text, ''),
			       COALESCE(u.nickname, ''), t.created_at, t.updated_at
			FROM withdrawal_transitions t
			LEFT JOIN users u ON u.id = t.actor_id
			WHERE t.withdrawal_id = ANY($1)
			ORDER BY t.withdrawal_id, t.created_at ASC
		`, ids)
		if err != nil {
			return nil, false, fmt.Errorf("get transitions batch: %w", err)
		}
		defer tRows.Close()

		for tRows.Next() {
			var t withdrawal.Transition
			if err := tRows.Scan(&t.ID, &t.WithdrawalID, &t.Status, &t.Note, &t.ActorID, &t.ActorName, &t.CreatedAt, &t.UpdatedAt); err != nil {
				return nil, false, fmt.Errorf("scan transition: %w", err)
			}
			if idx, ok := indexByID[t.WithdrawalID]; ok {
				items[idx].Transitions = append(items[idx].Transitions, t)
			}
		}
		if err := tRows.Err(); err != nil {
			return nil, false, fmt.Errorf("iterate transitions: %w", err)
		}
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
		       COALESCE(u.nickname, ''), COALESCE(u.email, ''),
		       u.adblock_detected_at, u.adblock_not_detected_at
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
			&item.AdblockDetectedAt, &item.AdblockNotDetectedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan withdrawal list item: %w", err)
		}
		decItem, err := crypto.Decrypt(r.encryptionKey, item.AccountNumber)
		if err != nil {
			return nil, 0, fmt.Errorf("decrypt withdrawal account number: %w", err)
		}
		item.AccountNumber = decItem
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate withdrawal list: %w", err)
	}

	return items, total, nil
}

// ── Atomic Cancel / Reject ───────────────────────────────────────────────────

// CancelWithdrawalAtomic locks the withdrawal and user_points rows, verifies
// the withdrawal is still pending, inserts a cancelled transition, and restores
// coins — all in a single transaction. Returns (amount, userID, error).
func (r *WithdrawalRepository) CancelWithdrawalAtomic(ctx context.Context, withdrawalID uuid.UUID, actorID string) (int, string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock withdrawal row and read user_id + amount
	var userID string
	var amount int
	err = tx.QueryRow(ctx,
		`SELECT user_id, amount FROM withdrawals WHERE id = $1 FOR UPDATE`,
		withdrawalID,
	).Scan(&userID, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("withdrawal not found")
	}
	if err != nil {
		return 0, "", fmt.Errorf("lock withdrawal: %w", err)
	}

	// Verify current status is pending
	var currentStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM withdrawal_transitions WHERE withdrawal_id = $1 ORDER BY created_at DESC LIMIT 1`,
		withdrawalID,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("no transitions found")
	}
	if err != nil {
		return 0, "", fmt.Errorf("check status: %w", err)
	}
	if currentStatus != withdrawal.StatusPending {
		return 0, "", apperr.NewConflictError(fmt.Sprintf("can only cancel pending withdrawals (current: %s)", currentStatus))
	}

	// Lock user_points row
	_, err = tx.Exec(ctx,
		`SELECT points FROM user_points WHERE user_id = $1 FOR UPDATE`,
		userID,
	)
	if err != nil {
		return 0, "", fmt.Errorf("lock user_points: %w", err)
	}

	// Insert cancelled transition
	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawal_transitions (withdrawal_id, status, note, actor_id) VALUES ($1, $2, '', $3)`,
		withdrawalID, withdrawal.StatusCancelled, actorID,
	)
	if err != nil {
		return 0, "", fmt.Errorf("insert cancelled transition: %w", err)
	}

	// Restore coins
	_, err = tx.Exec(ctx,
		`UPDATE user_points SET points = points + $2, updated_at = NOW() WHERE user_id = $1`,
		userID, amount,
	)
	if err != nil {
		return 0, "", fmt.Errorf("restore coins: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, "", fmt.Errorf("commit: %w", err)
	}
	return amount, userID, nil
}

// RejectWithdrawalAtomic locks the withdrawal and user_points rows, verifies
// the withdrawal is still pending, inserts a rejected transition with a note,
// and restores coins — all in a single transaction. Returns (amount, userID, error).
func (r *WithdrawalRepository) RejectWithdrawalAtomic(ctx context.Context, withdrawalID uuid.UUID, actorID, note string) (int, string, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return 0, "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock withdrawal row and read user_id + amount
	var userID string
	var amount int
	err = tx.QueryRow(ctx,
		`SELECT user_id, amount FROM withdrawals WHERE id = $1 FOR UPDATE`,
		withdrawalID,
	).Scan(&userID, &amount)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("withdrawal not found")
	}
	if err != nil {
		return 0, "", fmt.Errorf("lock withdrawal: %w", err)
	}

	// Verify current status is pending
	var currentStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM withdrawal_transitions WHERE withdrawal_id = $1 ORDER BY created_at DESC LIMIT 1`,
		withdrawalID,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, "", fmt.Errorf("no transitions found")
	}
	if err != nil {
		return 0, "", fmt.Errorf("check status: %w", err)
	}
	if currentStatus != withdrawal.StatusPending {
		return 0, "", apperr.NewConflictError(fmt.Sprintf("can only reject pending withdrawals (current: %s)", currentStatus))
	}

	// Lock user_points row
	_, err = tx.Exec(ctx,
		`SELECT points FROM user_points WHERE user_id = $1 FOR UPDATE`,
		userID,
	)
	if err != nil {
		return 0, "", fmt.Errorf("lock user_points: %w", err)
	}

	// Insert rejected transition
	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawal_transitions (withdrawal_id, status, note, actor_id) VALUES ($1, $2, $3, $4)`,
		withdrawalID, withdrawal.StatusRejected, note, actorID,
	)
	if err != nil {
		return 0, "", fmt.Errorf("insert rejected transition: %w", err)
	}

	// Restore coins
	_, err = tx.Exec(ctx,
		`UPDATE user_points SET points = points + $2, updated_at = NOW() WHERE user_id = $1`,
		userID, amount,
	)
	if err != nil {
		return 0, "", fmt.Errorf("restore coins: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, "", fmt.Errorf("commit: %w", err)
	}
	return amount, userID, nil
}

// ApproveWithdrawalAtomic locks the withdrawal row, verifies the withdrawal is
// still pending, and inserts an approved transition with a note — all in a
// single transaction.
func (r *WithdrawalRepository) ApproveWithdrawalAtomic(ctx context.Context, withdrawalID uuid.UUID, actorID, note string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Lock withdrawal row
	var userID string
	err = tx.QueryRow(ctx,
		`SELECT user_id FROM withdrawals WHERE id = $1 FOR UPDATE`,
		withdrawalID,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("withdrawal not found")
	}
	if err != nil {
		return fmt.Errorf("lock withdrawal: %w", err)
	}

	// Verify current status is pending
	var currentStatus string
	err = tx.QueryRow(ctx,
		`SELECT status FROM withdrawal_transitions WHERE withdrawal_id = $1 ORDER BY created_at DESC LIMIT 1`,
		withdrawalID,
	).Scan(&currentStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("no transitions found")
	}
	if err != nil {
		return fmt.Errorf("check status: %w", err)
	}
	if currentStatus != withdrawal.StatusPending {
		return apperr.NewConflictError(fmt.Sprintf("can only approve pending withdrawals (current: %s)", currentStatus))
	}

	// Insert approved transition
	_, err = tx.Exec(ctx,
		`INSERT INTO withdrawal_transitions (withdrawal_id, status, note, actor_id) VALUES ($1, $2, $3, $4)`,
		withdrawalID, withdrawal.StatusApproved, note, actorID,
	)
	if err != nil {
		return fmt.Errorf("insert approved transition: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
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
