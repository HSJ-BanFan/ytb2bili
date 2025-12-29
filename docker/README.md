# Docker 部署

此目录包含 Docker 相关的配置文件和脚本。

## 📦 文件说明

### Docker 配置文件

- `Dockerfile` - 主应用 Docker 镜像构建文件
- `Dockerfile.fixed` - 修复版本的 Dockerfile
- `Dockerfile.nginx.fixed` - Nginx 前端容器 Dockerfile
- `docker-compose.yml` - 容器编排配置
- `docker-entrypoint.sh` - 容器启动脚本
- `nginx-frontend.conf.fixed` - Nginx 配置文件

## 🚀 快速开始

### 构建并启动

```bash
cd docker
docker-compose up -d
```

### 查看日志

```bash
docker-compose logs -f
```

### 停止服务

```bash
docker-compose down
```

## 📋 部署架构

```
┌─────────────────┐
│   浏览器访问    │ :80
└────────┬────────┘
         │
┌────────▼────────┐
│  Nginx 容器     │ (前端静态文件 + API 反向代理)
└────────┬────────┘
         │
┌────────▼────────┐
│  Go 应用容器    │ (后端 API)
└─────────────────┘
```

## 🔗 相关文档

- [Docker 部署完整指南](../docs/08-文档归档/DOCKER_DEPLOYMENT.md)
- [前端构建说明](../docs/08-文档归档/FRONTEND_BUILD_GUIDE.md)

## ⚙️ 环境变量

参考 `config.toml.example` 配置应用环境。

---

*最后更新: 2024-12-29*
