package saga

import (
	"context"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
	"time"
	"usershards/internal/logger"
)

const TaskQueue = "user-task-queue"

type userService interface {
	CreateUserWorkflow(ctx workflow.Context, userID int64, phone, email string) (res int64, err error)
	CreateUserRecord(ctx context.Context, userID int64, phone, email string) error
	DeleteUserRecordIfPresentByUserID(ctx context.Context, userID int64) error
	DeleteEmailRecordIfPresentByUserID(ctx context.Context, email string) error
	CreateEmailRecord(ctx context.Context, userID int64, email string) error
}

func NewWorker(temporalClient client.Client, service userService) worker.Worker {
	w := worker.New(temporalClient, TaskQueue, worker.Options{})

	// Регистрируем workflow
	w.RegisterWorkflow(service.CreateUserWorkflow)
	w.RegisterActivity(service.CreateUserRecord)
	w.RegisterActivity(service.CreateEmailRecord)
	w.RegisterActivity(service.DeleteUserRecordIfPresentByUserID)
	w.RegisterActivity(service.DeleteEmailRecordIfPresentByUserID)

	// Запускаем worker и ждём его старта
	workerErrCh := make(chan error, 1)
	stopChan := make(<-chan interface{}, 1)
	go func() {
		workerErrCh <- w.Run(stopChan)
	}()

	// Ждём подтверждения, что worker запустился
	select {
	case err := <-workerErrCh:
		logger.Logger.Fatal("Worker failed to start", err)
	case <-time.After(time.Second): // Даём время на старт worker'а
		logger.Logger.Info("Worker started successfully")
	}

	// Теперь можно запускать workflow
	//
	//select {
	//case <-worker.InterruptCh():
	//	logger.Logger.Info("Interrupt signal received, shutting down...")
	//}

	return w
}
