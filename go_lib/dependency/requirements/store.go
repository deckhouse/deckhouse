package requirements

type MemoryValuesStore struct {
	values map[string]interface{}
}

func newMemoryValuesStore() *MemoryValuesStore {
	return &MemoryValuesStore{
		values: make(map[string]interface{}),
	}
}

func (m *MemoryValuesStore) Set(key string, value interface{}) {
	m.values[key] = value
}

func (m *MemoryValuesStore) Remove(key string) {
	delete(m.values, key)
}

func (m *MemoryValuesStore) Get(key string) (interface{}, bool) {
	v, ok := m.values[key]
	return v, ok
}
