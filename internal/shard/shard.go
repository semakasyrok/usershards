package shard

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"hash/crc32"
	"usershards/internal/config"
	"usershards/internal/logger"
	"usershards/migrations/emails"
	"usershards/migrations/users"
)

// TxFunc defines the function signature for transaction operations.
type TxFunc func(tx pgx.Tx) error

// WithTransaction executes a function within a database transaction.
func WithTransaction(ctx context.Context, db *pgxpool.Pool, fn TxFunc) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Ensure rollback if the function returns an error
	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p) // Re-panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("transaction failed: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

type ShardManager struct {
	UserShards  map[int]*pgxpool.Pool
	EmailShards map[int]*pgxpool.Pool
}

// NewShardManager создает новый менеджер шардов и инициализирует подключения
func NewShardManager(ctx context.Context, config *config.Config) (*ShardManager, error) {
	sm := &ShardManager{
		UserShards:  make(map[int]*pgxpool.Pool),
		EmailShards: make(map[int]*pgxpool.Pool),
	}

	// Инициализация user-shards
	for shardID, connStr := range config.DB.UserShards {
		conn, err := pgxpool.New(ctx, connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to user shard %d: %w", shardID, err)
		}

		// Проверяем доступность базы данных
		if err := conn.Ping(ctx); err != nil {
			return nil, fmt.Errorf("failed to ping user shard %d: %w", shardID, err)
		}

		err = RunMigrations(conn, users.Migration1, users.Migration2, users.Migration3)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}

		sm.UserShards[shardID] = conn
	}

	// Инициализация email-shards
	for shardID, connStr := range config.DB.EmailShards {
		conn, err := pgxpool.New(ctx, connStr)
		if err != nil {
			return nil, fmt.Errorf("failed to connect to email shard %d: %w", shardID, err)
		}

		// Проверяем доступность базы данных
		if err := conn.Ping(ctx); err != nil {
			return nil, fmt.Errorf("failed to ping email shard %d: %w", shardID, err)
		}

		err = RunMigrations(conn, emails.Migration1, emails.Migration2)
		if err != nil {
			return nil, fmt.Errorf("failed to run migrations: %w", err)
		}
		sm.EmailShards[shardID] = conn
	}

	return sm, nil
}

// Close закрывает все соединения с шардированными базами данных
func (sm *ShardManager) Close() {
	// Закрываем соединения для user-shards
	for _, conn := range sm.UserShards {
		conn.Close()
	}

	// Закрываем соединения для email-shards
	for _, conn := range sm.EmailShards {
		conn.Close()
	}
}

// HashPhoneNumber хеширует номер телефона и возвращает индекс шардирования
func (sm *ShardManager) HashPhoneNumber(phoneNumber string) int {
	// Хешируем номер телефона
	hash := crc32.ChecksumIEEE([]byte(phoneNumber))

	// Возвращаем индекс шардирования, деля хеш на количество шардов
	return int(hash) % len(sm.UserShards)
}

func (sm *ShardManager) HashEmail(email string) int {
	// Хешируем номер телефона
	hash := crc32.ChecksumIEEE([]byte(email))

	// Возвращаем индекс шардирования, деля хеш на количество шардов
	return int(hash) % len(sm.EmailShards)
}

// RunMigrations выполняет SQL-скрипты миграции
func RunMigrations(conn *pgxpool.Pool, migrations ...string) error {
	for _, migration := range migrations {
		_, err := conn.Exec(context.Background(), migration)
		if err != nil {
			return fmt.Errorf("ошибка выполнения миграции: %w", err)
		}
	}

	logger.Logger.Infof("migration successfuly executed")
	return nil
}

// ClearDatabases очищает все данные в шардах
func (sm *ShardManager) ClearDatabases(ctx context.Context) error {
	queries := []string{
		"DELETE FROM users",
		"DELETE FROM idempotence",
		"DELETE FROM transaction",
	}

	for _, conn := range sm.UserShards {
		for _, query := range queries {
			if _, err := conn.Exec(ctx, query); err != nil {
				return err
			}
		}
	}

	for _, conn := range sm.EmailShards {
		if _, err := conn.Exec(ctx, "DELETE FROM emails"); err != nil {
			return err
		}
	}

	return nil
}
