package main

import (
	"context"
	"go.temporal.io/sdk/client"
	"log"
	"usershards/internal/config"
	"usershards/internal/logger"
	"usershards/internal/profile"
	"usershards/internal/shard"
)

func main() {
	err := run()
	if err != nil {
		logger.Logger.Fatal(err)
	}
}

func run() error {
	profile.StartPprof()

	logger.InitLogger()
	defer logger.Logger.Sync()

	conf, err := config.LoadConfig("config.yaml")
	if err != nil {
		return err
	}

	ctx := context.Background()

	shardManager, err := shard.NewShardManager(ctx, conf)
	if err != nil {
		return err
	}
	defer shardManager.Close()

	// Initialize Temporal client
	temporalClient, err := client.Dial(client.Options{})
	if err != nil {
		log.Fatal("Unable to create Temporal client", err)
	}
	defer temporalClient.Close()
	logger.Logger.Info("Connected to Temporal successfully")

	//userService := services.NewUserService(shardManager, temporalClient)
	//simpleService := services.NewSimpleService()
	//
	//worker, _ := saga.NewWorker(temporalClient, simpleService, userService)
	//defer worker.Stop()
	//
	//tctx, cancel := context.WithTimeout(ctx, time.Second*10)
	//defer cancel()
	//_, err = userService.CreateUser(tctx, "+7823434335", "bob@qaz.ru")

	select {}

	return nil
}
