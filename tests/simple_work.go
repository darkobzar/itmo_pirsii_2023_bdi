package tests

import (
	"fmt"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/database"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"
)

func main() {
	v1 := &utils.Vector{1, []float64{0, 0, 0, 0}}
	v2 := &utils.Vector{2, []float64{0, 1, 1, 0}}
	v3 := &utils.Vector{3, []float64{0, 0, 0, 1}}
	v4 := &utils.Vector{4, []float64{0, 0, 0, 1}}
	vf := &utils.Vector{0, []float64{0, 0, 0, 1}}

	// Создаем или читаем БД
	dbs, err := database.NewDataBase("test_db", "./data")
	if err != nil {
		panic(err)
	}
	fmt.Println(dbs)

	// Добавляем новую коллекцию
	err = dbs.AddCollection("test_col")
	if err != nil {
		panic(err)
	}
	// Удаляем коллекцию
	err = dbs.RemoveCollection("test_col")
	if err != nil {
		panic(err)
	}

	colName := "test_col_1"
	// Добавляем новую коллекцию
	err = dbs.AddCollection(colName)
	if err != nil {
		panic(err)
	}
	err = dbs.Load(colName)
	if err != nil {
		panic(err)
	}
	// Добавляем в коллекцию вектора
	err = dbs.AddVector(colName, v1)
	err = dbs.AddVector(colName, v2)
	err = dbs.AddVector(colName, v3)
	err = dbs.AddVector(colName, v4)
	if err != nil {
		panic(err)
	}

	// Сохраняем на диск
	err = dbs.Flush(colName)
	if err != nil {
		panic(err)
	}
	// Поиск ближайщих
	results, err := dbs.FindClosest(colName, vf, measures.EuclideanDistanceMeasure{}, 3)
	if err != nil {
		panic(err)
	}
	fmt.Println(results)

	//Поиск по ID
	foundVector, err := dbs.FindById(colName, 1)
	if err != nil {
		panic(err)
	}
	fmt.Println(foundVector)

	//Удаление вектора
	err = dbs.RemoveVector(colName, 2)
	if err != nil {
		return
	}
	// Отчищаем коллекцию и зашружаем заново
	err = dbs.Flush(colName)
	if err != nil {
		panic(err)
	}

	err = dbs.Load(colName)
	if err != nil {
		panic(err)
	}

	results, err = dbs.FindClosest(colName, vf, measures.EuclideanDistanceMeasure{}, 3)
	if err != nil {
		panic(err)
	}
	fmt.Println(results)

	// Обновление вектора
	//v0 := utils.Vector{1, []float64{5, 5, 5, 5}}
	//db.SetVector(2, v0)
	//fmt.Println(db)

}
