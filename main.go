package main

import (
	"os"

	"mcfg/cmd"
)

func main() {
	// 统一由 cmd.Execute 返回进程退出码，main 只负责透传标准输入输出和退出状态。
	os.Exit(cmd.Execute(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
