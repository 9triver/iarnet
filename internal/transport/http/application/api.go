package application

import (
	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, am *application.Manager) {
	// api := NewAPI(am)
	// router.HandleFunc("/application/apps", api.handleGetApplications).Methods("GET")
	// router.HandleFunc("/application/apps", api.handleCreateApplication).Methods("POST")
	// router.HandleFunc("/application/apps/{id}", api.handleGetApplicationById).Methods("GET")
	// router.HandleFunc("/application/apps/{id}", api.handleDeleteApplication).Methods("DELETE")
}

type API struct {
	am *application.Manager
}

func NewAPI(am *application.Manager) *API {
	return &API{am: am}
}

// func (api *API) handleGetApplications(w http.ResponseWriter, r *http.Request) {
// 	apps, err := api.am.GetApplications()
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(apps)
// }

// func (api *API) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
// 	app, err := api.am.CreateApplication(r.Body)
// 	if err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	json.NewEncoder(w).Encode(app)
// }
