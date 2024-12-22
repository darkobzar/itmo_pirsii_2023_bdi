package api

import (
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/database"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"

	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	dim := r.URL.Query().Get("dim")

	ctx := r.Context()
	db := ctx.Value("db").(*database.DataBase)

	dbs, err := db.Get(vars["database"])
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	dim_int, err := strconv.Atoi(dim)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = dbs.AddCollection(name, dim_int)

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


	url := fmt.Sprintf("http://localhost:8001/api/replica/1/%s/collection/%s", db_name, name)
	url2 := fmt.Sprintf("http://localhost:8002/api/replica/2/%s/collection/%s", db_name, name)

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

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
    if err != nil {
        panic(err)
    }
	req2, err := http.NewRequest("POST", url2, bytes.NewBuffer(jsonData))
    if err != nil {
        panic(err)
    }
	req.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
    resp, err := client.Do(req)
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

	resp2, err := client.Do(req2)
    if err != nil {
        panic(err)
    }
    defer resp2.Body.Close()

	w.WriteHeader(http.StatusOK)
}

func Replicate(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	db_name := vars["database"]
	name := vars["name"]
	repl_num := vars["repl_num"]

	path := "./data-r-"+repl_num+"/"+db_name+"/"+name+"/FlatIndex.txt"

	var vects []utils.Vector

    // Unmarshal the JSON data into the slice of structs
	err := json.NewDecoder(r.Body).Decode(&vects)
    if err != nil {
        http.Error(w, "Invalid JSON", http.StatusBadRequest)
        return
    }


	if _, err := os.Stat(path); err == nil {
		err := os.Remove(path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
        	return
		}
	} else if !os.IsNotExist(err) {
		http.Error(w, err.Error(), http.StatusBadRequest)
        return
	}

	file, err := os.Create(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
        return
	}

	writer := bufio.NewWriter(file)

	for _, vect := range vects {

		var str_vect []string
		for _, v := range vect.Embedding {
			// Convert float to string with specified format and precision
			str := strconv.FormatFloat(v, 'f', -1, 64) // 'f' for decimal point notation
			str_vect = append(str_vect, str)
		}

		line := fmt.Sprintf("%d", vect.ID) + "\t" + strings.Join(str_vect, " ")

		_, err := writer.WriteString(line + "\n") // Append newline character
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
        	return
		}
	}

	err = writer.Flush()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
        return
	}

	err = file.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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
