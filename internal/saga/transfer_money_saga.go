package saga

import (
	"context"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	"time"
	"usershards/internal/apperrors"
	"usershards/internal/models"
)

type step int

const stepNoCompensations step = 0
const stepIncreaseFailed step = 1

type TransferMoneyParams struct {
	From          int64
	To            int64
	TransactionID string
	Amount        int64
}

func (s *UserSagaWorkflow) TransferMoney(ctx context.Context, from, to int64, amount int64) error {
	// move outside
	transactionID, err := uuid.NewV7()
	if err != nil {
		return err
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        transactionID.String(),
		TaskQueue: TransferTaskQueue,
	}

	params := TransferMoneyParams{
		From:          from,
		To:            to,
		TransactionID: transactionID.String(),
		Amount:        amount,
	}

	we, err := s.temporalClient.ExecuteWorkflow(ctx, workflowOptions, s.TransferMoneyWorkflow, params)
	if err != nil {
		return fmt.Errorf("failed to start workflows: %w", err)
	}

	var resUserID int64
	err = we.Get(ctx, &resUserID)
	if err != nil {
		return fmt.Errorf("failed to get workflows result: %w", err)
	}

	return nil
}

func (s *UserSagaWorkflow) TransferMoneyWorkflow(ctx workflow.Context, params TransferMoneyParams) error {
	ctx = workflow.WithActivityOptions(ctx, s.getDefaultOptions())
	logger := workflow.GetLogger(ctx)
	logger.Debug("TransferMoneyWorkflow start")

	err := workflow.ExecuteActivity(ctx, s.DecreaseMoney, params).Get(ctx, nil)
	if err != nil {
		return s.Compensations(ctx, stepNoCompensations, err, params)
	}

	err = workflow.ExecuteActivity(ctx, s.IncreaseMoney, params).Get(ctx, nil)
	if err != nil {
		return s.Compensations(ctx, stepIncreaseFailed, err, params)
	}

	logger.Debug("TransferMoneyWorkflow stop")
	return nil
}

func (s *UserSagaWorkflow) DecreaseMoney(
	ctx context.Context,
	params TransferMoneyParams,
) error {
	logger := activity.GetLogger(ctx)
	logger.Debug("DecreaseMoney start")
	err := s.userService.DecreaseMoneyFromUser(ctx, params.TransactionID, models.TransactionTypeDecrease, params.From, params.To, params.Amount)
	if err != nil {
		logger.Error("DecreaseMoney fails", zap.Error(err))
	}

	return err
}

func (s *UserSagaWorkflow) IncreaseMoney(
	ctx context.Context,
	params TransferMoneyParams,
) error {
	logger := activity.GetLogger(ctx)
	logger.Debug("IncreaseMoney start")
	err := s.userService.IncreaseMoneyToUser(ctx, params.TransactionID, models.TransactionTypeIncrease, params.From, params.To, params.Amount)
	if err != nil {
		logger.Error("IncreaseMoney fails", zap.Error(err))
		if errors.Is(err, apperrors.ErrUserIsBlocked) {
			return temporal.NewNonRetryableApplicationError(err.Error(), "apperrors.ErrUserIsBlocked", apperrors.ErrUserIsBlocked)
		}
	}

	return err
}

func (s *UserSagaWorkflow) CompensateMoney(
	ctx context.Context,
	params TransferMoneyParams,
) error {
	logger := activity.GetLogger(ctx)
	logger.Debug("CompensateMoney start")
	err := s.userService.IncreaseMoneyToUser(ctx, params.TransactionID, models.TransactionTypeCompensate, params.To, params.From, params.Amount)
	if err != nil {
		logger.Error("CompensateMoney fails", zap.Error(err))
	}

	return err
}

func (s *UserSagaWorkflow) Compensations(
	ctx workflow.Context,
	stepNumber step,
	err error,
	params TransferMoneyParams,
) error {
	logger := workflow.GetLogger(ctx)
	logger.Debug("Compensations start")

	switch stepNumber {
	case stepIncreaseFailed:
		logger.Debug("stepIncreaseFailed start")
		compensateErr := workflow.ExecuteActivity(ctx, s.CompensateMoney, params).Get(ctx, nil)
		if compensateErr != nil {
			logger.Debug("stepIncreaseFailed error", zap.Error(compensateErr))
			return compensateErr
		}
		fallthrough
	case stepNoCompensations:
		logger.Debug("stepNoCompensations  start")
		return temporal.NewNonRetryableApplicationError(apperrors.ErrCompensationCompleted.Error(),
			"apperrors.ErrCompensationCompleted", apperrors.ErrCompensationCompleted)
	}

	return err
}

func (s *UserSagaWorkflow) getDefaultOptions() workflow.ActivityOptions {
	return workflow.ActivityOptions{
		ScheduleToCloseTimeout: 10 * time.Second,
		StartToCloseTimeout:    5 * time.Second,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    10 * time.Second,
			MaximumAttempts:    5,
			NonRetryableErrorTypes: []string{
				"apperrors.ErrCompensationCompleted",
				"apperrors.ErrUserIsBlocked",
			},
		},
	}
}
