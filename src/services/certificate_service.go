package services

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"me.sttot/auto-cert/src/models"
	"me.sttot/auto-cert/src/utils"
)

// 从环境变量获取配置，如果环境变量不存在则使用默认值
var (
	ContextSecretName      = getEnvOrDefault("CONTEXT_SECRET_NAME", "acmesh-autocert-context")
	ContextSecretNamespace = getEnvOrDefault("CONTEXT_SECRET_NAMESPACE", "default")
)

// getEnvOrDefault 从环境变量获取值，如果不存在则返回默认值
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

type Certificate struct {
	Domain   string
	CertPath string
	KeyPath  string
}

type CertificateService struct {
	clientset    *kubernetes.Clientset
	certificates map[string]Certificate
}

func NewCertificateService(clientset *kubernetes.Clientset) *CertificateService {
	utils.DebugLog("创建证书服务")
	return &CertificateService{
		clientset:    clientset,
		certificates: make(map[string]Certificate),
	}
}

// LoadCertificateContext 从Kubernetes Secret加载证书上下文
func (cs *CertificateService) LoadCertificateContext(ctx context.Context) (*models.CertificateContext, error) {
	utils.DebugLog("从Secret %s/%s加载证书上下文", ContextSecretNamespace, ContextSecretName)

	secret, err := cs.clientset.CoreV1().Secrets(ContextSecretNamespace).Get(ctx, ContextSecretName, metav1.GetOptions{})
	if err != nil {
		// 如果Secret不存在，创建一个新的上下文
		utils.DebugLog("证书上下文Secret不存在，创建新的上下文")
		certContext := &models.CertificateContext{
			Certificates: make(map[string]models.Certificate),
		}
		return certContext, nil
	}

	dataJson, ok := secret.Data["context"]
	if !ok {
		utils.DebugLog("证书上下文Secret存在但没有context字段，创建新的上下文")
		return &models.CertificateContext{
			Certificates: make(map[string]models.Certificate),
		}, nil
	}

	var certContext models.CertificateContext
	if err := json.Unmarshal(dataJson, &certContext); err != nil {
		utils.ErrorLog("解析证书上下文数据失败: %v", err)
		return nil, fmt.Errorf("unmarshal certificate context: %v", err)
	}

	utils.DebugLog("成功加载证书上下文，包含%d个证书", len(certContext.Certificates))
	return &certContext, nil
}

// SaveCertificateContext 保存证书上下文到Kubernetes Secret
func (cs *CertificateService) SaveCertificateContext(ctx context.Context, certContext *models.CertificateContext) error {
	utils.DebugLog("保存证书上下文到Secret %s/%s", ContextSecretNamespace, ContextSecretName)

	dataJson, err := json.Marshal(certContext)
	if err != nil {
		utils.ErrorLog("序列化证书上下文失败: %v", err)
		return fmt.Errorf("marshal certificate context: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ContextSecretName,
			Namespace: ContextSecretNamespace,
		},
		Data: map[string][]byte{
			"context": dataJson,
		},
	}

	_, err = cs.clientset.CoreV1().Secrets(ContextSecretNamespace).Get(ctx, ContextSecretName, metav1.GetOptions{})
	if err != nil {
		// Secret不存在，创建新的
		utils.DebugLog("创建证书上下文Secret")
		_, err = cs.clientset.CoreV1().Secrets(ContextSecretNamespace).Create(ctx, secret, metav1.CreateOptions{})
	} else {
		// Secret存在，更新
		utils.DebugLog("更新现有的证书上下文Secret")
		_, err = cs.clientset.CoreV1().Secrets(ContextSecretNamespace).Update(ctx, secret, metav1.UpdateOptions{})
	}

	if err != nil {
		utils.ErrorLog("保存证书上下文失败: %v", err)
	} else {
		utils.DebugLog("证书上下文保存成功")
	}

	return err
}

// GetCertificate 获取特定域名的证书信息
func (cs *CertificateService) GetCertificate(ctx context.Context, name string) (*models.Certificate, error) {
	utils.DebugLog("获取证书 %s 的信息", name)

	certContext, err := cs.LoadCertificateContext(ctx)
	if err != nil {
		utils.ErrorLog("加载证书上下文失败: %v", err)
		return nil, err
	}

	cert, exists := certContext.Certificates[name]
	if !exists {
		utils.DebugLog("证书 %s 不存在", name)
		return nil, nil
	}

	utils.DebugLog("找到证书 %s", name)
	return &cert, nil
}

