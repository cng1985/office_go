# 架构

云笺是模块化单体。`cmd/server` 只调用 `app.Run()`。`internal/app` 组合配置、数据库、中间件、HTTP、账号、文档、日历和嵌入的前端。

```text
浏览器 Vue 3
    │  POST /api + HttpOnly Cookie
Gin Controller
    │  Request / Search
Resource
    │  业务规则、所有者范围
Repository
    │  事务
SQLite / MySQL / PostgreSQL / SQL Server
```

账号、文档和日历是三个叶子业务。日历只保存当前用户自己的日程，不和文档或团队共用事务。文稿、表格、演示和 PDF 是文档的 `kind`，不是独立业务模块。版本历史属于文档聚合：保存和恢复都要同时修改当前文档和版本表，因此放在同一个 Repository 事务里，不拆成需要跨模块事务的子模块。

平台不引用业务包。认证中间件只依赖 `auth.Authenticator`，由账号 Repository 实现。前端静态资源由 `web.RegisterStatic` 注册，不负责监听端口。

删除策略是硬删除。删除文档时在同一事务中先删版本，再删文档。重复删除返回不存在。
