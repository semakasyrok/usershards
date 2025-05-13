//go:build debug

package profile

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func StartPprof() {
	go func() {
		log.Println("pprof enabled on :6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
