package controllers

import (
	"context"
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/workqueue"

	"me.sttot/auto-cert/src/models"
	"me.sttot/auto-cert/src/services"
	"me.sttot/auto-cert/src/utils"
)

// 从环境变量获取配置，如果环境变量不存在则使用默认值
var (
	// 证书配置保存的Secret
	ConfigSecretName      = getEnvOrDefault("CONFIG_SECRET_NAME", "autocert-config")
	ConfigSecretNamespace = getEnvOrDefault("CONFIG_SECRET_NAMESPACE", "default")
	ConfigMapKey          = getEnvOrDefault("CONFIG_MAP_KEY", "config.yaml")

	// 证书检查周期
	CheckInterval = getCheckIntervalFromEnv()
)

// getEnvOrDefault 从环境变量获取值，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getCheckIntervalFromEnv 从环境变量获取检查周期
func getCheckIntervalFromEnv() time.Duration {
	value := os.Getenv("CHECK_INTERVAL")
	if value == "" {
		return 24 * time.Hour // 默认为24小时
	}

	// 尝试解析时间字符串
	duration, err := time.ParseDuration(value)
	if err != nil {
		utils.WarningLog("无法解析CHECK_INTERVAL环境变量 '%s', 使用默认值24小时: %v", value, err)
		return 24 * time.Hour
	}
	return duration
}

type CertificateController struct {
	clientset          *kubernetes.Clientset
	certificateService *services.CertificateService
	acmeService        *services.AcmeService
	queue              workqueue.RateLimitingInterface
	stopCh             chan struct{}
}

func NewCertificateController(clientset *kubernetes.Clientset, certService *services.CertificateService, acmeService *services.AcmeService) *CertificateController {
	controller := &CertificateController{
		clientset:          clientset,
		certificateService: certService,
		acmeService:        acmeService,
		queue:              workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "certificates"),
		stopCh:             make(chan struct{}),
	}

	return controller
}

// Start 启动证书控制器
func (c *CertificateController) Start(ctx context.Context) error {
	utils.InfoLog("启动证书控制器")
	utils.DebugLog("证书检查周期为 %s", CheckInterval)

	// 立即处理所有证书
	if err := c.ProcessAllCertificates(ctx); err != nil {
		utils.ErrorLog("初始处理证书失败: %v", err)
	}

	// 启动定期检查证书的goroutine
	// 这里使用time.Sleep和单独的goroutine替代wait.Until的立即执行特性
	// 这样可以避免在启动时重复执行ProcessAllCertificates
	go func() {
		utils.DebugLog("启动定期证书检查任务，首次检查将在 %s 后执行", CheckInterval)
		ticker := time.NewTicker(CheckInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				utils.DebugLog("执行定期证书检查任务")
				if err := c.ProcessAllCertificates(ctx); err != nil {
					utils.ErrorLog("定期处理证书失败: %v", err)
				}
			case <-c.stopCh:
				utils.DebugLog("定期证书检查任务已停止")
				return
			}
		}
	}()

	return nil
}

// Stop 停止证书控制器
func (c *CertificateController) Stop() {
	utils.InfoLog("停止证书控制器")
	close(c.stopCh)
	c.queue.ShutDown()
}

// LoadCertificatesFromConfig 从配置Secret加载证书配置
func (c *CertificateController) LoadCertificatesFromConfig(ctx context.Context) ([]*models.Certificate, error) {
	utils.DebugLog("从Secret %s/%s加载证书配置", ConfigSecretNamespace, ConfigSecretName)

	secret, err := c.clientset.CoreV1().Secrets(ConfigSecretNamespace).Get(ctx, ConfigSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("获取配置Secret失败: %v", err)
	}

	configYaml, ok := secret.Data[ConfigMapKey]
	if !ok {
		return nil, fmt.Errorf("配置Secret中没有找到config.yaml")
	}

	// 解析YAML配置
	utils.DebugLog("解析证书配置YAML数据")
	certs, err := parseYamlConfig(configYaml)
	if err != nil {
		return nil, fmt.Errorf("解析证书配置失败: %v", err)
	}

	utils.DebugLog("成功加载了%d个证书配置", len(certs))
	return certs, nil
}

// ProcessAllCertificates 处理所有证书
func (c *CertificateController) ProcessAllCertificates(ctx context.Context) error {
	// 从配置加载证书
	utils.DebugLog("开始处理所有证书")

	certs, err := c.LoadCertificatesFromConfig(ctx)
	if err != nil {
		return fmt.Errorf("加载证书配置失败: %v", err)
	}

	utils.DebugLog("准备处理%d个证书", len(certs))
	for _, cert := range certs {
		utils.DebugLog("开始处理证书 %s", cert.Name)
		if err := c.ProcessCertificate(ctx, cert); err != nil {
			utils.ErrorLog("处理证书 %s 失败: %v", cert.Name, err)
			// 继续处理下一个证书
			continue
		}
	}

	utils.DebugLog("所有证书处理完成")
	return nil
}

