package provider

type Provider interface {
	ValidateCredentials(string, string) ([]string, error)
}
