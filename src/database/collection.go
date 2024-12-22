package database

import (
	"errors"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/index"
	"os"
	"path"
)

// одна коллекция в БД
type Collection struct {
	Name  string
	Path  string
	Index *index.FlatIndex
	ID    int
	Dim   int
}

func NewCollection(name string, path string, dim int) (collection *Collection, err error) {
	collection = &Collection{name, path, &index.FlatIndex{}, 0, dim}
	err = collection.Init()
	if err != nil {
		return nil, err
	}
	return collection, nil
}

func (db *Collection) FullPath() string {
	return path.Join(db.Path, db.Name)
}

func (db *Collection) Init() error {
	info_path, err_path := os.Stat(db.Path)
	if os.IsNotExist(err_path) || !info_path.IsDir() {
		return errors.New("Path doesn't exist or not dir!")
	} else {
		if !info_path.IsDir() {
			return errors.New("Path should point to the directory!")
		} else {
			full_path := path.Join(db.Path, db.Index.GetName()+".txt")

			_, err_file := os.Stat(full_path)
			if os.IsNotExist(err_file) {

				if os.IsNotExist(err_path) {
					err := os.Mkdir(db.Path, os.ModePerm)
					if err != nil {
						return err
					}
				}

				//Создаем файл для хранения коллекции
				file, err := os.Create(full_path)
				if err != nil {
					return err
				}
				defer file.Close()

			}
		}
	}
	return nil
}

func (db *Collection) Flush() (err error) {
	indexPath := path.Join(db.Path, db.Index.GetName()+".txt")
	err = db.Index.Flush(indexPath)
	return err
}

func (db *Collection) Load() (err error) {
	full_path := path.Join(db.Path, db.Index.GetName()+".txt")
	_, err_path := os.Stat(full_path)
	if os.IsNotExist(err_path) {
		return errors.New("Collection path for index doesn't exist!")
	} else {
		err = db.Index.Load(full_path)
		return err
	}
	return nil
}
