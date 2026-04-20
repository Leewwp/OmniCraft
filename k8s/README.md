# K8s Configuration (P2 阶段使用)

此目录预留用于 P2 阶段（用户规模 > 10,000）迁移到 Kubernetes（阿里云 ACK）时使用。

## 迁移路线

1. 所有服务已通过 Docker Compose 容器化，镜像可直接复用
2. docker-compose.yml 中每个服务对应一个 K8s Deployment + Service
3. 数据库迁移：pg_dump 导出 → 云托管 RDS PostgreSQL 导入（停机窗口 < 30 分钟）
4. 域名/DNS 切换：Nginx 容器替换为 Ingress Controller

## 计划文件结构

```
k8s/
├── namespace.yaml
├── backend/
│   ├── deployment.yaml
│   ├── service.yaml
│   └── configmap.yaml
├── frontend/
│   ├── deployment.yaml
│   └── service.yaml
├── postgres/
│   └── (使用阿里云 RDS，无需自部署)
├── redis/
│   ├── deployment.yaml
│   └── service.yaml
└── ingress.yaml
```

## 当前阶段

P0：使用 `docker-compose.yml` 单机部署。
