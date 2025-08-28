package types

var Version string

type Request struct {
	URL          string `json:"url"`
	DownloadPath string `json:"downloadPath"`
	ProxyUrl     string `json:"proxyUrl"`
}
