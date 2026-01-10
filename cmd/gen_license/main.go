package main

import (
	"fmt"
	"os"

	"github.com/difyz9/ytb2bili/internal/core/services"
	"github.com/difyz9/ytb2bili/internal/core/types"
	"github.com/difyz9/ytb2bili/pkg/store"
)

func main() {
	// 加载配置
	cfg, err := types.LoadConfig("config.toml")
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 初始化数据库连接
	dbConn, err := store.NewDatabase(cfg)
	if err != nil {
		fmt.Printf("Failed to initialize database: %v\n", err)
		os.Exit(1)
	}

	// 初始化服务
	svc := services.NewLicenseService(dbConn)

	key, err := svc.GenerateLicense(types.TierEnterprise, "lifetime", nil, "0000")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(key)
}
