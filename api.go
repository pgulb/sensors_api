package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/couchbase/gocb/v2"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

type Measurement struct {
	Id          string  `json:"id"`
	Temperature float64 `json:"temperature"`
	Humidity    int     `json:"humidity"`
	Voltage     float64 `json:"voltage"`
	Timestamp   int     `json:"timestamp"`
}

type DbCreds struct {
	ConnectionString string `json:"connectionString"`
	BucketName       string `json:"bucketName"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Scope            string `json:"scope"`
	Collection       string `json:"collection"`
}

var coll *gocb.Collection
var cluster *gocb.Cluster

func idColons(id string) string {
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", id[0:2], id[2:4], id[4:6], id[6:8], id[8:10], id[10:12])
}

func idNoColons(id string) string {
	return strings.ReplaceAll(id, ":", "")
}

func InitDB() {
	log.Println("Loading couchbase file...")
	ex, err := os.Executable()
	exPath := filepath.Dir(ex)
	jsonPath := filepath.Join(exPath, "db.json")
	dbfile, err := os.ReadFile(jsonPath)
	var db DbCreds
	err = json.Unmarshal(dbfile, &db)
	if err != nil {
		log.Fatal(err)
	}

	options := gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: db.Username,
			Password: db.Password,
		},
	}
	if err := options.ApplyProfile(gocb.ClusterConfigProfileWanDevelopment); err != nil {
		log.Fatal(err)
	}

	log.Println("Connecting to couchbase...")
	cluster, err = gocb.Connect("couchbases://"+db.ConnectionString, options)
	if err != nil {
		log.Fatal(err)
	}

	bucket := cluster.Bucket(db.BucketName)
	err = bucket.WaitUntilReady(10*time.Second, nil)
	if err != nil {
		log.Fatal(err)
	}
	coll = bucket.Scope(db.Scope).Collection(db.Collection)
	log.Println("Connected to couchbase!")
}

func LastData(id string) (Measurement, error) {
	if strings.Contains(id, ":") {
		id = idNoColons(id)
	}
	doc, err := coll.Get(fmt.Sprintf("%s-latest", id), nil)
	if err != nil {
		return Measurement{}, err
	}
	var m Measurement
	err = doc.Content(&m)
	if err != nil {
		return Measurement{}, err
	}
	return m, nil
}

func Data(skip int) ([]Measurement, error) {
	q := fmt.Sprintf("SELECT id, temperature, humidity, voltage, timestamp FROM `sensors`.`sensors`.`sensors` ORDER BY timestamp DESC LIMIT 100 OFFSET %v", skip)
	log.Println(q)
	res, err := cluster.Query(q, nil)
	if err != nil {
		return nil, err
	}
	var m []Measurement
	for res.Next() {
		var doc Measurement
		err = res.Row(&doc)
		if err != nil {
			return nil, err
		}
		m = append(m, doc)
	}
	err = res.Err()
	if err != nil {
		return nil, err
	}
	log.Println("returning ", len(m))
	return m, nil
}
func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	InitDB()

	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)
	r.Use(middleware.Heartbeat("/healthz"))
	r.Use(middleware.Compress(5, "application/json"))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET"},
		AllowedHeaders:   []string{"Accept", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	r.Get("/latest/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := chi.URLParam(r, "id")
		m, err := LastData(id)
		if err != nil {
			log.Println(err)
			http.Error(w, "document not found", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})

	r.Get("/data/{skip}", func(w http.ResponseWriter, r *http.Request) {
		skipRaw := chi.URLParam(r, "skip")
		skip, err := strconv.Atoi(skipRaw)
		if err != nil {
			http.Error(w, "invalid skip", http.StatusBadRequest)
		}
		m, err := Data(skip)
		if err != nil {
			log.Println(err)
			http.Error(w, "documents not found", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	})

	http.ListenAndServe(":3000", r)
}
