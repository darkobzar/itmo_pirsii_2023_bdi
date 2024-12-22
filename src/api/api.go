package api

import (
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/database"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"

	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

var dbs *database.DataBaseStruct

type VectorRequest struct {
	ID     int       `json:"id"`
	Vector []float64 `json:"vector"`
}

func CreateOrGetDB(w http.ResponseWriter, r *http.Request) {

	var err error

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	name := r.URL.Query().Get("name")

	dbs, err = db.NewDataBase(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(dbs)
}

func CreateCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	name := r.URL.Query().Get("name")

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(vars["database"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = dbs.AddCollection(name)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func DeleteCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(vars["database"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = dbs.RemoveCollection(vars["name"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func LoadCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(vars["database"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = dbs.Load(vars["name"])

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func FlushCollection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	db_name := vars["database"]
	name := vars["name"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(db_name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	err = dbs.Flush(name)

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusOK)

}

func AddVector(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dbName := vars["database"]
	name := vars["name"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(dbName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	var vectorRequest VectorRequest

	err = json.NewDecoder(r.Body).Decode(&vectorRequest)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	v := &utils.Vector{ID: vectorRequest.ID, Embedding: vectorRequest.Vector}

	err = dbs.AddVector(name, v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func RemoveVector(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dbName := vars["database"]
	name := vars["name"]
	id_str := vars["id"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(dbName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		http.Error(w, "Invalid vector id", http.StatusBadRequest)
		return
	}

	err = dbs.RemoveVector(name, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)

}

func GetVector(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dbName := vars["database"]
	name := vars["name"]
	id_str := vars["id"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(dbName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	id, err := strconv.Atoi(id_str)
	if err != nil {
		http.Error(w, "Invalid vector id", http.StatusBadRequest)
		return
	}

	v, err := dbs.FindById(name, id)

	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	json.NewEncoder(w).Encode(v)

}

func UpdateVector(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	db_name := vars["database"]
	name := vars["name"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(db_name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var vectorRequest VectorRequest

	err = json.NewDecoder(r.Body).Decode(&vectorRequest)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	v := &utils.Vector{ID: vectorRequest.ID, Embedding: vectorRequest.Vector}

	err = dbs.UpdateVector(name, v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}

func GetClosest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	dbName := vars["database"]
	name := vars["name"]

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(dbName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//measure_str := vars["measure"]

	n_str := r.URL.Query().Get("n")

	n, err := strconv.Atoi(n_str)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var vectorRequest VectorRequest

	err = json.NewDecoder(r.Body).Decode(&vectorRequest)
	if err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	vector := &utils.Vector{ID: vectorRequest.ID, Embedding: vectorRequest.Vector}

	//TODO for cosine
	results, err := dbs.FindClosest(name, vector, measures.EuclideanDistanceMeasure{}, n)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(results)

}
