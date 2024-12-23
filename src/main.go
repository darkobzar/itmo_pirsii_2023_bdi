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
	"fmt"
	"sync"
	"bytes"
)

var (
    mu     sync.Mutex
	hostList HostList
)


const (
	host = "0.0.0.0"
	port = "8000"
	path = "./data"
	port2 = "8001"
	port3 = "8002"
	port4 = "8003"
)

func WrapContext(next http.Handler, db *database.DataBase) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.Method, "-", r.RequestURI)
		ctx := context.WithValue(r.Context(), "db", db)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

type Host struct{
	ID int
	Port string
	Server *http.Server
}

type HostList struct {
	Master  *Host
	Slaves  []*Host
	Unused []*Host
}

var db = database.DataBase{Path: path}
var masterRouter *mux.Router
var slaveRouter *mux.Router

func main() {

	var wait time.Duration
	flag.DurationVar(&wait, "graceful-timeout", time.Second*15, "the duration for which the server gracefully wait for existing connections to finish - e.g. 15s or 1m")
	flag.Parse()


	masterRouter = createMasterRouter()
	slaveRouter = createSlaveRouter()
	unusedRouter := createUnusedRouter()

	srv := &http.Server{
		Addr: host + ":" + port,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(masterRouter, &db), // Pass our instance of gorilla/mux in.
	}
	srv2 := &http.Server{
		Addr: host + ":" + port2,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(slaveRouter, &db), // Pass our instance of gorilla/mux in.
	}
	srv3 := &http.Server{
		Addr: host + ":" + port3,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(slaveRouter, &db), // Pass our instance of gorilla/mux in.
	}
	srv4 := &http.Server{
		Addr: host + ":" + port4,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: time.Second * 15,
		ReadTimeout:  time.Second * 15,
		IdleTimeout:  time.Second * 60,
		Handler:      WrapContext(unusedRouter, &db), // Pass our instance of gorilla/mux in.
	}


	host := Host{1, port, srv}
	host2 := Host{2, port2, srv2}
	host3 := Host{3, port3, srv3}
	host4 := Host{4, port4, srv4}

	var slaves []*Host
	var unused []*Host

	slaves = append(slaves, &host2)
	slaves = append(slaves, &host3)
	unused = append(unused, &host4)

	hostList = HostList{&host, slaves, unused}
	updateHostList()




	//Run our servers in a goroutine so that it doesn't block.
	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := srv2.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := srv3.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()
	go func() {
		if err := srv4.ListenAndServe(); err != nil {
			log.Println(err)
		}
	}()

	ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

	
	go monitorMaster(30*time.Second)

	time.Sleep(40 * time.Second) // Let the server run for 10 seconds
    stopServer(srv)                 // Simulate master failure

    //Keep the main function alive for demonstration purposes
    //select {}


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
	srv2.Shutdown(ctx)
	srv3.Shutdown(ctx)
	srv4.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	log.Println("shutting down")
	
	// Останавливаем внутренние процессы БД
	db.Stop()

	os.Exit(0)
}

func createMasterRouter() *mux.Router {

	router := mux.NewRouter()
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
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

	router.HandleFunc("/api/replica/init/{database}/collection/{name}", InitReplica).Methods("POST")

	

	return router

}

func InitReplica(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	db_name := vars["database"]
	name := vars["name"]

	minId := 100
	minUn := 100

	for index, sl := range hostList.Unused{
		if sl.ID < minUn{
			minUn = sl.ID
			minId = index
		}
	}
	if minId == 100 {
		http.Error(w,"There is no available hosts", http.StatusInternalServerError)
		fmt.Println("There is no available hosts")
		return
	}
	newSlaveHost := hostList.Unused[minId]
	newSlaveUrl := "http://localhost:" + newSlaveHost.Port + "/api/health"
	isWorking := checkHostStatus(newSlaveUrl)


	if isWorking{
		newSlaveHost.Server.Handler = WrapContext(slaveRouter, &db)
		hostList.Slaves = append(hostList.Slaves, newSlaveHost)
		hostList.Unused = append(hostList.Unused[:minId], hostList.Unused[minId+1:]...)
	} else{
		http.Error(w,"There is no available hosts", http.StatusInternalServerError)
		fmt.Println("There is no available hosts")
		return
	}

	fmt.Println("New replica is running on port: ", newSlaveHost.Port)

	dbs, err := db.Get(db_name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
	}



	err, vects := dbs.Flush(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	jsonData, err := json.Marshal(vects)
    if err != nil {
        fmt.Println("Error marshalling to JSON:", err)
        return
    }

	url := fmt.Sprintf("http://localhost:%s/api/replica/%s/collection/%s", newSlaveHost.Port, db_name, name)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	
	updateHostList()
}

func updateHostList(){
	file, _ := os.Create("replicas_list.txt")
	defer file.Close()

	for _, sl := range hostList.Slaves{
		_, err := file.WriteString(sl.Port + "\n")
		if err != nil {
            fmt.Println("Error writing to file:", err)
            return
        }
	}

}




func createSlaveRouter() *mux.Router{
	router := mux.NewRouter()
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	router.HandleFunc("/api/database", api.CreateOrGetDB).Methods("POST")
	router.HandleFunc("/api/database", api.CreateOrGetDB).Methods("POST")
	router.HandleFunc("/api/database/{database}/collection/{name}", api.LoadCollection).Methods("GET")
	router.HandleFunc("/api/replica/{database}/collection/{name}", api.Replicate).Methods("POST")

	return router
}



func createUnusedRouter() *mux.Router{
	router := mux.NewRouter()
	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	return router
}


func setNewMaster() {
	minId := 100
	minSl := 100

	
	for index, sl := range hostList.Slaves{
		if sl.ID < minSl{
			masterHost := hostList.Slaves[index]
			masterURL := "http://localhost:" + masterHost.Port + "/api/health"
			isWorking := checkHostStatus(masterURL)
			if isWorking{
				minSl = sl.ID
				minId = index
			}
		}
	}

	if minId == 100{
		fmt.Println("There is no available hosts")
		return
	}

	masterHost := hostList.Slaves[minId]
	hostList.Slaves = append(hostList.Slaves[:minId], hostList.Slaves[minId+1:]...)
	masterHost.Server.Handler = WrapContext(masterRouter, &db)
	hostList.Master = masterHost


	fmt.Println("New master is running on port: ", masterHost.Port)
	updateHostList()

}

func checkHostStatus(url string) bool{
    resp, err := http.Get(url)
    if err != nil {
        return false
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusOK {
        return true
    } else {
        return false
    }
}

func checkMasterStatus(url string) {
    resp, err := http.Get(url)
    if err != nil {
        fmt.Println("Error contacting master:", err)
		setNewMaster()
        return
    }
    defer resp.Body.Close()

    if resp.StatusCode == http.StatusOK {
        fmt.Println("Master is up and running.")
    } else {
        fmt.Printf("Master is down. Status code: %d\n", resp.StatusCode)
		setNewMaster()
    }
}

func monitorMaster(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
			url := "http://localhost:" + hostList.Master.Port + "/api/health"
            checkMasterStatus(url)
        }
    }
}

func stopServer(server *http.Server) {
    mu.Lock()
    defer mu.Unlock()
    
    if server != nil {
        fmt.Println("Stopping master server...")
        if err := server.Close(); err != nil {
            fmt.Println("Error stopping server:", err)
        }
    }
}
