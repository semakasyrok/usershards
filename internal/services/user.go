package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"
	"time"
	"usershards/internal/apperrors"
	"usershards/internal/id"
	"usershards/internal/models"
	"usershards/internal/shard"
)

const WelcomeBonus = 1000_00

func NewUserService(manager *shard.ShardManager) *UserService {
	return &UserService{
		ShardManager: manager,
	}
}

type UserService struct {
	ShardManager *shard.ShardManager
}

func (s *UserService) GetUserByID(ctx context.Context, userID int64) (*models.User, error) {
	_, shardID, _ := id.ParseUserID(userID)

	usersDB, ok := s.ShardManager.UserShards[shardID]
	if !ok {
		return nil, fmt.Errorf("user shard %d not found for id %d", shardID, userID)
	}

	const query = `SELECT id, phone_number, email, balance, created_at, updated_at FROM users WHERE id = $1`

	user := models.User{}
	err := usersDB.QueryRow(ctx, query, userID).Scan(&user.ID, &user.Phone, &user.Email,
		&user.Balance, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return &user, nil
}

func (s *UserService) MarkUserAsBlocked(ctx context.Context, userID int64) error {
	_, shardID, _ := id.ParseUserID(userID)

	usersDB, ok := s.ShardManager.UserShards[shardID]
	if !ok {
		return fmt.Errorf("user shard %d not found for id %d", shardID, userID)
	}

	const query = `UPDATE users SET is_blocked = true WHERE id = $1`

	rows, err := usersDB.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("failed to get user: %w", err)
	}
	if rows.RowsAffected() != 1 {
		return fmt.Errorf("expected 1 rows affected, got %d", rows.RowsAffected())
	}

	return nil
}

func (s *UserService) CreateUserRecord(ctx context.Context, userID int64, phone, email string) error {
	userShard := s.ShardManager.HashPhoneNumber(phone)
	usersDB, ok := s.ShardManager.UserShards[userShard]
	if !ok {
		return fmt.Errorf("user shard %d not found", userShard)
	}

	now := time.Now().UTC()

	_, err := usersDB.Exec(ctx, `INSERT INTO users (id, phone_number, email, balance, created_at, updated_at) 
								 VALUES ($1, $2, $3, $4, $5, $6)`, userID, phone, email, WelcomeBonus, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert user: %w", err)
	}

	return nil
}

func (s *UserService) DeleteUserRecordIfPresentByUserID(ctx context.Context, userID int64) error {
	_, shardID, _ := id.ParseUserID(userID)
	shardDB, ok := s.ShardManager.UserShards[shardID]
	if !ok {
		return fmt.Errorf("shard not found %d", shardID)
	}
	_, err := shardDB.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to delete user record for userID %d: %w", userID, err)
	}

	return nil
}

func (s *UserService) DeleteEmailRecordIfPresentByUserID(ctx context.Context, email string) error {
	shardID := s.ShardManager.HashEmail(email)
	shardDB, ok := s.ShardManager.EmailShards[shardID]
	if !ok {
		return fmt.Errorf("shard not found %d", shardID)
	}
	_, err := shardDB.Exec(ctx, `DELETE FROM emails WHERE email = $1`, email)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("failed to delete user record for userID %s: %w", email, err)
	}

	return nil
}

func (s *UserService) CreateEmailRecord(ctx context.Context, userID int64, email string) error {
	emailShard := s.ShardManager.HashEmail(email)
	emailsDB, ok := s.ShardManager.EmailShards[emailShard]
	if !ok {
		return fmt.Errorf("email shard %d not found", emailShard)
	}

	now := time.Now().UTC()
	_, err := emailsDB.Exec(ctx, "INSERT INTO emails (email, user_id, created_at, updated_at) VALUES ($1, $2, $3, $4)",
		email, userID, now, now)
	if err != nil {
		return fmt.Errorf("failed to insert email: %w", err)
	}

	return nil
}

