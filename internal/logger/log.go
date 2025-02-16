package logger

import (
	"go.uber.org/zap"
	"sync"
	"sync/atomic"
	"unsafe"
)

var Logger *zap.SugaredLogger

func L() *zap.SugaredLogger {
	return (*zap.SugaredLogger)(atomic.LoadPointer((*unsafe.Pointer)(unsafe.Pointer(&Logger))))
}

func InitLogger() {
	sync.OnceFunc(func() {
		logger, _ := zap.NewProduction()
		defer logger.Sync() // flushes buffer, if any
		sugar := logger.Sugar()
		atomic.StorePointer((*unsafe.Pointer)(unsafe.Pointer(&Logger)), unsafe.Pointer(sugar))
	})()
}
