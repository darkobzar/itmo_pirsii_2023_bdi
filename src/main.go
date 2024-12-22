package main

import (
	"context"
	"encoding/json"
	"flag"
	"github.com/gorilla/mux"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/api"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/database"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"
)

const (
	host = "0.0.0.0"
	port = "8000"
	path = "./data"
	port_r = "8001"
	port_r2 = "8002"
)

func WrapContext(next http.Handler, db *database.DataBase) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, "-", r.RequestURI)
		ctx := context.WithValue(r.Context(), "db", db)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func main() {

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()

	router := mux.NewRouter()
	router_r := mux.NewRouter()
	router_r2 := mux.NewRouter()

	db := database.DataBase{Path: path}

	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	router_r.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	router_r2.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	router.HandleFunc("/api/database", api.CreateOrGetDB).Methods("POST")

	router.HandleFunc("/api/database/{database}/collection", api.CreateCollection).Methods("POST")
	router.HandleFunc("/api/database/{database}/collection/{name}", api.LoadCollection).Methods("GET")
	router.HandleFunc("/api/database/{database}/collection/{name}", api.FlushCollection).Methods("PUT")
	router.HandleFunc("/api/database/{database}/collection/{name}", api.DeleteCollection).Methods("DELETE")

	router.HandleFunc("/api/database/{database}/collection/{name}/vector", api.AddVector).Methods("POST")
	router.HandleFunc("/api/database/{database}/collection/{name}/vector/{id}", api.GetVector).Methods("GET")
	router.HandleFunc("/api/database/{database}/collection/{name}/vector/{id}", api.UpdateVector).Methods("PUT")
	router.HandleFunc("/api/database/{database}/collection/{name}/vector/{id}", api.RemoveVector).Methods("DELETE")
	router.HandleFunc("/api/database/{database}/collection/{name}/find", api.GetClosest).Methods("POST")


	//replication
	router_r.HandleFunc("/api/replica/{repl_num}/{database}/collection/{name}", api.Replicate).Methods("POST")
	router_r2.HandleFunc("/api/replica/{repl_num}/{database}/collection/{name}", api.Replicate).Methods("POST")

	srv := &http.Server{
		Addr: host + ":" + port,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(router, &db), // Pass our instance of gorilla/mux in.
	}
	srv_r := &http.Server{
		Addr: host + ":" + port_r,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(router_r, &db), // Pass our instance of gorilla/mux in.
	}
	srv_r2 := &http.Server{
		Addr: host + ":" + port_r2,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(router_r2, &db), // Pass our instance of gorilla/mux in.
	}

	// Run our server in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := srv_r.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := srv_r2.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()




	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	// Block until we receive our signal.
	<-c

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	os.Exit(0)
}
