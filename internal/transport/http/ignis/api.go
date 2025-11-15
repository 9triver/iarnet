package ignis

import (
	"github.com/9triver/iarnet/internal/domain/ignis"
	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, platform *ignis.Platform) {
	// router.HandleFunc("/ignis/platform", platform.GetPlatform).Methods("GET")
}
