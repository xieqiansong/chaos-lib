package proxy

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	linkName    = "current"
	versionFile = ".current-version"
)

var sdkRoots = map[string]string{
	"jdk":    `D:\opt\jdk`,
	"maven":  `D:\opt\maven`,
	"python": `D:\opt\python`,
	"llama":  `D:\opt\llama`,
}

type SdkInfo struct {
	CurrentVersion string   ``
	VersionList    []string ``
}

func getSdkInfo(root string) SdkInfo {
	var info SdkInfo
	var currentVersion string
	verFilePath := filepath.Join(root, versionFile)
	if data, err := os.ReadFile(verFilePath); err == nil {
		currentVersion = strings.TrimSpace(string(data))
	}
	info.CurrentVersion = currentVersion
	entries, err := os.ReadDir(root)
	if err != nil {
		return info
	}
	var list []string
	for _, e := range entries {
		name := e.Name()
		if name == linkName || name == versionFile {
			continue
		}
		if e.IsDir() {
			list = append(list, name)
		}
	}
	info.VersionList = list
	return info
}

func GetSdkVersions(c *gin.Context) {
	result := make(map[string]SdkInfo)
	for typ, root := range sdkRoots {
		result[typ] = getSdkInfo(root)
	}
	c.JSON(200, result)
}

func GetSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	root := sdkRoots[typ]
	if root == "" {
		c.JSON(404, gin.H{"error": "SDK type not found"})
		return
	}
	info := getSdkInfo(root)
	c.JSON(200, info)
}

func UpdateSdkVersion(c *gin.Context) {
	typ := c.Param("type")
	root := sdkRoots[typ]
	var req struct {
		Version string ``
	}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON body"})
		return
	}
	target := filepath.Join(root, req.Version)
	link := filepath.Join(root, linkName)
	verFile := filepath.Join(root, versionFile)
	if _, err := os.Stat(target); os.IsNotExist(err) {
		c.JSON(400, gin.H{"error": "Target version does not exist: " + req.Version})
		return
	}
	if _, err := os.Lstat(link); err == nil {
		os.Remove(link)
	}
	err := os.Symlink(target, link)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to create symlink: " + err.Error() + ". (Maybe need admin rights?)"})
		return
	}
	err = os.WriteFile(verFile, []byte(req.Version), 0644)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to write version file: " + err.Error()})
		return
	}
	c.JSON(200, gin.H{"message": "Successfully switched to " + req.Version, "version": req.Version})
}
