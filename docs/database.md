# 数据库

迁移版本按顺序是 `0001_init`、`0002_folder`、`0003_team`、`0004_calendar`。执行记录写入 `schema_migrations`。SQLite 在 `BEGIN IMMEDIATE` 事务中执行，失败回滚。MySQL 的 DDL 可能隐式提交，PostgreSQL 和 SQL Server 脚本使用迁移锁；这三种数据库还没有实际执行记录。

| 表 | 所有权 | 说明 |
| --- | --- | --- |
| account_user | 账号 | 用户名唯一 |
| account_session | 账号 | 只存会话令牌的 SHA-256 |
| office_document | 文档 | 当前版本、修订号、所有者 |
| office_document_version | 文档 | `(document_id, version_no)` 唯一，正文追加写入 |
| calendar_event | 日历 | 当前用户的日程。全天用 `start_on`/`end_on`，定时用 `starts_at`/`ends_at` |

`office_document.current_version_id` 不建外键，避免文档和首个版本互相引用。版本表引用文档和作者。SQLite 连接打开外键，非法版本引用会失败。

标识是正整数，接口以字符串传输。时间由服务端写入 UTC。文档删除是硬删除。

标题筛选使用参数绑定的 `LIKE ... ESCAPE`，会转义 `%`、`_` 和反斜杠。排序列来自白名单。
