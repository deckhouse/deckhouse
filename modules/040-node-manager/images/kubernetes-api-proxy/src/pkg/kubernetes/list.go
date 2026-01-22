package kubernetes

type ListPicker interface {
	PickAsString() (string, error)
}