// ProcessCertificate 处理单个证书
func (c *CertificateController) ProcessCertificate(ctx context.Context, cert *models.Certificate) error {
	utils.InfoLog("处理证书: %s, 域名: %v", cert.Name, cert.Domains)
	utils.DebugLog("证书提供方: %s, 服务器: %s", cert.DNSProvider, cert.Server)

	// 从存储中获取证书
	existingCert, err := c.certificateService.GetCertificate(ctx, cert.Name)
	if err != nil {
		return fmt.Errorf("获取证书信息失败: %v", err)
	}

	// 如果证书不存在，则尝试颁发新证书
	if existingCert == nil {
		utils.InfoLog("证书 %s 不存在，颁发新证书", cert.Name)
		utils.DebugLog("开始为域名 %v 颁发新证书", cert.Domains)

		if err := c.acmeService.IssueCertificate(ctx, cert); err != nil {
			return fmt.Errorf("颁发证书失败: %v", err)
		}

		// 存储证书信息
		utils.DebugLog("存储新颁发的证书信息")
		if err := c.certificateService.StoreCertificate(ctx, cert); err != nil {
			return fmt.Errorf("存储证书信息失败: %v", err)
		}

		// 更新Secret
		utils.DebugLog("更新Secret中的证书数据")
		if err := c.certificateService.UpdateSecrets(ctx, cert); err != nil {
			return fmt.Errorf("更新Secret失败: %v", err)
		}

		utils.InfoLog("颁发并存储证书 %s 成功", cert.Name)
		return nil
	}

	// 检查证书是否过期或即将过期
	needsRenewal := false
	if existingCert.CertData != "" {
		utils.DebugLog("检查证书 %s 是否需要续签", cert.Name)
		var expiryTime time.Time
		needsRenewal, expiryTime, err = c.certificateService.CheckCertificateExpiry(existingCert.CertData)
		if err != nil {
			utils.ErrorLog("检查证书有效期失败: %v，尝试续签", err)
			needsRenewal = true
		} else if needsRenewal {
			utils.InfoLog("证书 %s 将在 %s 过期，需要续签", cert.Name, expiryTime.Format("2006-01-02"))
		} else {
			utils.InfoLog("证书 %s 有效期至 %s，不需要续签", cert.Name, expiryTime.Format("2006-01-02"))
		}
	} else {
		// 如果没有证书数据，也需要续签
		utils.DebugLog("证书 %s 缺少数据，需要续签", cert.Name)
		needsRenewal = true
	}

	// 如果需要续签，则尝试续签
	if needsRenewal {
		// 复制域名和配置信息到现有证书
		existingCert.Domains = cert.Domains
		existingCert.DNSProvider = cert.DNSProvider
		existingCert.Server = cert.Server
		existingCert.Secrets = cert.Secrets

		utils.DebugLog("更新证书配置: 域名=%v, 提供方=%s", existingCert.Domains, existingCert.DNSProvider)

		utils.InfoLog("尝试续签证书 %s", cert.Name)
		if err := c.acmeService.RenewCertificate(ctx, existingCert); err != nil {
			return fmt.Errorf("续签证书失败: %v", err)
		}

		// 存储更新后的证书信息
		utils.DebugLog("存储更新后的证书信息")
		if err := c.certificateService.StoreCertificate(ctx, existingCert); err != nil {
			return fmt.Errorf("存储更新的证书信息失败: %v", err)
		}

		// 更新Secret
		utils.DebugLog("更新Secret中的证书数据")
		if err := c.certificateService.UpdateSecrets(ctx, existingCert); err != nil {
			return fmt.Errorf("更新Secret失败: %v", err)
		}

		utils.InfoLog("续签并更新证书 %s 成功", cert.Name)
	} else {
		// 确保Secret中的证书是最新的
		utils.DebugLog("确保Secret中的证书是最新的")
		if err := c.certificateService.UpdateSecrets(ctx, existingCert); err != nil {
			return fmt.Errorf("更新Secret失败: %v", err)
		}
		utils.DebugLog("Secret包含最新的证书数据")
	}

	return nil
}

// parseYamlConfig 解析YAML配置为证书对象
func parseYamlConfig(yamlData []byte) ([]*models.Certificate, error) {
	var config struct {
		Domains []models.Certificate `yaml:"domains"`
	}

	if err := yaml.Unmarshal(yamlData, &config); err != nil {
		return nil, fmt.Errorf("解析YAML配置失败: %v", err)
	}

	var result []*models.Certificate
	for i := range config.Domains {
		result = append(result, &config.Domains[i])
	}

	return result, nil
}
