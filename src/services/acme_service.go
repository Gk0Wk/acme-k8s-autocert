package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"me.sttot/auto-cert/src/models"
	"me.sttot/auto-cert/src/utils"
)

const (
	// acme.sh 可能的位置
	acmeShPath    = "/usr/local/bin/acme.sh"
	certOutputDir = "/tmp/certs"
)

type AcmeService struct{}

func NewAcmeService() *AcmeService {
	utils.DebugLog("创建ACME服务")
	// 确保输出目录存在
	if err := os.MkdirAll(certOutputDir, 0755); err != nil {
		utils.ErrorLog("警告: 创建证书输出目录失败: %v", err)
	} else {
		utils.DebugLog("证书输出目录: %s", certOutputDir)
	}
	return &AcmeService{}
}

// IssueCertificate 使用acme.sh签发新证书
func (a *AcmeService) IssueCertificate(ctx context.Context, cert *models.Certificate) error {
	utils.InfoLog("为域名 %v 签发新证书", cert.Domains)
	utils.DebugLog("使用DNS提供商: %s, 服务器: %s", cert.DNSProvider, cert.Server)

	// 准备命令参数
	args := []string{
		"--issue",
		"--dns", cert.DNSProvider,
		"--server", cert.Server,
	}

	// 添加所有域名
	for _, domain := range cert.Domains {
		args = append(args, "-d", domain)
	}

	// 添加email参数(如果提供)
	if cert.Email != "" {
		args = append(args, "--email", cert.Email)
		utils.DebugLog("使用邮箱: %s", cert.Email)
	}

	// 设置环境变量
	var env []string
	if len(cert.Envs) > 0 {
		utils.DebugLog("设置环境变量")
		for key, value := range cert.Envs {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
			utils.DebugLog("环境变量: %s=***", key) // 不打印实际值，保护隐私数据
		}
	}

	utils.DebugLog("执行ACME命令颁发证书")
	err := a.executeAcmeCommand(env, args, cert)
	if err != nil {
		utils.ErrorLog("常规签发失败，尝试使用--force参数强制签发: %v", err)
		// 常规签发失败，尝试强制签发
		return a.ForceRenewCertificate(context.Background(), cert)
	}

	utils.DebugLog("证书签发成功")
	return nil
}

// RenewCertificate 使用acme.sh续签证书
func (a *AcmeService) RenewCertificate(ctx context.Context, cert *models.Certificate) error {
	primaryDomain := cert.Domains[0]
	utils.InfoLog("为域名 %s 及其他 %d 个域名续签证书", primaryDomain, len(cert.Domains)-1)

	// 首先尝试常规续签
	args := []string{
		"--renew",
		"-d", primaryDomain,
	}

	// 添加所有域名
	utils.DebugLog("包含域名: %v", cert.Domains)
	for _, domain := range cert.Domains[1:] {
		args = append(args, "-d", domain)
	}

	// 添加email参数(如果提供)
	if cert.Email != "" {
		args = append(args, "--email", cert.Email)
		utils.DebugLog("使用邮箱: %s", cert.Email)
	}

	// 设置环境变量
	var env []string
	if len(cert.Envs) > 0 {
		utils.DebugLog("设置环境变量")
		for key, value := range cert.Envs {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
			utils.DebugLog("环境变量: %s=***", key) // 不打印实际值，保护隐私数据
		}
	}

	utils.DebugLog("执行ACME命令续签证书")
	err := a.executeAcmeCommand(env, args, cert)
	if err != nil {
		utils.ErrorLog("常规续签失败，尝试强制重新签发: %v", err)
		// 常规续签失败，尝试强制重新签发
		return a.ForceRenewCertificate(context.Background(), cert)
	}

	utils.DebugLog("证书续签成功")
	return nil
}

// ForceRenewCertificate 强制使用acme.sh重新签发证书
func (a *AcmeService) ForceRenewCertificate(ctx context.Context, cert *models.Certificate) error {
	utils.InfoLog("强制为域名 %v 重新签发证书", cert.Domains)

	// 准备命令参数
	args := []string{
		"--issue",
		"--force",
		"--dns", cert.DNSProvider,
		"--server", cert.Server,
	}

	// 添加所有域名
	for _, domain := range cert.Domains {
		args = append(args, "-d", domain)
	}

	// 添加email参数(如果提供)
	if cert.Email != "" {
		args = append(args, "--email", cert.Email)
		utils.DebugLog("使用邮箱: %s", cert.Email)
	}

	// 设置环境变量
	var env []string
	if len(cert.Envs) > 0 {
		utils.DebugLog("设置环境变量")
		for key, value := range cert.Envs {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
			utils.DebugLog("环境变量: %s=***", key) // 不打印实际值，保护隐私数据
		}
	}

	utils.DebugLog("执行ACME命令强制重新签发证书")
	return a.executeAcmeCommand(env, args, cert)
}

