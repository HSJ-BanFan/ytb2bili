# Docker 部署指南

本文档说明如何使用 Docker 部署 ytb2bili 应用，包括前后端分离架构和 SQLite 支持。

## 📋 目录

- [架构概述](#架构概述)
- [前置要求](#前置要求)
- [快速开始](#快速开始)
- [部署模式](#部署模式)
- [环境变量配置](#环境变量配置)
- [数据持久化](#数据持久化)
- [常见问题](#常见问题)

---

## 🏗️ 架构概述

### 新架构特性（PR #1）

1. **前后端分离**：前端使用独立的 Nginx 容器，后端为 Go 应用
2. **SQLite 支持**：后端支持 SQLite 数据库（适合单机部署）
3. **多数据库支持**：可选 MySQL/PostgreSQL（适合生产环境）
4. **反向代理**：Nginx 自动转发 API 请求到后端

### 服务组件

```
┌─────────────────┐
│   浏览器访问    │ :80
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Frontend (Nginx)│ 
│  - 静态文件服务  │
│  - API 代理转发  │
└────────┬────────┘
         │ /api/* 请求
         ▼
┌─────────────────┐
│  Backend (Go)   │ :8096
│  - 业务逻辑     │
│  - 视频处理     │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│   Database      │
│  SQLite/MySQL   │
└─────────────────┘
```

---

## ✅ 前置要求

- Docker >= 20.10
- Docker Compose >= 2.0
- 磁盘空间 >= 10GB（用于视频缓存）
- 内存 >= 2GB

---

## 🚀 快速开始

### 方式一：使用 SQLite（推荐新手）

**1. 克隆项目**
```bash
git clone https://github.com/difyz9/ytb2bili.git
cd ytb2bili
```

**2. 配置数据库类型**
```bash
cp config.toml.example config.toml
```

编辑 `config.toml`，设置数据库为 SQLite：
```toml
[database]
type = "sqlite"
dsn = "/data/ytb2bili/ytb2bili.db"
```

**3. 启动服务（仅后端 + 前端）**
```bash
docker-compose up -d ytb2bili frontend
```

**4. 访问应用**
- 前端界面：http://localhost
- 后端 API：http://localhost:8096
- 健康检查：http://localhost/health

---

### 方式二：使用 MySQL（推荐生产）

**1. 配置环境变量**
```bash
cat > .env <<EOF
MYSQL_ROOT_PASSWORD=your_secure_root_password
MYSQL_DATABASE=ytb2bili
MYSQL_USER=ytb2bili
MYSQL_PASSWORD=your_secure_password
EOF
```

**2. 修改 config.toml**
```toml
[database]
type = "mysql"
host = "mysql"  # Docker Compose 服务名
port = 3306
user = "ytb2bili"
password = "your_secure_password"
database = "ytb2bili"
```

**3. 启动完整服务**
```bash
docker-compose up -d
```

---

## 🔧 部署模式

### 1. 开发模式（本地测试）

```bash
# 仅启动后端（前端本地开发）
docker-compose up -d ytb2bili

# 前端开发服务器
cd web
npm install
npm run dev
```

### 2. 生产模式（完整部署）

```bash
# 构建并启动所有服务
docker-compose up -d --build

# 查看日志
docker-compose logs -f

# 查看服务状态
docker-compose ps
```

### 3. 最小化部署（仅必需服务）

```bash
# 后端 + 前端 + SQLite（无 MySQL/Redis）
docker-compose up -d ytb2ibili frontend
```

---

## 🌍 环境变量配置

### 后端服务（ytb2bili）

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `CONFIG_FILE` | 配置文件路径 | `/app/config.toml` |
| `TZ` | 时区设置 | `Asia/Shanghai` |

### 前端服务（frontend）

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `BACKEND_URL` | 后端服务地址 | `http://ytb2bili:8096` |

**自定义后端地址示例**：
```yaml
# docker-compose.yml
services:
  frontend:
    environment:
      - BACKEND_URL=http://your-backend-domain.com:8096
```

### MySQL 服务

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| `MYSQL_ROOT_PASSWORD` | Root 密码 | `ytb2bili_root_2024` |
| `MYSQL_DATABASE` | 数据库名 | `ytb2bili` |
| `MYSQL_USER` | 用户名 | `ytb2bili` |
| `MYSQL_PASSWORD` | 用户密码 | `ytb2bili_2024` |

---

## 💾 数据持久化

Docker Compose 自动创建以下 Volume：

| Volume 名称 | 挂载路径 | 用途 |
|-------------|----------|------|
| `ytb2bili_data` | `/data/ytb2bili` | 应用数据（SQLite DB、临时文件） |
| `ytb2bili_logs` | `/app/logs` | 应用日志 |
| `mysql_data` | `/var/lib/mysql` | MySQL 数据文件 |
| `redis_data` | `/data` | Redis 数据 |

### 备份数据

**SQLite 备份**：
```bash
# 导出数据库
docker exec ytb2bili-app cp /data/ytb2bili/ytb2bili.db /app/backup.db
docker cp ytb2bili-app:/app/backup.db ./ytb2bili-backup-$(date +%Y%m%d).db

# 恢复数据库
docker cp ./ytb2bili-backup.db ytb2bili-app:/data/ytb2bili/ytb2bili.db
```

**MySQL 备份**：
```bash
# 导出
docker exec ytb2bili-mysql mysqldump -u root -p'ytb2bili_root_2024' ytb2bili > backup.sql

# 导入
docker exec -i ytb2bili-mysql mysql -u root -p'ytb2bili_root_2024' ytb2bili < backup.sql
```

---

## 🔍 常见问题

### 1. 前端无法连接后端

**症状**：浏览器控制台显示 API 请求失败

**解决方案**：
```bash
# 检查后端服务是否启动
docker-compose ps ytb2bili

# 查看后端日志
docker-compose logs ytb2bili

# 测试健康检查
curl http://localhost/health
```

### 2. 端口冲突

**症状**：`Error: bind: address already in use`

**解决方案**：
```yaml
# 修改 docker-compose.yml，更改端口映射
services:
  frontend:
    ports:
      - "8080:80"  # 改为其他端口
```

### 3. 数据库连接失败

**症状**：后端启动失败，日志显示 "failed to connect to database"

**解决方案**：
```bash
# 检查数据库服务状态
docker-compose ps mysql

# 验证 config.toml 中的数据库配置
cat config.toml | grep -A 5 "\[database\]"

# 重启服务
docker-compose restart ytb2bili
```

### 4. Go 版本不支持

**症状**：构建时出现 `golang:1.24-alpine not found`

**解决方案**：
```dockerfile
# 修改 Dockerfile 第3行
FROM golang:1.23-alpine AS backend-builder
```

### 5. 权限问题（Linux）

**症状**：容器内无法写入文件

**解决方案**：
```bash
# 检查挂载目录权限
ls -la /path/to/mounted/dir

# 修改所有者（UID 1001 是容器内的 ytb2bili 用户）
sudo chown -R 1001:1001 ./data ./logs
```

---

## 🛠️ 高级配置

### 自定义 Nginx 配置

如需修改前端路由规则，编辑 `nginx-frontend.conf`：

```nginx
# 添加自定义响应头
location / {
    add_header X-Frame-Options "SAMEORIGIN";
    add_header X-Content-Type-Options "nosniff";
    try_files $uri $uri/ /index.html;
}
```

### 启用 HTTPS

1. 准备 SSL 证书（Let's Encrypt 推荐）
2. 修改 `docker-compose.yml`：

```yaml
services:
  frontend:
    ports:
      - "443:443"
    volumes:
      - ./ssl:/etc/nginx/ssl:ro
      - ./nginx-https.conf:/etc/nginx/conf.d/default.conf:ro
```

3. 创建 `nginx-https.conf`：

```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com;

    ssl_certificate /etc/nginx/ssl/fullchain.pem;
    ssl_certificate_key /etc/nginx/ssl/privkey.pem;

    # ... 其他配置同 nginx-frontend.conf
}
```

### 性能优化

**增加工作进程数**（多核 CPU）：
```nginx
# nginx.conf
worker_processes auto;
worker_connections 2048;
```

**后端并发配置**：
```toml
# config.toml
[server]
max_connections = 100
read_timeout = "60s"
write_timeout = "60s"
```

---

## 📊 监控与日志

### 查看实时日志
```bash
# 所有服务
docker-compose logs -f

# 特定服务
docker-compose logs -f ytb2bili
docker-compose logs -f frontend
```

### 资源使用情况
```bash
docker stats
```

### 健康检查
```bash
# 后端健康检查
curl http://localhost:8096/health

# 前端健康检查
curl http://localhost/health
```

---

## 🔄 更新与维护

### 更新应用版本
```bash
# 拉取最新代码
git pull origin main

# 重新构建并启动
docker-compose down
docker-compose up -d --build
```

### 清理旧镜像
```bash
docker system prune -a
```

---

## 📝 变更记录

### PR #1 - Docker 重构（2025-01-23）

**新增功能**：
- ✅ SQLite 支持（CGO 编译）
- ✅ 前后端分离架构
- ✅ Nginx 反向代理
- ✅ 独立前端容器

**迁移指南**：
旧版本用户无需修改配置，新架构完全向后兼容。如需使用 SQLite：
1. 修改 `config.toml` 中 `database.type = "sqlite"`
2. 重启服务：`docker-compose restart ytb2bili`

---

## 📞 技术支持

- GitHub Issues: https://github.com/difyz9/ytb2bili/issues
- 文档: https://github.com/difyz9/ytb2bili
- 贡献指南: CONTRIBUTING.md

---

**最后更新**: 2025-01-23  
**维护者**: @difyz9
