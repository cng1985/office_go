# 部署

## 环境变量

定位参数：`APP_BASE_DIR`、`APP_CONFIG_DIR`、`APP_DATA_DIR`、`APP_PROFILE`。

| 变量 | 配置键 |
| --- | --- |
| APP_NAME | app.name |
| APP_SERVER_HOST | server.host |
| APP_SERVER_PORT | server.port |
| APP_SERVER_READ_HEADER_TIMEOUT | server.read_header_timeout |
| APP_SERVER_IDLE_TIMEOUT | server.idle_timeout |
| APP_SERVER_MAX_HEADER_BYTES | server.max_header_bytes |
| APP_SERVER_SHUTDOWN_TIMEOUT | server.shutdown_timeout |
| APP_SERVER_REQUEST_TIMEOUT | server.request_timeout |
| APP_SERVER_SSE_TIMEOUT | server.sse_timeout |
| APP_SERVER_MAX_BODY_BYTES | server.max_body_bytes |
| APP_SERVER_TRUSTED_PROXIES | server.trusted_proxies，JSON 字符串数组 |
| APP_DATABASE_ENABLED | database.enabled |
| APP_DATABASE_DRIVER | database.driver |
| APP_DATABASE_DSN | database.dsn，敏感；显式空值失败 |
| APP_DATABASE_MAX_OPEN_CONNS | database.max_open_conns |
| APP_DATABASE_MAX_IDLE_CONNS | database.max_idle_conns |
| APP_DATABASE_CONN_MAX_LIFETIME | database.conn_max_lifetime |
| APP_DATABASE_BATCH_SIZE | database.batch_size |
| APP_DATABASE_SQLITE_PATH | database.sqlite.path |
| APP_DATABASE_SQLITE_FOREIGN_KEYS | database.sqlite.foreign_keys |
| APP_DATABASE_SQLITE_BUSY_TIMEOUT | database.sqlite.busy_timeout |
| APP_DATABASE_SQLITE_JOURNAL_MODE | database.sqlite.journal_mode |
| APP_AUTH_ENABLED | auth.enabled |
| APP_AUTH_SESSION_TTL | auth.session_ttl |
| APP_AUTH_COOKIE_SECURE | auth.cookie_secure |
| APP_LOGGING_LEVEL | logging.level |
| APP_LOGGING_FORMAT | logging.format |

布尔值只接受 `true` 或 `false`。时长必须带单位，例如 `5s`、`8h`。未设置的变量不覆盖 YAML；设置为空字符串会按空值解析并接受后续校验。

## 进程

发布包中的二进制包含版本、提交号和构建时间。Linux 服务以非 root 用户运行，工作目录 `/opt/yunjian`，数据目录 `/var/lib/yunjian`。

关闭时按 Fx 生命周期停止 HTTP，等待 `server.shutdown_timeout`，然后关闭数据库。

## 备份

停写后复制 SQLite 文件，或对打开的库执行 `VACUUM INTO`。恢复时替换数据目录中的 `app.db`，再启动服务并检查 `/health/ready`。MySQL、PostgreSQL 和 SQL Server 使用各自的原生备份工具。
