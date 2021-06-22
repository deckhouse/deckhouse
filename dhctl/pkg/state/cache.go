package state

const TombstoneKey = ".tombstone"

type Cache interface {
	Save(string, []byte) error
	SaveStruct(string, interface{}) error

	Load(string) []byte
	LoadStruct(string, interface{}) error

	Delete(string)
	Clean()

	GetPath(string) string
	Iterate(func(string, []byte) error) error
	InCache(string) bool
}
