package entity

type SEP struct {
	ID        uint64
	Name      string
	IsActive  bool
	IsCreated bool
	Pools     []Pool
}

type Pool struct {
	Name string
}
