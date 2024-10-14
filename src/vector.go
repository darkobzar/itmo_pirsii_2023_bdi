package main

type Vector struct {
	ID        int
	Embedding []float64
}

type SearchResult struct {
	ID       int
	Distance float64
	Vector   Vector
}

type IndexStruct struct {
	ID     int
	Vector Vector
	Result []float64
}
