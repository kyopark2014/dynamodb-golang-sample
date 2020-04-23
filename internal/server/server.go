package server

import (
	"dynamodb-golang-sample/internal/config"
	"dynamodb-golang-sample/internal/db"
	"dynamodb-golang-sample/internal/log"
	"net/http"

	"github.com/gorilla/mux"
)

// LiveCheck is the api to check the pod is alive
func LiveCheck(w http.ResponseWriter, r *http.Request) {
	log.D("Live Check ...")
}

// InitServer initializes the REST api server
func InitServer(conf *config.AppConfig) error {
	// Initiate the SQL database
	dberror := db.NewDatabase(conf.Dynamo)
	if dberror != nil {
		log.D("Faile to open dynamodb: %v", dberror.Error())
	}

	// Init Router
	r := mux.NewRouter()

	// Route Handler / Endpoints
	r.HandleFunc("/", LiveCheck).Methods("GET")

	var muxerr error
	muxerr = http.ListenAndServe(":8080", r)

	return muxerr
}
