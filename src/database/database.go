package database

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"strconv"

	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/index"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"
)

// интерфейс коллекции
type IDataBase interface {
	Init() (err error)

	Load(collectionName string) (err error)
	Flush(collectionName string) (err error)

	AddVector(collectionName string, v *utils.Vector) error
	RemoveVector(collectionName string, id int) error
	//UpdateVector(collectionName string, v *utils.Vector) (err error)

	AddCollection(collectionName string) (err error)
	RemoveCollection(collectionName string) (err error)

	FindClosest(collectionName string, v *utils.Vector, measure measures.Measure, n int) (results []*index.SearchResult, err error)
	FindById(collectionName string, id int) (v *utils.Vector, err error)
}

type DataBase struct {
	Path       string
	structures map[string]*DataBaseStruct
	m          sync.RWMutex
}

func (db *DataBase) NewDataBase(name string) (dataBase *DataBaseStruct, err error) {
	db.m.Lock()
	defer db.m.Unlock()

	dataBase = &DataBaseStruct{name: name, Path: db.Path, Collections: make(map[string]*CollectionInfo), LoadedCollections: make(map[string]*Collection), ID_count: 0}
	err = dataBase.Init()
	if err != nil {
		return nil, err
	}
	if db.structures == nil {
		db.structures = map[string]*DataBaseStruct{}
	}
	db.structures[name] = dataBase
	return dataBase, nil
}

func (db *DataBase) Get(name string) (dbs *DataBaseStruct, err error) {
	db.m.RLock()
	defer db.m.RUnlock()

	dbs, ok := db.structures[name]
	if !ok {
		return nil, errors.New(fmt.Sprintf("Structure %s doesnt exist", name))
	}
	return dbs, nil
}

func NewDataBase(name string, path string) (dataBase *DataBaseStruct, err error) {
	dataBase = &DataBaseStruct{name: name, Path: path, Collections: make(map[string]*CollectionInfo), LoadedCollections: make(map[string]*Collection), ID_count: 0, cm: sync.RWMutex{}, lm: sync.RWMutex{}}
	err = dataBase.Init()
	if err != nil {
		return nil, err
	}
	return dataBase, nil
}

type CollectionInfo struct {
    Path string
    Dim  int
}

// собственно сама БД
type DataBaseStruct struct {
	name              string
	Path              string
	Collections       map[string]*CollectionInfo 
	LoadedCollections map[string]*Collection
	ID_count          int

	cm sync.RWMutex
	lm sync.RWMutex
}

// Инициализируем БД
// Для хранения БД испольуется файл с именами коллекций и путями к ним
func (dbs *DataBaseStruct) Init() (err error) {
	filePath := path.Join(dbs.Path, dbs.name+".txt")
	dirPath := path.Join(dbs.Path, dbs.name)

	_, err_path := os.Stat(filePath)
	if os.IsNotExist(err_path) {
		// Если файла нет, то создаем
		file, err := os.Create(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err_path := os.Stat(dirPath)
		if os.IsNotExist(err_path) {
			// Если папки нет, то создаем
			err = os.Mkdir(dirPath, os.ModePerm)
			if err != nil {
				return err
			}
		}
		dbs.ID_count = 0
	} else {
		// Если папка есть, то читаем
		file, _ := os.Open(filePath)
		defer file.Close()

		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			str_emb := strings.Fields(line)
			name_collection := str_emb[0]
			path_collection := str_emb[1]
			dim_collection := str_emb[2]

			dim, err := strconv.Atoi(dim_collection)
			if err != nil {
				fmt.Println("Error converting string to int:", err)
			}

			dbs.Collections[name_collection] = &CollectionInfo{Path: path_collection, Dim: dim}
		}
		dbs.ID_count = len(dbs.Collections)
	}
	return nil
}

