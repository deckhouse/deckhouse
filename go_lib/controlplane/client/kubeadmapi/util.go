package kubeadmapi

import v1 "k8s.io/api/core/v1"

// APIEndpoint struct contains elements of API server instance deployed on a node.
type APIEndpoint struct {
	// AdvertiseAddress sets the IP address for the API server to advertise.
	AdvertiseAddress string

	// BindPort sets the secure port for the API Server to bind to.
	// Defaults to 6443.
	BindPort int32
}

type EnvVar struct {
	v1.EnvVar
}

// Arg represents an argument with a name and a value.
type Arg struct {
	Name  string
	Value string
}

// GetArgValue traverses an argument slice backwards and returns the value
// of the given argument name and the index where it was found.
// If the argument does not exist an empty string and -1 are returned.
// startIdx defines where the iteration starts. If startIdx is a negative
// value or larger than the size of the argument slice the iteration
// will start from the last element.
func GetArgValue(args []Arg, name string, startIdx int) (string, int) {
	if startIdx < 0 || startIdx > len(args)-1 {
		startIdx = len(args) - 1
	}
	for i := startIdx; i >= 0; i-- {
		arg := args[i]
		if arg.Name == name {
			return arg.Value, i
		}
	}
	return "", -1
}

// SetArgValues updates the value of one or more arguments or adds a new
// one if missing. The function works backwards in the argument list.
// nArgs holds how many existing arguments with this name should be set.
// If nArgs is less than 1, all of them will be updated.
func SetArgValues(args []Arg, name, value string, nArgs int) []Arg {
	var count int
	var found bool
	for i := len(args) - 1; i >= 0; i-- {
		if args[i].Name == name {
			found = true
			args[i].Value = value
			if nArgs < 1 {
				continue
			}
			count++
			if count >= nArgs {
				return args
			}
		}
	}
	if found {
		return args
	}
	args = append(args, Arg{Name: name, Value: value})
	return args
}
