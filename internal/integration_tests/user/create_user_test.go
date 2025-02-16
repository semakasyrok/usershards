package user

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"usershards/internal/id"
	"usershards/internal/integration_tests/pkg"
)

func TestCreateUser(t *testing.T) {
	deps := pkg.SetupTest(t)

	phone := "+79133971113"
	email := "test2@test.ru"

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*10)
	defer cancel()
	userID, err := deps.UserService.CreateUser(ctx, phone, email)
	require.NoError(t, err)
	_, shardID, _ := id.ParseUserID(userID)
	t.Log("created user with id:", userID, "shard_id:", shardID)
}

func TestCreateManyUsers(t *testing.T) {
	deps := pkg.SetupTest(t)

	counter := atomic.Uint32{}
	const generatedData = 1000
	const numberOfWorkers = 60
	const buffer = 200
	type fakeData struct {
		Email string
		Phone string
	}
	input := make(chan fakeData, buffer)

	wg := &sync.WaitGroup{}
	wg.Add(numberOfWorkers)
	for i := 0; i < numberOfWorkers; i++ {
		go func() {
			defer wg.Done()
			for data := range input {
				_, err := deps.UserService.CreateUser(context.TODO(), data.Phone, data.Email)
				require.NoError(t, err)
				current := counter.Add(1)
				if current%10 == 0 {
					t.Logf("execution status: %d/%d", current, generatedData)
				}
			}
		}()
	}

	start := time.Now()
	for i := 0; i < generatedData; i++ {
		ph := fmt.Sprintf("+1123%04d", i)
		em := fmt.Sprintf("user%04d@email.com", i)
		input <- fakeData{Email: em, Phone: ph}
	}
	close(input)

	wg.Wait()
	t.Logf("completed %d in %f seconds", generatedData, time.Since(start).Seconds())
}