func (dbs *DataBaseStruct) AddCollection(collectionName string, dim int) (err error) {
	dbs.cm.Lock()
	defer dbs.cm.Unlock()

	fullPath := path.Join(dbs.Path, dbs.name, collectionName)
	_, err_path := os.Stat(fullPath)
	if os.IsNotExist(err_path) {
		err = os.Mkdir(fullPath, os.ModePerm)
		if err != nil {
			return err
		}
		file, err := os.Create(path.Join(fullPath, "FlatIndex.txt"))
		if err != nil {
			return err
		}
		defer file.Close()
	} else {
		return errors.New("Collection already exists")
	}

	//Записываем в файл БД название коллекции и путь к ней
	dbFile, err := os.OpenFile(path.Join(dbs.Path, dbs.name+".txt"), os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer dbFile.Close()
	_, err = dbFile.WriteString(collectionName + " " + fullPath + " " + strconv.Itoa(dim) + "\n")
	if err != nil {
		return err
	}

	dbs.Collections[collectionName] = &CollectionInfo{Path: fullPath, Dim: dim}
	dbs.ID_count += 1
	return nil
}

// удаляем коллекцию из БД
func (dbs *DataBaseStruct) RemoveCollection(collectionName string) (err error) {
	dbs.cm.Lock()
	defer dbs.cm.Unlock()

	collection, ok := dbs.Collections[collectionName]
	if !ok {
		return nil
	}

	e := os.RemoveAll(collection.Path)
	if e != nil {
		return e
	}
	delete(dbs.Collections, collectionName)

	file, err := os.OpenFile(path.Join(dbs.Path, dbs.name+".txt"), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	for name, collection := range dbs.Collections {
		_, err = file.WriteString(name + " " + collection.Path + " " + strconv.Itoa(collection.Dim) + "\n")
		if err != nil {
			return err
		}
	}

	return e
}

func (dbs *DataBaseStruct) Load(collectionName string) (err error) {
	dbs.cm.Lock()
	dbs.lm.Lock()
	defer dbs.cm.Unlock()
	defer dbs.lm.Unlock()

	collection, ok := dbs.LoadedCollections[collectionName]
	if ok {
		// Коллекция уже загружена
		return nil
	}
	collection_info, ok := dbs.Collections[collectionName]
	if !ok {
		return errors.New(fmt.Sprintf("Cant find collection %s", collectionName))
	}
	collection, err = NewCollection(collectionName, collection_info.Path, collection_info.Dim)
	if err != nil {
		return err
	}
	err = collection.Load()
	if err != nil {
		return err
	}
	dbs.LoadedCollections[collectionName] = collection
	return nil
}

func (dbs *DataBaseStruct) Flush(collectionName string) (err error, vects []*utils.Vector) {
	// dbs.lm.Lock()
	// defer dbs.lm.Unlock()

	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		return err, nil
	}
	err = collection.Flush()
	vects = collection.Index.GetVectors()
	delete(dbs.LoadedCollections, collectionName)
	return err, vects
}

func (dbs *DataBaseStruct) AddVector(collectionName string, v *utils.Vector) error {
	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		return err
	}
	if (v.Len() !=collection.Dim){
		return fmt.Errorf("Dimention of the vector = %d doesnt match dimention of the collection = %d", v.Len(), collection.Dim)
	}

	collection.Index.AddVector(v)
	return nil
}

func (dbs *DataBaseStruct) UpdateVector(collectionName string, v *utils.Vector) error {
	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		return err
	}
	err = collection.Index.UpdateVector(v)
	return nil
}

func (dbs *DataBaseStruct) RemoveVector(collectionName string, id int) error {
	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		return err
	}

	collection.Index.RemoveVector(id)
	return nil
}

func (dbs *DataBaseStruct) getLoadedCollection(collectionName string) (collection *Collection, err error) {
	dbs.lm.RLock()
	defer dbs.lm.RUnlock()

	collection, ok := dbs.LoadedCollections[collectionName]
	if !ok {
		// Коллекция уже загружена
		return nil, errors.New(fmt.Sprintf("Collection %s doesnt load", collectionName))
	}
	return collection, nil
}

func (dbs *DataBaseStruct) FindById(collectionName string, id int) (v *utils.Vector, err error) {
	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		// Коллекция уже загружена
		return nil, errors.New(fmt.Sprintf("Collection %s doesnt load", collectionName))
	}
	return collection.Index.FindById(id)
}

func (dbs *DataBaseStruct) FindClosest(collectionName string, v *utils.Vector, measure measures.Measure, n int) (results []*index.SearchResult, err error) {
	collection, err := dbs.getLoadedCollection(collectionName)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Collection %s doesnt load", collectionName))
	}
	return collection.Index.FindClosest(v, measure, n), nil
}