func (s *UserService) DecreaseMoneyFromUser(
	ctx context.Context,
	transactionID string,
	transactionType models.TransactionType,
	fromUserID,
	toUserID int64,
	amount int64,
) error {
	// Check for negative amount
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}

	_, shardID, _ := id.ParseUserID(fromUserID)
	userDB, ok := s.ShardManager.UserShards[shardID]
	if !ok {
		return fmt.Errorf("user shard %d not found", shardID)
	}

	now := time.Now().UTC()
	err := shard.WithTransaction(ctx, userDB, func(tx pgx.Tx) error {
		// check idempotentency key
		const query = `INSERT INTO idempotence (id, type, created_at) VALUES ($1, $2, $3)`
		_, err := tx.Exec(ctx, query, transactionID, transactionType, now)
		if err != nil {
			if pgErr, ok := lo.ErrorsAs[*pgconn.PgError](err); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return nil
				}
			}
			return fmt.Errorf("failed to insert idempotetency: %w", err)
		}

		// select user to check balance and blocked status
		var userIDFromDB uint64
		var balance int64
		var isBlocked bool
		const selectUser = `SELECT id, balance, is_blocked FROM users WHERE id = $1 FOR UPDATE`
		err = tx.QueryRow(ctx, selectUser, fromUserID).Scan(&userIDFromDB, &balance, &isBlocked)
		if err != nil {
			return fmt.Errorf("failed to select user: %w", err)
		}

		if isBlocked {
			return apperrors.ErrUserIsBlocked
		}

		if balance < amount {
			return fmt.Errorf("balance lower than amount of user")
		}

		// decrease user money
		const decreaseMoney = `UPDATE users SET balance = balance - $1 WHERE id = $2`
		rows, err := tx.Exec(ctx, decreaseMoney, amount, fromUserID)
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}
		if rows.RowsAffected() != 1 {
			return fmt.Errorf("failed to update balance: expected 1 row affected, got %d", rows.RowsAffected())
		}

		// add transaction history
		const addHistoryQuery = `INSERT INTO transaction 
    							 (id, from_id, to_id, amount, created_at) 
								 VALUES ($1, $2, $3, $4, $5)`
		_, err = tx.Exec(ctx, addHistoryQuery, uuid.NewString(), fromUserID, toUserID, amount, now)
		if err != nil {
			return fmt.Errorf("failed to insert transaction history: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) IncreaseMoneyToUser(
	ctx context.Context,
	transactionID string,
	transactionType models.TransactionType,
	fromUserID,
	toUserID int64,
	amount int64,
) error {
	// Check for negative amount
	if amount < 0 {
		return fmt.Errorf("amount cannot be negative")
	}

	_, shardID, _ := id.ParseUserID(toUserID)
	userDB, ok := s.ShardManager.UserShards[shardID]
	if !ok {
		return fmt.Errorf("user shard %d not found", shardID)
	}

	now := time.Now().UTC()
	err := shard.WithTransaction(ctx, userDB, func(tx pgx.Tx) error {
		// check idempotentency key
		const query = `INSERT INTO idempotence (id, type, created_at) VALUES ($1, $2, $3)`
		_, err := tx.Exec(ctx, query, transactionID, transactionType, now)
		if err != nil {
			if pgErr, ok := lo.ErrorsAs[*pgconn.PgError](err); ok {
				if pgErr.Code == pgerrcode.UniqueViolation {
					return nil
				}
			}
			return fmt.Errorf("failed to insert idempotetency: %w", err)
		}

		// select user to check balance
		var isBlocked bool
		const selectUser = `SELECT is_blocked FROM users WHERE id = $1 FOR UPDATE`
		err = tx.QueryRow(ctx, selectUser, toUserID).Scan(&isBlocked)
		if err != nil {
			return fmt.Errorf("failed to select user: %w", err)
		}

		if isBlocked {
			return apperrors.ErrUserIsBlocked
		}

		// increase user money
		const increaseMoney = `UPDATE users SET balance = balance + $1 WHERE id = $2`
		rows, err := tx.Exec(ctx, increaseMoney, amount, toUserID)
		if err != nil {
			return fmt.Errorf("failed to update balance: %w", err)
		}
		if rows.RowsAffected() != 1 {
			return fmt.Errorf("failed to update balance: expected 1 row affected, got %d", rows.RowsAffected())
		}

		// add transaction history
		const addHistoryQuery = `INSERT INTO transaction
    							 (id, from_id, to_id, amount, created_at) 
								 VALUES ($1, $2, $3, $4, $5)`
		_, err = tx.Exec(ctx, addHistoryQuery, uuid.NewString(), fromUserID, toUserID, amount, now)
		if err != nil {
			return fmt.Errorf("failed to insert transaction history: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetShardManager() *shard.ShardManager {
	return s.ShardManager
}
