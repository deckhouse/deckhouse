package config

type Credentials struct {
	AppID         string
	AppSecret     string
	OAuth2URL     string
	ControllerURL string
	Insecure      bool
}
