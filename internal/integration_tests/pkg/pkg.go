package pkg

import (
	"context"
	"testing"
	"time"
	"usershards/internal/saga"

	"go.temporal.io/sdk/client"
	"usershards/internal/config"
	"usershards/internal/logger"
	"usershards/internal/services"
	"usershards/internal/shard"
)

// TestDeps хранит зависимости для тестов
type TestDeps struct {
	ShardManager   *shard.ShardManager
	TemporalClient client.Client
	UserService    *services.UserService
	UserSaga       *saga.UserSagaWorkflow
}

type Setup struct {
	SetupUserService func(shardManager *shard.ShardManager) *services.UserService
}

// SetupTest инициализирует зависимости для тестирования
func SetupTest(t *testing.T, setupSet Setup) *TestDeps {
	t.Helper()
	logger.InitLogger()

	const configPath = "../pkg/config.yaml"
	conf, err := config.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	ctx := context.Background()

	// Инициализация ShardManager
	shardManager, err := shard.NewShardManager(ctx, conf)
	if err != nil {
		t.Fatalf("failed to initialize ShardManager: %v", err)
	}

	// Очистка БД перед тестами
	if err := shardManager.ClearDatabases(ctx); err != nil {
		t.Fatalf("failed to reset shards: %v", err)
	}

	// Initialize Temporal client
	temporalClient, err := client.Dial(client.Options{})
	if err != nil {
		t.Fatal("Unable to create Temporal client", err)
	}
	logger.Logger.Info("Connected to Temporal successfully")

	userService := services.NewUserService(shardManager)
	userSaga := saga.NewUserSagaWorkflow(userService, temporalClient)

	w1, w2 := saga.NewWorker(temporalClient, userSaga)

	// Очищаем ресурсы после теста
	t.Cleanup(func() {
		t.Log("Cleaning up temporal client")
		w1.Stop()
		w2.Stop()
		temporalClient.Close()
		shardManager.Close()
	})

	// Ждём инициализации перед началом тестов
	time.Sleep(500 * time.Millisecond)

	return &TestDeps{
		ShardManager:   shardManager,
		TemporalClient: temporalClient,
		UserService:    userService,
		UserSaga:       userSaga,
	}
}
