package controllers

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/util/errors"

	"me.sttot/auto-cert/src/services"
	"me.sttot/auto-cert/src/utils"
)

type RenewalController struct {
	acmeService        *services.AcmeService
	certificateService *services.CertificateService
}

func NewRenewalController(acmeService *services.AcmeService, certificateService *services.CertificateService) *RenewalController {
	utils.DebugLog("创建续签控制器")
	return &RenewalController{
		acmeService:        acmeService,
		certificateService: certificateService,
	}
}

func (rc *RenewalController) CheckAndRenewCertificates(ctx context.Context) {
	utils.InfoLog("检查并续签过期证书")

	certContext, err := rc.certificateService.LoadCertificateContext(ctx)
	if err != nil {
		utils.ErrorLog("加载证书上下文失败: %v", err)
		return
	}

	utils.DebugLog("已加载证书上下文，包含 %d 个证书", len(certContext.Certificates))

	var errs []error
	for name, cert := range certContext.Certificates {
		utils.DebugLog("检查证书 %s 是否需要续签", name)
		certCopy := cert // 创建副本以避免在闭包中使用循环变量
		needsRenewal, expiryTime, err := rc.certificateService.CheckCertificateExpiry(certCopy.CertData)
		if err != nil {
			utils.ErrorLog("检查证书 %s 过期时间出错: %v", name, err)
			continue
		}

		if needsRenewal {
			utils.InfoLog("证书 %s 即将在 %s 过期，尝试续签", name, expiryTime.Format("2006-01-02"))
			certPtr := &certCopy
			utils.DebugLog("开始续签证书 %s", name)

			if err := rc.acmeService.RenewCertificate(ctx, certPtr); err != nil {
				errMsg := fmt.Errorf("续签证书 %s 失败: %v", name, err)
				utils.ErrorLog(errMsg.Error())
				errs = append(errs, errMsg)
				continue
			}

			// 更新证书上下文
			utils.DebugLog("存储已续签的证书 %s", name)
			if err := rc.certificateService.StoreCertificate(ctx, certPtr); err != nil {
				errMsg := fmt.Errorf("存储已续签证书 %s 失败: %v", name, err)
				utils.ErrorLog(errMsg.Error())
				errs = append(errs, errMsg)
				continue
			}

			// 更新Kubernetes Secret
			utils.DebugLog("更新证书 %s 的Kubernetes Secret", name)
			if err := rc.certificateService.UpdateSecrets(ctx, certPtr); err != nil {
				errMsg := fmt.Errorf("更新证书 %s 的Secret失败: %v", name, err)
				utils.ErrorLog(errMsg.Error())
				errs = append(errs, errMsg)
				continue
			}

			utils.InfoLog("成功续签证书 %s", name)
		} else {
			utils.DebugLog("证书 %s 尚未过期，有效期至 %s", name, expiryTime.Format("2006-01-02"))
		}
	}

	if len(errs) > 0 {
		utils.ErrorLog("证书续签过程中发生错误: %v", errors.NewAggregate(errs))
	} else {
		utils.DebugLog("证书检查与续签完成，所有操作成功")
	}
}
