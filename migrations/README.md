# 迁移

`0001_init` 创建账号、会话、文档和文档版本表。执行记录写入 `schema_migrations`。

| 方言文件 | driver |
| --- | --- |
| 0001_init.sqlite.sql | sqlite |
| 0001_init.mysql.sql | mysql |
| 0001_init.postgres.sql | postgres / pgsql |
| 0001_init.sqlserver.sql | sqlserver |

SQLite 在单连接事务中执行，失败会回滚。MySQL 的部分 DDL 会隐式提交，不能把整份脚本视为一个数据库事务；启动失败后需要按 `schema_migrations` 是否写入来判断能否重试。PostgreSQL 与 SQL Server 脚本按语句顺序执行，并使用数据库锁避免多个实例同时迁移。

后续版本同样为四种方言各准备一份脚本，例如 `0004_calendar`。

当前自动化测试覆盖 SQLite。另外三种数据库的脚本随发布包提供，尚未在对应引擎上执行兼容测试。
