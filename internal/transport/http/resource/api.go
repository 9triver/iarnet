package resource

import (
	"github.com/9triver/iarnet/internal/domain/resource"
	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, resMgr *resource.Manager) {
	// router.HandleFunc("/resource/capacity", resMgr.GetResourceCapacity).Methods("GET")
}
