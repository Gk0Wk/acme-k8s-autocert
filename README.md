# AutoCert - 自定义的Kubernetes证书管理器

AutoCert 是一个类似 cert-manager 的自定义Kubernetes证书管理器，它使用 acme.sh 来自动签发和续签 Let's Encrypt 证书。本项目专为需要定制证书管理流程的Kubernetes环境设计，提供了完整的证书生命周期管理功能。

项目地址: https://github.com/Gk0Wk/acme-k8s-autocert

## 目录

1. [AutoCert - 自定义的Kubernetes证书管理器](#autocert---自定义的kubernetes证书管理器)
   1. [目录](#目录)
   2. [特性](#特性)
   3. [架构设计](#架构设计)
   4. [功能详解](#功能详解)
      1. [证书签发](#证书签发)
      2. [证书续签](#证书续签)
      3. [证书存储](#证书存储)
   5. [安装](#安装)
      1. [前提条件](#前提条件)
      2. [构建镜像](#构建镜像)
   6. [配置说明](#配置说明)
      1. [证书配置](#证书配置)
      2. [DNS提供商支持](#dns提供商支持)
   7. [故障排查](#故障排查)
      1. [常见问题](#常见问题)
      2. [日志分析](#日志分析)
   8. [开发者指南](#开发者指南)
      1. [代码结构](#代码结构)
      2. [贡献指南](#贡献指南)
   9. [License](#license)
   10. [使用Helm部署](#使用helm部署)
       1. [前提条件](#前提条件-1)
       2. [使用Helm部署步骤](#使用helm部署步骤)
       3. [Helm Chart参数说明](#helm-chart参数说明)
       4. [Helm Chart高级配置](#helm-chart高级配置)
          1. [使用现有的Secret](#使用现有的secret)
          2. [配置资源限制](#配置资源限制)
          3. [配置节点选择器和容忍度](#配置节点选择器和容忍度)

## 特性

- 支持自动签发和续签 Let's Encrypt 证书
- 支持多种 DNS 提供商的 DNS-01 验证方式
- 支持多域名和通配符证书
- 自动更新 Kubernetes Secret，集成到Ingress和其他服务
- 持久化证书状态，确保可靠的证书管理
- 证书接近过期时自动续签（30天前）
- 灵活的配置选项，支持自定义存储位置和DNS参数

## 架构设计

AutoCert 采用以下组件架构:

1. **证书控制器 (CertificateController)**: 负责证书的整体生命周期管理
2. **续签控制器 (RenewalController)**: 定期检查证书并处理自动续签
3. **证书服务 (CertificateService)**: 处理证书的存储和检索
4. **ACME服务 (AcmeService)**: 与acme.sh交互，执行证书签发和续签操作

整体工作流程:

```
                  +-------------------+
                  | Kubernetes Secret |
                  | (配置和证书存储)   |
                  +--------+----------+
                           |
                           v
+---------------+   +------+---------+   +--------------+
| 证书控制器     |-->| 证书服务       |<--| 续签控制器    |
+-------+-------+   +------+---------+   +--------------+
        |                  |
        v                  v
+---------------+   +------+---------+
| ACME服务      |-->| acme.sh        |
+---------------+   +----------------+
```

## 功能详解

### 证书签发

AutoCert 使用 acme.sh 通过 DNS-01 验证方式向 Let's Encrypt 申请证书。支持通配符证书和多域名证书。签发过程:

1. 从配置中读取域名和DNS提供商信息
2. 设置必要的DNS API环境变量
3. 调用acme.sh执行DNS验证和证书签发
4. 将证书存储到Kubernetes Secret中

### 证书续签

AutoCert 会自动检查证书的有效期:

1. RenewalController 定期检查所有证书
2. 对于30天内即将过期的证书，触发续签流程
3. 如果常规续签失败，将尝试强制重新签发
4. 续签成功后更新Kubernetes Secret

### 证书存储

证书数据以两种方式存储:

1. **上下文存储**: 证书元数据和状态存储在 `acmesh-autocert-context` Secret中
2. **用户Secret**: 实际的证书和私钥存储在用户配置的Secret中，可用于Ingress等服务

## 安装

### 前提条件

- Kubernetes 集群 (1.16+)
- 对应DNS提供商的API访问凭证
- 域名所有权

### 构建镜像

```bash
# 构建Docker镜像
docker build -t your-registry/autocert:latest .

# 推送镜像到仓库 (可选)
docker push your-registry/autocert:latest
```

## 配置说明

### 证书配置

每个证书配置包含以下字段:

| 字段 | 描述 | 示例 |
|------|------|------|
| name | 证书标识名 | example.com |
| domains | 域名列表 | ["*.example.com", "example.com"] |
| dns | DNS提供商 | dns_cf |
| server | ACME服务器 | https://acme-v02.api.letsencrypt.org/directory |
| secrets | 证书存储位置 | 见下文 |

Secret 配置:

| 字段 | 描述 | 示例 |
|------|------|------|
| namespace | Secret命名空间 | default |
| name | Secret名称 | example-tls |
| envs | 环境变量 | CF_Key: "apikey" |

### DNS提供商支持

AutoCert 通过 acme.sh 支持多种DNS提供商:

| 提供商 | 代码 | 必需的环境变量 |
|--------|------|--------------|
| Cloudflare | dns_cf | CF_Key, CF_Email |
| Aliyun | dns_ali | Ali_Key, Ali_Secret |
| DNSPod | dns_dp | DP_Id, DP_Key |
| AWS Route53 | dns_aws | AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY |
| GoDaddy | dns_gd | GD_Key, GD_Secret |

完整列表请参考 [acme.sh DNS API文档](https://github.com/acmesh-official/acme.sh/wiki/dnsapi)

## 故障排查

### 常见问题

1. **证书签发失败**

   - 检查DNS API凭证是否正确
   - 确认域名是否归属于当前DNS账户
   - 查看acme.sh日志了解具体错误

   ```bash
   kubectl logs -f <pod-name> -n <namespace>
   ```

2. **Secret未更新**

   - 检查RBAC权限是否正确配置
   - 确认Secret命名空间和名称是否正确
   - 验证AutoCert服务是否有权限更新目标Secret

3. **证书续签问题**

   - 检查certificateService.GetAllCertificates函数是否正常工作
   - 确认RenewalController是否正确启动
   - 查看日志中是否有关于续签的错误信息

### 日志分析

AutoCert输出的日志包含以下关键信息:

- 证书处理流程的每个步骤
- acme.sh命令的执行输出
- 证书的过期时间和续签计划
- 错误和异常情况

## 开发者指南

### 代码结构

```
src/
  ├── main.go                    # 应用程序入口点
  ├── controllers/               # 控制器模块
  │   ├── certificate_controller.go  # 证书主控制器
  │   └── renewal_controller.go  # 证书续签控制器
  ├── models/                    # 数据模型
  │   └── certificate.go         # 证书相关数据结构
  └── services/                  # 服务模块
      ├── acme_service.go        # ACME操作服务
      └── certificate_service.go # 证书管理服务
```

### 贡献指南

如果您想为项目做出贡献，请遵循以下步骤:

1. Fork项目并克隆到本地
2. 创建功能分支 (`git checkout -b feature/amazing-feature`)
3. 提交更改 (`git commit -m 'Add some amazing feature'`)
4. 推送到分支 (`git push origin feature/amazing-feature`)
5. 创建Pull Request

## License

MIT License

Copyright (c) 2025 Your Name

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

## 使用Helm部署

AutoCert项目提供了Helm Chart，可以更简便地在Kubernetes集群中部署和管理。

### 前提条件

- Kubernetes 集群 (1.16+)
- [Helm](https://helm.sh/docs/intro/install/) 3.0+
- 对应DNS提供商的API访问凭证
- 域名所有权

### 使用Helm部署步骤

1. **克隆项目仓库**

```bash
git clone https://github.com/Gk0Wk/acme-k8s-autocert.git
cd acme-k8s-autocert
```

2. **修改配置值**

创建一个自定义的values.yaml文件，根据您的环境进行配置：

```bash
# 复制示例配置文件并修改
cp charts/autocert/values.yaml my-values.yaml
```

修改 `my-values.yaml` 文件，主要关注以下配置：

```yaml
# 镜像配置
image:
  repository: your-registry/autocert
  tag: "latest"

# 证书配置
certificates:
  configExamples:
    enabled: true
    config: |-
      domains:
        - name: yourdomain.com
          domains:
            - "*.yourdomain.com"
            - "yourdomain.com"
          dns: "dns_cf"
          server: "https://acme-v02.api.letsencrypt.org/directory"
          email: "admin@yourdomain.com"
          secrets:
            - namespace: "default"
              name: "yourdomain-tls"
          envs:
            CF_Key: "your-cloudflare-api-key"
            CF_Email: "your-cloudflare-email"

# RBAC配置
rbac:
  create: true
  clusterWide: true

# 持久化存储配置
persistence:
  enabled: true
  storageClass: "standard"
  size: 1Gi
```

3. **使用Helm安装Chart**

从本地安装：

```bash
helm install autocert ./charts/autocert -f my-values.yaml -n cert-system --create-namespace
```

4. **验证部署**

```bash
# 检查Pod是否正常运行
kubectl get pods -n cert-system -l app.kubernetes.io/name=autocert

# 查看AutoCert日志
kubectl logs -f -n cert-system -l app.kubernetes.io/name=autocert
```

5. **更新部署**

如果需要更新配置或升级版本，可以使用以下命令：

```bash
# 编辑my-values.yaml后执行
helm upgrade autocert ./charts/autocert -f my-values.yaml -n cert-system
```

6. **卸载**

```bash
helm uninstall autocert -n cert-system
```

### Helm Chart参数说明

| 参数 | 描述 | 默认值 |
|------|------|-------|
| `image.repository` | 镜像仓库 | `autocert` |
| `image.tag` | 镜像标签 | `latest` |
| `image.pullPolicy` | 镜像拉取策略 | `IfNotPresent` |
| `replicaCount` | 副本数 | `1` |
| `certificates.contextSecretName` | 证书上下文Secret名称 | `acmesh-autocert-context` |
| `certificates.checkInterval` | 证书检查间隔 | `24h` |
| `certificates.configExamples.enabled` | 是否启用示例配置 | `false` |
| `certificates.existingSecret.enabled` | 是否使用已存在的配置Secret | `false` |
| `rbac.create` | 是否创建RBAC资源 | `true` |
| `rbac.clusterWide` | 是否使用集群级别权限 | `true` |
| `persistence.enabled` | 是否启用持久化存储 | `true` |
| `persistence.size` | 存储大小 | `1Gi` |
| `persistence.storageClass` | 存储类 | `""` |

完整参数列表请参考 `charts/autocert/values.yaml` 文件。

### Helm Chart高级配置

#### 使用现有的Secret

如果您已经有一个包含证书配置的Secret，可以指定使用它而不是创建新的：

```yaml
certificates:
  existingSecret:
    enabled: true
    name: "your-existing-secret"
    namespace: "your-namespace"
    key: "config.yaml"
```

#### 配置资源限制

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

#### 配置节点选择器和容忍度

```yaml
nodeSelector:
  kubernetes.io/os: linux

tolerations:
- key: "dedicated"
  operator: "Equal"
  value: "autocert"
  effect: "NoSchedule"
```