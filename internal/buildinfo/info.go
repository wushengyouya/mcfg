package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	// Version 表示构建时注入的版本号。
	Version = "dev"
	// Commit 表示构建时注入的提交哈希。
	Commit = "unknown"
	// BuildDate 表示构建时注入的构建时间。
	BuildDate = "unknown"
)

// Info 表示当前二进制的构建信息。
type Info struct {
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
	GoVersion string `json:"go_version"`
	Platform  string `json:"platform"`
}

// Current 返回当前进程的构建信息快照。
func Current() Info {
	return Info{
		Version:   Version,
		Commit:    Commit,
		BuildDate: BuildDate,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}
