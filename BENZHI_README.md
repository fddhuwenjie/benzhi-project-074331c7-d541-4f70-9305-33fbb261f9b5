# BENZHI_README

## 项目说明
- 项目：benzhi-project-074331c7-d541-4f70-9305-33fbb261f9b5
- 项目用途：触觉导览图制版校验工作台提供规格冻结、结构化校样、盲文与版面规则检查、问题整改、定向复验、独立批准和不可变母版发布清单的完整闭环。
- Go 工具链：`golang:1.23`
- 前端工具链：原生 HTML、CSS 和 JavaScript，由 Go 服务直接提供

## 项目描述
- 项目名称：tactile-atlas-gate
- 项目介绍：面向博物馆无障碍内容团队的触觉导览图制版校验工作台，将导览点语义、盲文标签、触觉符号与版面约束纳入一次可追溯的校样发布闭环，避免未经复验的母版进入制作环节。
- 项目概述：面向博物馆无障碍内容团队的触觉导览图制版校验工作台，将导览点语义、盲文标签、触觉符号与版面约束纳入一次可追溯的校样发布闭环，避免未经复验的母版进入制作环节。
- 核心工作流：制版员创建导览图项目并冻结制作规格，登记首版校样中的导览点、触觉符号和盲文标签，执行确定性规则检查；复核员逐项确认问题，制版员提交修订版并完成定向复验，全部问题关闭后由复核员批准，系统把项目状态从草拟依次推进至已冻结规格、检查中、待整改、待批准和已发布，并生成不可变母版清单。
- 对外接口：Go 服务在同源地址提供原生 HTML、CSS 和 JavaScript 单页工作台，页面包含项目状态栏、规格与校样编辑区、规则检查结果、问题整改队列、版本差异、审批区和发布清单视图；同源 JSON 端点仅服务该页面，不作为独立产品界面。

## 标准构建、运行和测试命令
进入容器后执行：
cd '/app' && GOTOOLCHAIN=local go build ./...

cd /app && GOTOOLCHAIN=local go run ./cmd/server -self-check -addr=127.0.0.1:19081

cd '/app' && GOTOOLCHAIN=local go test ./...

## Docker 构建和进入容器
chmod +x build_benzhi_docker.sh

./build_benzhi_docker.sh benzhi-project-074331c7-d541-4f70-9305-33fbb261f9b5-amd64 linux/amd64

./build_benzhi_docker.sh benzhi-project-074331c7-d541-4f70-9305-33fbb261f9b5-arm64 linux/arm64

docker run -it benzhi-project-074331c7-d541-4f70-9305-33fbb261f9b5-amd64:latest

## 题目验证命令
1. 预期退出码 0：`go test ./...`
2. 预期退出码 0：`go run ./cmd/server -self-check -addr=127.0.0.1:19081`
