FROM golang:1.24.1 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY src/ ./src/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -installsuffix cgo -o auto-cert ./src/main.go

FROM neilpang/acme.sh:latest

WORKDIR /root/

# 创建证书目录
RUN mkdir -p /tmp/certs

# 复制构建的应用
COPY --from=builder /app/auto-cert .
COPY scripts/acme-wrapper.sh /root/acme-wrapper.sh
RUN chmod +x /root/acme-wrapper.sh
RUN chmod +x /root/auto-cert

# 设置时区
ENV TZ=Asia/Shanghai
ENV DEBUG_MODE=false

# 配置 acme.sh 环境变量
ENV LE_CONFIG_HOME=/acme.sh

# 创建卷挂载点
VOLUME /acme.sh

CMD ["./auto-cert"]