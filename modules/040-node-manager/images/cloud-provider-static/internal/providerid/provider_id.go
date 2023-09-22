package providerid

import (
	"fmt"
	"github.com/pkg/errors"
	"regexp"
	"sigs.k8s.io/cluster-api/util"
)

const (
	// Prefix is the prefix for a static node provider ID.
	Prefix = "static://"
)

type ProviderID string

// GenerateProviderID generates a provider ID for a static node.
func GenerateProviderID() ProviderID {
	return ProviderID(fmt.Sprintf("%s/%s", Prefix, util.RandomString(16)))
}

// ValidateProviderID validates a provider ID for a static node.
func ValidateProviderID(providerID ProviderID) error {
	match, err := regexp.MatchString(fmt.Sprintf("%s/.+", Prefix), string(providerID))
	if err != nil {
		return err
	}
	if match {
		return nil
	}

	return errors.New("invalid format for provider id")
}
