package tools

type LinkInfo struct {
	Path       string `json:"FullName"`
	LinkType   string `json:"LinkType"`
	Target     string `json:"Target"`
	IsLink     bool   `json:"IsLink"`
	Attributes string `json:"Attributes"`
	Mode       string `json:"Mode"`
}
