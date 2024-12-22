package index

import (
	"bufio"
	"container/heap"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/measures"
	"github.com/karpovich-alex/itmo_pirsii_2023_bdi/src/utils"
)

type SearchResult struct {
	Vector   utils.Vector
	Distance float64
}

type IndexStruct struct {
	ID       int
	VectorId int
	Result   []float64
}

func remove(s []*utils.Vector, i int) []*utils.Vector {
	s[i] = s[len(s)-1]
	return s[:len(s)-1]
}

type Index interface {
	AddVector(v *utils.Vector)
	RemoveVector(id int)
	UpdateVector(v *utils.Vector) (err error)

	Load(path string) (err error)
	Flush(path string) (err error)

	FindClosest(v *utils.Vector, measure measures.Measure, n int) (results []*SearchResult)
	FindById(id int) (v *utils.Vector, err error)
	Len() int
}

type FlatIndex struct {
	vectors []*utils.Vector
	sync.RWMutex
}

func (index *FlatIndex) GetName() string { return "FlatIndex" }

func (index *FlatIndex) GetVectors() []*utils.Vector {
	index.RLock()
	defer index.RUnlock()
	return index.vectors
}

func (index *FlatIndex) AddVector(v *utils.Vector) {
	index.Lock()
	defer index.Unlock()
	index.vectors = append(index.vectors, v)
}

func (index *FlatIndex) RemoveVector(id int) {
	index.Lock()
	defer index.Unlock()

	for idx := range index.vectors {
		if index.vectors[idx].ID == id {
			index.vectors = remove(index.vectors, idx)
			return
		}
	}
	return
}

func (index *FlatIndex) UpdateVector(v *utils.Vector) (err error) {
	index.Lock()
	defer index.Unlock()

	for idx := range index.vectors {
		if index.vectors[idx].ID == v.ID {
			index.vectors[idx] = v
			return nil
		}
	}
	return errors.New(fmt.Sprintf("Cant find vector with id = %s", v.ID))
}

func (index *FlatIndex) FindClosest(v *utils.Vector, measure measures.Measure, n int) (results []*SearchResult) {
	index.RLock()
	defer index.RUnlock()

	pq := priorityQueue{}

	for idx := range index.vectors {
		dist := measure.Calc(*v, *index.vectors[idx])
		heap.Push(&pq, &queueItem{
			value:    *index.vectors[idx],
			priority: -dist,
		})
		if pq.Len() > n {
			heap.Pop(&pq)
		}
	}
	for i := pq.Len() - 1; i >= 0; i-- {
		qi := pq[i]
		// TODO: Нужно ли делать копии каждого вектора?
		results = append(results, &SearchResult{qi.value, -qi.priority})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Distance < results[j].Distance
	})
	return results
}

func (index *FlatIndex) FindById(id int) (v *utils.Vector, err error) {
	index.RLock()
	defer index.RUnlock()

	for idx := range index.vectors {
		if index.vectors[idx].ID == id {
			v := index.vectors[idx].Copy()
			return &v, nil
		}
	}
	return nil, errors.New(fmt.Sprintf("Cant find vector with id = %s", id))
}

func (index *FlatIndex) Load(path string) (err error) {
	file, _ := os.Open(path)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		str_emb := strings.Fields(line)
		vect_id, err := strconv.Atoi(str_emb[0])
		if err != nil {
			return err
		}

		var float_emb []float64
		for _, str := range str_emb[1:] {
			if value, err := strconv.ParseFloat(str, 64); err == nil {
				float_emb = append(float_emb, value)
			} else {
				return err
			}
		}

		vect := utils.Vector{vect_id, float_emb}
		index.AddVector(&vect)
	}

	err = file.Close()
	if err != nil {
		return err
	}

	return nil
}

func (index *FlatIndex) Flush(path string) (err error) {
	index.Lock()
	defer index.Unlock()

	if _, err := os.Stat(path); err == nil {
		err := os.Remove(path)
		if err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}

	writer := bufio.NewWriter(file)

	for _, vect := range index.vectors {

		var str_vect []string
		for _, v := range vect.Embedding {
			// Convert float to string with specified format and precision
			str := strconv.FormatFloat(v, 'f', -1, 64) // 'f' for decimal point notation
			str_vect = append(str_vect, str)
		}

		line := fmt.Sprintf("%d", vect.ID) + "\t" + strings.Join(str_vect, " ")

		_, err := writer.WriteString(line + "\n") // Append newline character
		if err != nil {
			return err
		}
	}

	err = writer.Flush()
	if err != nil {
		return err
	}

	err = file.Close()
	if err != nil {
		return err
	}

	return nil
}

func (index FlatIndex) Len() int {
	index.RLock()
	defer index.RUnlock()

	return len(index.vectors)
}
