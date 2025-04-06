#!/bin/bash
# acme-wrapper.sh - acme.sh的包装脚本，用于更好地处理证书签发和续签过程

set -e

# acme.sh 路径 - 使用软链接，这在 neilpang/acme.sh 镜像中是可用的
ACME_SH="/usr/local/bin/acme.sh"

# 检查 acme.sh 是否存在
if [ ! -f "$ACME_SH" ] && [ ! -L "$ACME_SH" ]; then
    echo "错误: acme.sh 未安装在预期位置 $ACME_SH"
    # 尝试在原始位置查找
    if [ -f "/root/.acme.sh/acme.sh" ]; then
        echo "找到 acme.sh 在 /root/.acme.sh/acme.sh，将使用此路径"
        ACME_SH="/root/.acme.sh/acme.sh"
    else
        exit 1
    fi
fi

# 解析命令行参数
ACTION=""
DOMAIN=""
DNS_PROVIDER=""
SERVER=""
CERT_PATH=""
KEY_PATH=""
FULLCHAIN_PATH=""
FORCE=false

# 解析参数
while [[ $# -gt 0 ]]; do
    case $1 in
    --issue | --renew)
        ACTION="$1"
        shift
        ;;
    -d | --domain)
        DOMAIN="$2"
        shift 2
        ;;
    --dns)
        DNS_PROVIDER="$2"
        shift 2
        ;;
    --server)
        SERVER="$2"
        shift 2
        ;;
    --cert-file)
        CERT_PATH="$2"
        shift 2
        ;;
    --key-file)
        KEY_PATH="$2"
        shift 2
        ;;
    --fullchain-file)
        FULLCHAIN_PATH="$2"
        shift 2
        ;;
    --force)
        FORCE=true
        shift
        ;;
    *)
        echo "未知参数: $1"
        exit 1
        ;;
    esac
done

# 检查必要参数
if [ -z "$ACTION" ] || [ -z "$DOMAIN" ]; then
    echo "用法: $0 --issue|--renew -d domain.com [--dns dns_provider] [--server acme_server] [--cert-file path] [--key-file path] [--fullchain-file path] [--force]"
    exit 1
fi

# 构建acme.sh命令 - 添加 --config-home 参数确保使用正确的配置目录
CMD="$ACME_SH $ACTION -d $DOMAIN --config-home /acme.sh"

# 如果是签发操作，添加DNS提供商
if [ "$ACTION" == "--issue" ] && [ ! -z "$DNS_PROVIDER" ]; then
    CMD="$CMD --dns $DNS_PROVIDER"
fi

# 如果指定了服务器
if [ ! -z "$SERVER" ]; then
    CMD="$CMD --server $SERVER"
fi

# 如果有指定证书输出路径
if [ ! -z "$CERT_PATH" ]; then
    CMD="$CMD --cert-file $CERT_PATH"
fi

# 如果有指定密钥输出路径
if [ ! -z "$KEY_PATH" ]; then
    CMD="$CMD --key-file $KEY_PATH"
fi

# 如果有指定全链证书输出路径
if [ ! -z "$FULLCHAIN_PATH" ]; then
    CMD="$CMD --fullchain-file $FULLCHAIN_PATH"
fi

# 如果需要强制签发
if [ "$FORCE" = true ]; then
    CMD="$CMD --force"
fi

echo "执行命令: $CMD"
eval "$CMD"

# 检查命令执行结果
if [ $? -eq 0 ]; then
    echo "证书操作成功完成"

    # 验证证书文件是否存在
    if [ ! -z "$FULLCHAIN_PATH" ] && [ -f "$FULLCHAIN_PATH" ]; then
        echo "证书已生成: $FULLCHAIN_PATH"
    else
        echo "警告: 找不到生成的证书文件"
    fi

    if [ ! -z "$KEY_PATH" ] && [ -f "$KEY_PATH" ]; then
        echo "私钥已生成: $KEY_PATH"
    else
        echo "警告: 找不到生成的私钥文件"
    fi

    exit 0
else
    echo "证书操作失败"
    exit 1
fi
