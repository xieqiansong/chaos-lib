package tools

// LinkInfo 描述一个路径的链接（符号链接 / 目录联接 / 硬链接）信息。
// 普通文件或目录也会返回，此时 LinkType 为空、IsLink 为 false。
type LinkInfo struct {
	Path       string `json:"FullName"`   // 解析后的完整路径
	LinkType   string `json:"LinkType"`   // SymbolicLink / Junction / HardLink，非链接则为空
	Target     string `json:"Target"`     // 指向目标；多目标以 ";" 拼接
	IsLink     bool   `json:"IsLink"`     // 是否为 reparse point / 符号链接
	Attributes string `json:"Attributes"` // 如 "Directory, ReparsePoint"
	Mode       string `json:"Mode"`       // PowerShell 的 Mode，如 "l----"
}
