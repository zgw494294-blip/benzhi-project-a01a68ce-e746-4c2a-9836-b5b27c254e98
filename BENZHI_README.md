# BENZHI_README

## 项目说明
- 项目：benzhi-project-a01a68ce-e746-4c2a-9836-b5b27c254e98
- 项目用途：食品研发实验室盲样感官评审工作台，支持会话配置、可复现盲码、隔离评分、确定性核验、异常裁定、双角色解盲和不可变归档。
- Go 工具链：`golang:1.22`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...
cd /app && GOTOOLCHAIN=local go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck
cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh
./build_benzhi_docker.sh benzhi-project-a01a68ce-e746-4c2a-9836-b5b27c254e98-amd64 linux/amd64
./build_benzhi_docker.sh benzhi-project-a01a68ce-e746-4c2a-9836-b5b27c254e98-arm64 linux/arm64
docker run -it benzhi-project-a01a68ce-e746-4c2a-9836-b5b27c254e98-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -addr=127.0.0.1:19081 -selfcheck`
