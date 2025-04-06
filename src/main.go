package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"me.sttot/auto-cert/src/controllers"
	"me.sttot/auto-cert/src/services"
	"me.sttot/auto-cert/src/utils"
)

func main() {
	// 初始化日志系统
	utils.InitLogger()

	utils.InfoLog("启动 AutoCert 服务...")

	// 获取 Kubernetes 配置
	cfg, err := config.GetConfig()
	if err != nil {
		log.Fatalf("无法获取 Kubernetes 配置: %v", err)
	}

	utils.DebugLog("成功获取Kubernetes配置")

	// 创建 Kubernetes 客户端
	clientset, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("无法创建 Kubernetes 客户端: %v", err)
	}

	utils.DebugLog("成功创建Kubernetes客户端")

	// 初始化服务
	acmeService := services.NewAcmeService()
	certificateService := services.NewCertificateService(clientset)

	utils.DebugLog("服务初始化完成")

	// 初始化控制器
	certController := controllers.NewCertificateController(clientset, certificateService, acmeService)
	utils.DebugLog("控制器初始化完成")

	// 创建上下文
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 启动控制器
	utils.DebugLog("正在启动证书控制器...")
	if err := certController.Start(ctx); err != nil {
		log.Fatalf("启动证书控制器失败: %v", err)
	}
	utils.DebugLog("证书控制器已成功启动")

	// 等待信号以优雅退出
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	<-sigCh

	utils.InfoLog("收到退出信号，正在停止服务...")
	certController.Stop()
	utils.DebugLog("控制器已停止")
	utils.InfoLog("服务已停止")
}