// executeAcmeCommand 执行acme.sh命令并处理结果
func (a *AcmeService) executeAcmeCommand(env []string, args []string, cert *models.Certificate) error {
	primaryDomain := cert.Domains[0]
	utils.DebugLog("主域名: %s", primaryDomain)

	// 创建临时输出目录
	outputDir := filepath.Join(certOutputDir, primaryDomain)
	utils.DebugLog("创建证书输出目录: %s", outputDir)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		utils.ErrorLog("创建证书输出目录失败: %v", err)
		return fmt.Errorf("创建证书输出目录失败: %v", err)
	}

	// 添加输出目录参数
	certFile := filepath.Join(outputDir, "cert.pem")
	keyFile := filepath.Join(outputDir, "key.pem")
	fullchainFile := filepath.Join(outputDir, "fullchain.pem")

	utils.DebugLog("证书文件路径: %s", certFile)
	utils.DebugLog("密钥文件路径: %s", keyFile)
	utils.DebugLog("完整证书链文件路径: %s", fullchainFile)

	args = append(args, "--cert-file", certFile)
	args = append(args, "--key-file", keyFile)
	args = append(args, "--fullchain-file", fullchainFile)

	// 创建完整命令
	cmd := exec.Command(acmeShPath, args...)
	utils.DebugLog("执行命令: %s %v", acmeShPath, args)

	// 设置环境变量
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
		utils.DebugLog("设置了 %d 个环境变量", len(env))
	}

	// 修改为实时输出方式
	// 创建标准输出和标准错误的管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		utils.ErrorLog("创建输出管道失败: %v", err)
		return fmt.Errorf("创建输出管道失败: %v", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		utils.ErrorLog("创建错误输出管道失败: %v", err)
		return fmt.Errorf("创建错误输出管道失败: %v", err)
	}

	// 创建一个缓冲区来存储所有输出，以便于后续使用
	var outputBuffer strings.Builder

	// 开始执行命令
	utils.DebugLog("开始执行ACME命令")
	if err := cmd.Start(); err != nil {
		utils.ErrorLog("启动ACME命令失败: %v", err)
		return fmt.Errorf("启动ACME命令失败: %v", err)
	}

	// 创建一个等待组来确保所有goroutine完成
	var wg sync.WaitGroup
	wg.Add(2)

	// 实时处理标准输出
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stdout.Read(buf)
			if n > 0 {
				output := string(buf[:n])
				outputBuffer.WriteString(output)
				utils.InfoLog("ACME输出: %s", output)
			}
			if err != nil {
				break
			}
		}
	}()

	// 实时处理标准错误
	go func() {
		defer wg.Done()
		buf := make([]byte, 1024)
		for {
			n, err := stderr.Read(buf)
			if n > 0 {
				output := string(buf[:n])
				outputBuffer.WriteString(output)
				utils.InfoLog("ACME错误: %s", output)
			}
			if err != nil {
				break
			}
		}
	}()

	// 等待命令执行完成
	err = cmd.Wait()

	// 等待所有输出读取完毕
	wg.Wait()

	if err != nil {
		utils.ErrorLog("acme.sh 命令执行失败: %v", err)
		return fmt.Errorf("acme.sh 命令执行失败: %v", err)
	}

	utils.InfoLog("acme.sh 命令执行成功")

	// 读取生成的证书和密钥
	utils.DebugLog("读取生成的证书和密钥文件")
	certPath := filepath.Join(outputDir, "fullchain.pem")
	keyPath := filepath.Join(outputDir, "key.pem")

	certData, err := os.ReadFile(certPath)
	if err != nil {
		utils.ErrorLog("读取证书文件失败: %v", err)
		return fmt.Errorf("读取证书文件失败: %v", err)
	}
	utils.DebugLog("成功读取证书文件，大小: %d 字节", len(certData))

	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		utils.ErrorLog("读取密钥文件失败: %v", err)
		return fmt.Errorf("读取密钥文件失败: %v", err)
	}
	utils.DebugLog("成功读取密钥文件，大小: %d 字节", len(keyData))

	// 将证书和密钥数据进行Base64编码并更新到证书对象
	cert.CertData = base64.StdEncoding.EncodeToString(certData)
	cert.KeyData = base64.StdEncoding.EncodeToString(keyData)
	utils.DebugLog("已将证书和密钥编码并保存到证书对象")

	utils.InfoLog("证书处理完成")
	return nil
}
