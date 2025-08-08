package common

func ToPgetArgs(url string, req Request) []string {
	var ags []string
	ags = append(ags, "-x")
	if req.ProxyUrl != "" {
		ags = append(ags, req.ProxyUrl)
	} else {
		ags = append(ags, "http://127.0.0.1:7897")
	}
	ags = append(ags, "-p")
	ags = append(ags, "4")
	if req.DownloadPath != "" {
		ags = append(ags, "-o")
		ags = append(ags, req.DownloadPath)
	}
	ags = append(ags, url)
	return ags
}
