//go:build !debug

package profile

import (
	"log"
	"net/http"
)

func StartPprof() {
	go func() {
		log.Println("pprof enabled on :6060")
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
