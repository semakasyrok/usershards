package saga

import (
	"context"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"go.uber.org/zap"
	"time"
	"usershards/internal/logger"
	"usershards/internal/models"
	"usershards/internal/shard"
)

const TaskQueue = "user-task-queue"
const TransferTaskQueue = "transfer-task-queue"

type userService interface {
	CreateUserRecord(ctx context.Context, userID int64, phone, email string) error
	DeleteUserRecordIfPresentByUserID(ctx context.Context, userID int64) error
	DeleteEmailRecordIfPresentByUserID(ctx context.Context, email string) error
	CreateEmailRecord(ctx context.Context, userID int64, email string) error
	DecreaseMoneyFromUser(
		ctx context.Context,
		transactionID string,
		transactionType models.TransactionType,
		fromUserID,
		toUserID int64,
		amount int64,
	) error
	IncreaseMoneyToUser(
		ctx context.Context,
		transactionID string,
		transactionType models.TransactionType,
		fromUserID,
		toUserID int64,
		amount int64,
	) error
	GetShardManager() *shard.ShardManager
}

type userSagaService interface {
	CreateUserRecord(ctx context.Context, userID int64, phone, email string) error
	DeleteUserRecordIfPresentByUserID(ctx context.Context, userID int64) error
	DeleteEmailRecordIfPresentByUserID(ctx context.Context, email string) error
	CreateEmailRecord(ctx context.Context, userID int64, email string) error
	CreateUserWorkflow(ctx workflow.Context, userID int64, phone, email string) (res int64, err error)
	GetShardManager() *shard.ShardManager
	TransferMoneyWorkflow(ctx workflow.Context, params TransferMoneyParams) error
	DecreaseMoney(ctx context.Context, params TransferMoneyParams) error
	CompensateMoney(ctx context.Context, params TransferMoneyParams) error
	IncreaseMoney(ctx context.Context, params TransferMoneyParams) error
}

// startWorker is a helper function that starts a worker and waits for confirmation
// that it has started successfully.
func startWorker(w worker.Worker, workerName string) {
	workerErrCh := make(chan error, 1)
	stopChan := make(<-chan interface{}, 1)

	go func() {
		err := w.Run(stopChan)
		if err != nil {
			logger.Logger.Error(workerName+" worker error", zap.Error(err))
		}
		workerErrCh <- err
	}()

	// Wait for confirmation that the worker started
	select {
	case err := <-workerErrCh:
		logger.Logger.Fatal(workerName+" worker failed to start", err)
	case <-time.After(time.Second): // Give time for worker to start
		logger.Logger.Info(workerName + " worker started successfully")
	}
}

func NewWorker(temporalClient client.Client, service userSagaService) (worker.Worker, worker.Worker) {
	// Create and start user worker
	userWorker := worker.New(temporalClient, TaskQueue, worker.Options{})

	// Register user workflow and activities
	userWorker.RegisterWorkflow(service.CreateUserWorkflow)
	userWorker.RegisterActivity(service.CreateUserRecord)
	userWorker.RegisterActivity(service.CreateEmailRecord)
	userWorker.RegisterActivity(service.DeleteUserRecordIfPresentByUserID)
	userWorker.RegisterActivity(service.DeleteEmailRecordIfPresentByUserID)

	// Start the user worker
	startWorker(userWorker, "User")

	// Create and start transfer worker
	transferWorker := worker.New(temporalClient, TransferTaskQueue, worker.Options{})

	// Register transfer workflow and activities
	transferWorker.RegisterWorkflow(service.TransferMoneyWorkflow)
	transferWorker.RegisterActivity(service.IncreaseMoney)
	transferWorker.RegisterActivity(service.DecreaseMoney)
	transferWorker.RegisterActivity(service.CompensateMoney)

	// Start the transfer worker
	startWorker(transferWorker, "Transfer")

	return userWorker, transferWorker
}
