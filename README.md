# 云笺

云笺是一个按 Go 项目结构规范实现的在线办公应用。用户可以注册登录，在浏览器里编辑文稿、表格和演示，把内容保存到自己的账号下，并查看、恢复历史版本。

## 启动

要求 Go 1.22 和 Node.js 22。前端产物已经包含在 `web/dist`，可以直接启动：

```bash
go run ./cmd/server
```

默认监听 `127.0.0.1:8080`，数据库文件是 `data/app.db`。浏览器打开该地址后注册账号即可使用。

开发前端时另开一个终端：

```bash
cd web
npm ci
npm run dev
```

Vite 会把 `/api` 和 `/health` 代理到 `127.0.0.1:8080`。

```bash
go run ./cmd/server --version
```

## 配置

所有运行配置都在 `configs/`：

| 文件 | 作用 |
| --- | --- |
| application.yaml | 基础配置 |
| application-dev.yaml | `--profile dev` 或 `APP_PROFILE=dev` |
| application-pro.yaml | `--profile pro` 或 `APP_PROFILE=pro` |

启动定位参数先于 YAML 生效：`--base-dir`、`--config-dir`、`--data-dir`、`--profile`，对应环境变量 `APP_BASE_DIR`、`APP_CONFIG_DIR`、`APP_DATA_DIR`、`APP_PROFILE`。这四项不能写进 YAML。其余字段按“基础配置 < profile 配置 < 环境变量”覆盖。命令行不接收数据库连接串，避免出现在进程列表里。

网络数据库必须设置 `APP_DATABASE_DSN`，并在 profile 中把 `database.driver` 改成 `mysql`、`postgres`、`pgsql` 或 `sqlserver`。显式把 DSN 设为空字符串会启动失败。未知配置项、未知 profile、缺失的 profile 文件也会启动失败。

本项目把普通请求体上限提高到 8MiB，因为文档正文以 JSON 提交；业务层仍拒绝超过 1MiB 的规范化正文。登录 Cookie 默认不是 Secure，便于本机 HTTP。放到 HTTPS 后面时设置 `APP_AUTH_COOKIE_SECURE=true`。

环境变量名称见 `docs/deployment.md`。

## 数据库支持

| 数据库 | driver | 验证情况 |
| --- | --- | --- |
| SQLite | sqlite | 已用纯 Go 驱动 `github.com/glebarez/sqlite` 跑通迁移、外键、CRUD、事务回滚和备份 |
| MySQL | mysql | 已提供方言脚本和连接代码，尚未在 MySQL 上执行兼容测试 |
| PostgreSQL | postgres / pgsql | 已提供方言脚本和连接代码，尚未执行兼容测试 |
| SQL Server | sqlserver | 已提供方言脚本和连接代码，尚未执行兼容测试 |

SQLite 默认关闭 WAL，使用 DELETE 日志。外键和忙等待通过每条连接的 DSN pragma 设置。交叉编译保持 `CGO_ENABLED=0`。

## 构建

Windows：

```bat
scripts\build-platforms.cmd
```

已安装 PowerShell 7 的环境：

```bash
pwsh scripts/build-platforms.ps1
```

脚本从自身位置找到仓库根目录，执行 `npm ci`、前端测试和构建，然后以 `CGO_ENABLED=0` 编译 `windows/amd64`、`linux/amd64`、`linux/arm64`。产物在 `release/<os>-<arch>/`，包含二进制、配置模板、迁移脚本、安装脚本、版本说明和 SHA256。不包含数据库、上传文件或密钥。

## 安装

在 Linux 上用 root 执行发布目录中的 `install-linux.sh`。程序安装到 `/opt/yunjian`，数据目录是 `/var/lib/yunjian`，服务用户是 `yunjian`，profile 为 `pro`。更新时先停服务，再原子替换二进制，并保留已有配置和数据。

```bash
systemctl status yunjian
journalctl -u yunjian
```

数据库迁移成功后，不能假定换回旧二进制就会回滚表结构。回退前需要确认迁移是否兼容旧程序。

## 日志、健康和备份

日志默认写到标准输出。开发配置是文本，生产 profile 是 JSON。每条请求记录请求 ID、用户、路由模板、耗时、应用码和错误分类，不记录密码、会话令牌或正文。

| 端点 | 含义 |
| --- | --- |
| GET /health/live | 进程存活，HTTP 200 |
| GET /health/ready | 迁移完成且数据库可连接，失败时 HTTP 503 |

SQLite 使用 DELETE 日志时，停服务后可以复制 `app.db`。在线一致性备份使用 `VACUUM INTO`，`TestBackupSQLiteRoundTrip` 会创建副本并重新打开核对数据。不要在 WAL 模式下只复制主数据库文件；本项目默认不用 WAL。

## 验证

```bash
go test ./...
cd web && npm test && npm run build
```

手工验证：注册、登录、新建三种文档、编辑、保存、查看版本、恢复版本、另一个账号不能打开前者的文档。
