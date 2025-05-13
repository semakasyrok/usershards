package user

import (
	"context"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
	"usershards/internal/integration_tests/pkg"
	"usershards/internal/services"
)

func TestTransferMoney(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*1)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: transfer 10 rubles from user1 to user2
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	require.NoError(t, err)

	// step 3: check that we decrease money from user1 and add money to user2
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, user1.Balance, int64(services.WelcomeBonus-transferAmount))
	require.Equal(t, user2.Balance, int64(services.WelcomeBonus+transferAmount))
}

func TestTransferMoney_SecondUserIsBlocked(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: mark second user as blocked to fail our transaction
	err = deps.UserService.MarkUserAsBlocked(ctx, userID2)
	require.NoError(t, err)

	// step 3: transfer 10 rubles from user1 to user2
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 4: check that we return money to first user
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test when the sender user is blocked
func TestTransferMoney_FirstUserIsBlocked(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: mark first user as blocked to fail our transaction
	err = deps.UserService.MarkUserAsBlocked(ctx, userID1)
	require.NoError(t, err)

	// step 3: transfer 10 rubles from user1 to user2
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 4: check that balances remain unchanged
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test when the sender doesn't have enough balance
func TestTransferMoney_InsufficientBalance(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: transfer more money than user1 has
	const transferAmount = services.WelcomeBonus + 1_00 // 1 more than available
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 3: check that balances remain unchanged
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test with a zero amount transfer
func TestTransferMoney_ZeroAmount(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: transfer 0 rubles from user1 to user2
	const transferAmount = 0
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	// The behavior here depends on the implementation - it might allow or disallow zero transfers
	// For this test, we'll assume it's allowed
	require.NoError(t, err)

	// step 3: check that balances remain unchanged
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test when the sender doesn't exist
func TestTransferMoney_SenderDoesNotExist(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create only user2
	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: try to transfer from a non-existent user1 to user2
	nonExistentUserID := int64(999999) // Assuming this ID doesn't exist
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, nonExistentUserID, userID2, transferAmount)
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 3: check that user2's balance remains unchanged
	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test when the recipient doesn't exist
func TestTransferMoney_RecipientDoesNotExist(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create only user1
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	// step 2: try to transfer from user1 to a non-existent user2
	nonExistentUserID := int64(999999) // Assuming this ID doesn't exist
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, userID1, nonExistentUserID, transferAmount)
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 3: check that user1's balance remains unchanged (compensation worked)
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)
	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
}

// Test with a negative amount transfer
func TestTransferMoney_NegativeAmount(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user1 and user2 with some money
	phone1 := "+79133971111"
	email1 := "test1@test.ru"
	userID1, err := deps.UserSaga.CreateUser(ctx, phone1, email1)
	require.NoError(t, err)

	phone2 := "+79133971112"
	email2 := "test2@test.ru"
	userID2, err := deps.UserSaga.CreateUser(ctx, phone2, email2)
	require.NoError(t, err)

	// step 2: transfer negative amount from user1 to user2
	const transferAmount = -10_00
	err = deps.UserSaga.TransferMoney(ctx, userID1, userID2, transferAmount)
	// The behavior here depends on the implementation - it should reject negative transfers
	require.Error(t, err)
	time.Sleep(time.Second * 5)

	// step 3: check that balances remain unchanged
	user1, err := deps.UserService.GetUserByID(ctx, userID1)
	require.NoError(t, err)

	user2, err := deps.UserService.GetUserByID(ctx, userID2)
	require.NoError(t, err)

	require.Equal(t, int64(services.WelcomeBonus), user1.Balance)
	require.Equal(t, int64(services.WelcomeBonus), user2.Balance)
}

// Test transferring money from a user to themselves
func TestTransferMoney_SameUser(t *testing.T) {
	deps := pkg.SetupTest(t, pkg.Setup{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// step 1: create user
	phone := "+79133971111"
	email := "test1@test.ru"
	userID, err := deps.UserSaga.CreateUser(ctx, phone, email)
	require.NoError(t, err)

	// step 2: transfer money from user to themselves
	const transferAmount = 10_00
	err = deps.UserSaga.TransferMoney(ctx, userID, userID, transferAmount)
	// The behavior here depends on the implementation - it might allow or disallow self-transfers
	// For this test, we'll check the actual behavior
	if err != nil {
		// If self-transfers are not allowed, check that the balance remains unchanged
		user, err := deps.UserService.GetUserByID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, int64(services.WelcomeBonus), user.Balance)
	} else {
		// If self-transfers are allowed, the balance should remain the same
		// (decrease and increase cancel each other out)
		user, err := deps.UserService.GetUserByID(ctx, userID)
		require.NoError(t, err)
		require.Equal(t, int64(services.WelcomeBonus), user.Balance)
	}
}

// transfer with services

// workflow by first attempt is error
