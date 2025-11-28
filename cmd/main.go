package main

import (
	"fmt"
	"os"

	"github.com/Sky-ey/HexDiff/pkg/cli"
)

const (
	AppName        = "hexdiff"
	AppVersion     = "1.0.0"
	AppDescription = "高效的二进制补丁工具"
)

func main() {
	// 创建引擎适配器
	engine, err := cli.NewEngineAdapter()
	if err != nil {
		fmt.Fprintf(os.Stderr, "初始化引擎失败: %v\n", err)
		os.Exit(1)
	}

	// 创建CLI应用程序
	app := cli.NewApp(AppName, AppVersion, AppDescription, engine)

	// 创建错误处理器
	errorHandler := cli.NewErrorHandler(app.GetLogger(), false)

	// 运行应用程序
	if err := app.Run(os.Args); err != nil {
		exitCode := errorHandler.Handle(err)
		os.Exit(exitCode)
	}
}
