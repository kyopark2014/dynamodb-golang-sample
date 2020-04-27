package server

import (
	"dynamodb-golang-sample/internal/config"
	"dynamodb-golang-sample/internal/data"
	"dynamodb-golang-sample/internal/dynamo"
	"dynamodb-golang-sample/internal/log"
	"dynamodb-golang-sample/internal/logger"
	"dynamodb-golang-sample/internal/rediscache"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
)

// ProfileService is a list of service
type ProfileService struct {
	log *logger.Logger
}

// Init is to start Profile Service
func (p *ProfileService) Init(conf *config.AppConfig) error {
	p.log = logger.NewLogger("ProfileServer")

	// initialize radis for in-memory cache
	rediscache.NewRedisCache(conf.Redis)

	// Initiate the dynamo database
	error := dynamo.NewDatabase(conf.Dynamo)
	if error != nil {
		log.D("Faile to open dynamodb: %v", error.Error())
		return error
	}

	return nil
}

// Start is to run Profile Server
func (p *ProfileService) Start() error {
	log.I("start Profile Server...")

	// Init Router
	r := mux.NewRouter()

	// Route Handler / Endpoints
	r.HandleFunc("/add", Insert).Methods("POST")
	r.HandleFunc("/search/{key}", Retrieve).Methods("GET")
	r.HandleFunc("/", LiveCheck).Methods("GET")

	var err error
	err = http.ListenAndServe(":8080", r)

	return err
}

// OnTerminate is to close the servcie
func (p *ProfileService) OnTerminate() error {
	log.I("Profile Server was terminated")

	// To-Do: add codes for error cases if requires
	return nil
}

// Insert is the api to append an Item
func Insert(w http.ResponseWriter, r *http.Request) {
	// parse the data
	var item data.UserProfile
	_ = json.NewDecoder(r.Body).Decode(&item)
	log.D("item: %+v", item)

	if err := dynamo.Write(item); err != nil {
		log.E("Got error calling PutItem: %v", err.Error())

		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// put the item into rediscache
	key := item.UID // UID to identify the profile
	_, rErr := rediscache.SetCache(key, &item)
	if rErr != nil {
		log.E("Error of setCache: %v", rErr)
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	log.D("Successfully inserted in redis cache")

	w.WriteHeader(http.StatusOK)
}

// Retrieve is the api to search an Item
func Retrieve(w http.ResponseWriter, r *http.Request) {
	uid := strings.Split(r.URL.Path, "/")[2]
	log.D("Looking for uid: %v ...", uid)

	// search in redis cache
	cache, err := rediscache.GetCache(uid)
	if err != nil {
		log.E("Error: %v", err)
	}
	if cache != nil {
		log.D("value from cache: %+v", cache)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cache)
	} else {
		log.D("No data in redis cache then search it in database.")

		// search in database
		item, err := dynamo.Read(uid)
		if err != nil {
			log.D("Fail to read: %v", err.Error())
			w.WriteHeader(http.StatusNotFound)
			return
		}

		log.D("%v", item)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(item)
	}
}

// LiveCheck is the api to check the pod is alive
func LiveCheck(w http.ResponseWriter, r *http.Request) {
	log.D("Live Check ...")
}
