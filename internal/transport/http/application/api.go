package application

import (
	"encoding/json"
	"net/http"

	"github.com/9triver/iarnet/internal/domain/application"
	"github.com/9triver/iarnet/internal/transport/http/util/response"
	"github.com/gorilla/mux"
)

func RegisterRoutes(router *mux.Router, am *application.Manager) {
	api := NewAPI(am)
	router.HandleFunc("/application/apps", api.handleGetApplicationList).Methods("GET")
	router.HandleFunc("/application/apps", api.handleCreateApplication).Methods("POST")
	// router.HandleFunc("/application/apps/{id}", api.handleGetApplicationById).Methods("GET")
	// router.HandleFunc("/application/apps/{id}", api.handleDeleteApplication).Methods("DELETE")
}

type API struct {
	am *application.Manager
}

func NewAPI(am *application.Manager) *API {
	return &API{am: am}
}

func (api *API) handleGetApplicationList(w http.ResponseWriter, r *http.Request) {
	apps, err := api.am.GetAllAppMetadata(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Success((&GetApplicationListResponse{}).FromAppMetadataArray(apps)).WriteJSON(w)
}

func (api *API) handleCreateApplication(w http.ResponseWriter, r *http.Request) {
	req := CreateApplicationRequest{}
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	appID, err := api.am.CreateAppMetadata(r.Context(), req.ToAppMetadata())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	response.Success((&CreateApplicationResponse{}).FromAppID(appID)).WriteJSON(w)
}
