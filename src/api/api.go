package api

import (
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/database"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"

	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net"
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

	// if len(name) < 3 {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

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

func ReadReplica(w http.ResponseWriter, r *http.Request){
	_, repl_num, _ := net.SplitHostPort(r.Host)
	vars := mux.Vars(r)

	full_path := "./data-r-" + repl_num + "/" + vars["database"] + "/" + vars["name"] + "/FlatIndex.txt"
	_, err_path := os.Stat(full_path)
	if os.IsNotExist(err_path) {
		fmt.Println(full_path)
		http.Error(w,"Collection path for index doesn't exist!" , http.StatusInternalServerError)
	} 

	file, _ := os.Open(full_path)
	scanner := bufio.NewScanner(file)

	var vectors []utils.Vector

	for scanner.Scan() {
		line := scanner.Text()
		fmt.Println(line)
		str_emb := strings.Fields(line)
		vect_id, err := strconv.Atoi(str_emb[0])
		if err != nil {
			http.Error(w, err.Error() , http.StatusInternalServerError)
		}

		var float_emb []float64
		for _, str := range str_emb[1:] {
			if value, err := strconv.ParseFloat(str, 64); err == nil {
				float_emb = append(float_emb, value)
			} else {
				http.Error(w, err.Error() , http.StatusInternalServerError)
			}
		}

		vect := utils.Vector{vect_id, float_emb}
		vectors = append(vectors, vect)
	}

	err := file.Close()
	if err != nil {
		http.Error(w, err.Error() , http.StatusInternalServerError)
	}

	return
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

	for _, v := range dbs.LoadedCollections[vars["name"]].Index.GetVectors(){
		var stringValues []string
		for _, value := range v.Embedding {
			
			// Convert float to string with desired format
			strValue := strconv.FormatFloat(value, 'f', -1, 64) // 'f' for decimal point format
			stringValues = append(stringValues, strValue)
		}
	
		// Step 4: Join the string slice into one string
		result := strings.Join(stringValues, ", ")
		fmt.Println(v.ID, ": ", result)
	}

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

	hosts := readHostList()
	for _, h := range hosts {

		url := fmt.Sprintf("http://localhost:%s/api/replica/%s/collection/%s", h, db_name, name)
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
	}

	w.WriteHeader(http.StatusOK)
}

func readHostList() []string {
	file, err := os.Open("replicas_list.txt")

	if err != nil {
		fmt.Println(err)
		return nil
	}
	defer file.Close() // Ensure the file is closed at the end

	// Create a scanner to read the file line by line
	scanner := bufio.NewScanner(file)

	// Read and print each line
	var hosts []string

	// Read each line and append it to the slice
	for scanner.Scan() {
		hosts = append(hosts, scanner.Text())
	}

	return hosts
}

func Replicate(w http.ResponseWriter, r *http.Request) {
	_, repl_num, _ := net.SplitHostPort(r.Host)

	vars := mux.Vars(r)
	db_name := vars["database"]
	name := vars["name"]

	path := "./data-r-" + repl_num + "/" + db_name + "/" + name + "/FlatIndex.txt"
	os.MkdirAll(path, 0755)

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
