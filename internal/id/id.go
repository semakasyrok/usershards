package id

import (
	"math/rand"
	"time"
)

// Начало отсчета времени (01.01.2024)
const customEpoch = 1704067200000 // Timestamp в миллисекундах

// Генерация 64-битного ID с timestamp и shardID
func GenerateUserID(shardID int) int64 {
	if shardID < 0 || shardID > 3 {
		panic("Shard ID должен быть от 0 до 3")
	}

	// Текущий timestamp (в миллисекундах)
	now := time.Now().UnixMilli() - customEpoch

	// Генерируем случайный счетчик (20 бит)
	counter := rand.Int63n(1 << 20) // 0 - 1048575

	// Собираем 64-битный ID
	return (now << 22) | (int64(shardID) << 20) | counter
}

// Расшифровка ID
func ParseUserID(userID int64) (time.Time, int, int64) {
	// Извлекаем timestamp
	timestamp := (userID >> 22) + customEpoch
	createdAt := time.UnixMilli(timestamp)

	// Извлекаем номер шарда (2 бита)
	shardID := (userID >> 20) & 3

	// Извлекаем счетчик (20 бит)
	counter := userID & ((1 << 20) - 1)

	return createdAt, int(shardID), counter
}
