package saga

import (
	"context"
	"fmt"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	"usershards/internal/apperrors"
	"usershards/internal/id"
	"usershards/internal/shard"
)

// User saga step constants
type userStep int

const userStepNoCompensations userStep = 0
const userStepEmailCreated userStep = 1

type UserSagaWorkflow struct {
	userService    userService
	temporalClient client.Client
}

func NewUserSagaWorkflow(userService userService, client client.Client) *UserSagaWorkflow {
	return &UserSagaWorkflow{
		userService:    userService,
		temporalClient: client,
	}
}

func (s *UserSagaWorkflow) CreateUser(ctx context.Context, phone, email string) (int64, error) {
	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-user-%s", phone),
		TaskQueue: TaskQueue,
	}
	userID := id.GenerateUserID(s.userService.GetShardManager().HashPhoneNumber(phone))

	// Запускаем workflows
	we, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOptions, s.CreateUserWorkflow, userID, phone, email)
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

func (s *UserSagaWorkflow) CreateUserWorkflow(ctx workflow.Context, userID int64, phone, email string) (res int64, err error) {
	ctx = workflow.WithActivityOptions(ctx, s.getDefaultOptions())
	logger := workflow.GetLogger(ctx)
	logger.Debug("CreateUserWorkflow start")

	logger.Debug("CreateEmailRecord start")
	err = workflow.ExecuteActivity(ctx, s.CreateEmailRecord, userID, email).Get(ctx, nil)
	if err != nil {
		logger.Error("CreateEmailRecord fails", zap.Error(err))
		return 0, s.UserCompensations(ctx, userStepNoCompensations, err, userID, email)
	}
	logger.Debug("CreateEmailRecord stop")

	logger.Debug("CreateUserRecord start")
	err = workflow.ExecuteActivity(ctx, s.CreateUserRecord, userID, phone, email).Get(ctx, nil)
	if err != nil {
		logger.Error("CreateUserRecord fails", zap.Error(err))
		return 0, s.UserCompensations(ctx, userStepEmailCreated, err, userID, email)
	}
	logger.Debug("CreateUserRecord stop")

	logger.Debug("CreateUserWorkflow completed")
	return userID, nil
}

func (s *UserSagaWorkflow) CreateUserRecord(ctx context.Context, userID int64, phone, email string) error {
	return s.userService.CreateUserRecord(ctx, userID, phone, email)
}

func (s *UserSagaWorkflow) DeleteUserRecordIfPresentByUserID(ctx context.Context, userID int64) error {
	return s.userService.DeleteUserRecordIfPresentByUserID(ctx, userID)
}

func (s *UserSagaWorkflow) DeleteEmailRecordIfPresentByUserID(ctx context.Context, email string) error {
	return s.userService.DeleteEmailRecordIfPresentByUserID(ctx, email)
}

func (s *UserSagaWorkflow) CreateEmailRecord(ctx context.Context, userID int64, email string) error {
	return s.userService.CreateEmailRecord(ctx, userID, email)
}

func (s *UserSagaWorkflow) GetShardManager() *shard.ShardManager {
	return s.userService.GetShardManager()
}

// UserCompensations handles the compensation logic for the user saga
func (s *UserSagaWorkflow) UserCompensations(
	ctx workflow.Context,
	stepNumber userStep,
	err error,
	userID int64,
	email string,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Debug("User Compensations start")

	switch stepNumber {
	case userStepEmailCreated:
		logger.Debug("userStepEmailCreated compensation start")
		compensateErr := workflow.ExecuteActivity(ctx, s.DeleteEmailRecordIfPresentByUserID, email).Get(ctx, nil)
		if compensateErr != nil {
			logger.Debug("userStepEmailCreated compensation error", zap.Error(compensateErr))
			return compensateErr
		}
		fallthrough
	case userStepNoCompensations:
		logger.Debug("userStepNoCompensations start")
		return temporal.NewNonRetryableApplicationError(apperrors.ErrCompensationCompleted.Error(),
			"apperrors.ErrCompensationCompleted", apperrors.ErrCompensationCompleted)
	}

	return err
}
