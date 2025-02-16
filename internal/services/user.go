package services

import (
	"context"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	"time"
	"usershards/internal/id"
	"usershards/internal/saga"
	"usershards/internal/shard"
)

func NewUserService(manager *shard.ShardManager, client client.Client) *UserService {
	return &UserService{
		ShardManager:   manager,
		TemporalClient: client,
	}
}

type UserService struct {
	ShardManager   *shard.ShardManager
	TemporalClient client.Client
}

// CreateUser создает пользователя через Temporal Workflow
// https://temporal.io/blog/compensating-actions-part-of-a-complete-breakfast-with-sagas
func (s *UserService) CreateUser(ctx context.Context, phone, email string) (int64, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-user-%s", phone),
		TaskQueue: saga.TaskQueue,
	}
	userID := id.GenerateUserID(s.ShardManager.HashPhoneNumber(phone))

	// Запускаем workflows
	we, err := s.TemporalClient.ExecuteWorkflow(ctx, workflowOptions, s.CreateUserWorkflow, userID, phone, email)
	if err != nil {
		return 0, fmt.Errorf("failed to start workflows: %w", err)
	}

	// Ожидаем завершения (синхронно)
	var resUserID int64
	err = we.Get(ctx, &resUserID)
	if err != nil {
		return 0, fmt.Errorf("failed to get workflows result: %w", err)
	}

	return resUserID, nil
}

func (s *UserService) CreateUserWorkflow(ctx workflow.Context, userID int64, phone, email string) (res int64, err error) {

	options := s.getDefaultOptions()
	ctx = workflow.WithActivityOptions(ctx, options)
	logger := workflow.GetLogger(ctx)
	logger.Debug("CreateUserWorkflow start")

	logger.Debug("CreateEmailRecord start")
	err = workflow.ExecuteActivity(ctx, s.CreateEmailRecord, userID, email).Get(ctx, nil)
	if err != nil {
		logger.Debug("CreateEmailRecord fails", zap.Error(err))
		return 0, err
	}
	logger.Debug("CreateEmailRecord stop")

	logger.Debug("CreateUserRecord start")
	err = workflow.ExecuteActivity(ctx, s.CreateUserRecord, userID, phone, email).Get(ctx, nil)
	if err != nil {
		delEmailErr := workflow.ExecuteActivity(ctx, s.DeleteEmailRecordIfPresentByUserID, email).Get(ctx, nil)
		if delEmailErr != nil {
			return 0, fmt.Errorf("failed to delete email record: %w", delEmailErr)
		}
		logger.Debug("CreateUserRecord fails", zap.Error(err))
		return 0, err
	}
	logger.Debug("CreateUserRecord stop")

	logger.Debug("CreateUserWorkflow completed")
	return userID, nil
}

func (s *UserService) CreateUserRecord(ctx context.Context, userID int64, phone, email string) error {
	userShard := s.ShardManager.HashPhoneNumber(phone)
	usersDB, ok := s.ShardManager.UserShards[userShard]
	if !ok {
		return fmt.Errorf("user shard %d not found", userShard)
	}

	now := time.Now().UTC()
	_, err := usersDB.Exec(ctx, `INSERT INTO users (id, phone_number, email, balance, created_at, updated_at) 
								 VALUES ($1, $2, $3, $4, $5, $6)`, userID, phone, email, 0, now, now)
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

func (s *UserService) getDefaultOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		ScheduleToCloseTimeout: 10 * time.Second,
		StartToCloseTimeout:    5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    5 * time.Second,
			MaximumAttempts:    3,
		},
	}
}
