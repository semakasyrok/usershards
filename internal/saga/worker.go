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

func NewWorker(temporalClient client.Client, service userSagaService) (worker.Worker, worker.Worker) {
	w2 := worker.New(temporalClient, TaskQueue, worker.Options{})

	// Регистрируем workflow
	w2.RegisterWorkflow(service.CreateUserWorkflow)

	w2.RegisterActivity(service.CreateUserRecord)
	w2.RegisterActivity(service.CreateEmailRecord)
	w2.RegisterActivity(service.DeleteUserRecordIfPresentByUserID)
	w2.RegisterActivity(service.DeleteEmailRecordIfPresentByUserID)

	workerErrCh2 := make(chan error, 1)
	stopChan2 := make(<-chan interface{}, 1)
	go func() {
		err := w2.Run(stopChan2)
		if err != nil {
			logger.Logger.Error("worker error", zap.Error(err))
		}
		workerErrCh2 <- err
	}()

	// Ждём подтверждения, что worker запустился
	select {
	case err := <-workerErrCh2:
		logger.Logger.Fatal("Worker failed to start", err)
	case <-time.After(time.Second): // Даём время на старт worker'а
		logger.Logger.Info("Worker started successfully")
	}

	//=========

	w3 := worker.New(temporalClient, TransferTaskQueue, worker.Options{})

	// Регистрируем workflow
	w3.RegisterWorkflow(service.TransferMoneyWorkflow)
	w3.RegisterActivity(service.IncreaseMoney)
	w3.RegisterActivity(service.DecreaseMoney)
	w3.RegisterActivity(service.CompensateMoney)

	workerErrCh3 := make(chan error, 1)
	stopChan3 := make(<-chan interface{}, 1)
	go func() {
		err := w3.Run(stopChan3)
		if err != nil {
			logger.Logger.Error("worker error", zap.Error(err))
		}
		workerErrCh3 <- err
	}()

	// Ждём подтверждения, что worker запустился
	select {
	case err := <-workerErrCh3:
		logger.Logger.Fatal("Worker failed to start", err)
	case <-time.After(time.Second): // Даём время на старт worker'а
		logger.Logger.Info("Worker started successfully")
	}

	return w2, w3
}