// StoreCertificate 存储证书信息
func (cs *CertificateService) StoreCertificate(ctx context.Context, cert *models.Certificate) error {
	utils.DebugLog("存储证书 %s 信息", cert.Name)

	certContext, err := cs.LoadCertificateContext(ctx)
	if err != nil {
		utils.ErrorLog("加载证书上下文失败: %v", err)
		return err
	}

	certContext.Certificates[cert.Name] = *cert
	utils.DebugLog("添加/更新证书 %s 到上下文", cert.Name)

	return cs.SaveCertificateContext(ctx, certContext)
}

// UpdateSecrets 更新Kubernetes Secret中的证书
func (cs *CertificateService) UpdateSecrets(ctx context.Context, cert *models.Certificate) error {
	utils.DebugLog("更新证书 %s 的Kubernetes Secret", cert.Name)

	if cert.CertData == "" || cert.KeyData == "" {
		utils.ErrorLog("证书或密钥数据为空")
		return fmt.Errorf("certificate or key data is empty")
	}

	certBytes, err := base64.StdEncoding.DecodeString(cert.CertData)
	if err != nil {
		utils.ErrorLog("解码证书数据失败: %v", err)
		return fmt.Errorf("decode certificate data: %v", err)
	}

	keyBytes, err := base64.StdEncoding.DecodeString(cert.KeyData)
	if err != nil {
		utils.ErrorLog("解码密钥数据失败: %v", err)
		return fmt.Errorf("decode key data: %v", err)
	}

	// 更新所有指定的Secret
	utils.DebugLog("证书 %s 需要更新 %d 个Secret", cert.Name, len(cert.Secrets))

	for _, secretRef := range cert.Secrets {
		utils.DebugLog("更新Secret %s/%s", secretRef.Namespace, secretRef.Name)

		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      secretRef.Name,
				Namespace: secretRef.Namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": certBytes,
				"tls.key": keyBytes,
			},
		}

		_, err = cs.clientset.CoreV1().Secrets(secretRef.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
		if err != nil {
			// Secret不存在，创建新的
			utils.DebugLog("Secret %s/%s 不存在，创建新的", secretRef.Namespace, secretRef.Name)
			_, err = cs.clientset.CoreV1().Secrets(secretRef.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		} else {
			// Secret存在，更新
			utils.DebugLog("Secret %s/%s 已存在，更新", secretRef.Namespace, secretRef.Name)
			_, err = cs.clientset.CoreV1().Secrets(secretRef.Namespace).Update(ctx, secret, metav1.UpdateOptions{})
		}

		if err != nil {
			utils.ErrorLog("更新Secret %s/%s 失败: %v", secretRef.Namespace, secretRef.Name, err)
			return fmt.Errorf("update secret %s/%s: %v", secretRef.Namespace, secretRef.Name, err)
		}

		utils.DebugLog("成功更新Secret %s/%s", secretRef.Namespace, secretRef.Name)
	}

	return nil
}

// CheckCertificateExpiry 检查证书是否过期或即将过期
func (cs *CertificateService) CheckCertificateExpiry(certData string) (bool, time.Time, error) {
	utils.DebugLog("检查证书是否过期")

	certBytes, err := base64.StdEncoding.DecodeString(certData)
	if err != nil {
		utils.ErrorLog("解码证书数据失败: %v", err)
		return false, time.Time{}, fmt.Errorf("decode certificate data: %v", err)
	}

	// 解析PEM格式证书
	block, _ := pem.Decode(certBytes)
	if block == nil {
		utils.ErrorLog("解析PEM格式证书失败")
		return false, time.Time{}, fmt.Errorf("failed to parse certificate PEM")
	}

	// 解析X.509证书
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		utils.ErrorLog("解析X.509证书失败: %v", err)
		return false, time.Time{}, fmt.Errorf("parse certificate: %v", err)
	}

	// 检查证书是否过期或即将过期（30天内）
	now := time.Now()
	expiryTime := cert.NotAfter
	expiresInDays := int(expiryTime.Sub(now).Hours() / 24)

	utils.DebugLog("证书有效期至 %s（还有%d天）", expiryTime.Format("2006-01-02"), expiresInDays)

	// 如果证书已过期或将在30天内过期，返回true
	needsRenewal := expiresInDays < 30
	if needsRenewal {
		utils.DebugLog("证书将在30天内过期，需要续签")
	}

	return needsRenewal, expiryTime, nil
}
