# 接口

普通业务接口都是 `POST /api/...`，请求和响应为 JSON。成功和业务错误的 HTTP 状态都是 200。分页成功没有 `code`/`msg`。健康检查使用真实的 200 或 503。

标识用十进制字符串，例如 `"id":"1"`。时间是 UTC RFC3339。

## 账号

| 路径 | 认证 | 说明 |
| --- | --- | --- |
| /api/account/register | 否 | 注册并创建会话 |
| /api/account/login | 否 | 登录 |
| /api/account/logout | 是 | 吊销当前会话 |
| /api/account/me | 是 | 当前用户 |

注册和登录成功后设置 HttpOnly、SameSite=Lax 的 `yunjian_session`。响应正文不返回令牌。用户名保存为小写。密码错误和用户不存在使用同一句“用户名或密码错误”。

## 文档

以下接口都需要登录，并且只访问当前用户的数据。

| 路径 | 说明 |
| --- | --- |
| /api/document/create | 创建空文稿、表格或演示，并生成版本 1 |
| /api/document/update | 只修改标题，修订号加一，不新增内容版本 |
| /api/document/one | 详情，包含当前正文 |
| /api/document/search | 分页。`kind`、`title` 可选，标题是包含匹配 |
| /api/document/delete | 硬删除文档和全部版本 |
| /api/document/save | 保存正文。内容、标题或非空说明变化时新增版本 |
| /api/document/version/search | 某个文档的版本分页，不含正文 |
| /api/document/version/one | 一个历史版本的正文 |
| /api/document/version/restore | 把历史版本恢复成新的当前版本 |

保存和恢复必须携带当前 `revision`。不匹配时返回应用码 `10004`。用户名重复返回 `10001`。

分页默认第 1 页、每页 20 条，最大 200。JSON 无法区分省略和 0，因此 0 视为默认值；负数和超限返回 400。排序只允许 `updated_at`、`created_at`、`title` 和 `asc`/`desc`。

## 正文

`kind` 为 `word`、`excel` 或 `ppt`。服务端拒绝未知字段，并重新序列化后保存。

文稿是段落数组，段落类型为 `paragraph`、`heading1`、`heading2`、`bullet`、`numbered`，文本片段可以加粗、斜体、下划线。表格是工作簿：最多 20 个工作表，每表最多 120 行、30 列。旧的单表字段 `sheet_name`、`row_count`、`col_count`、`cells` 会规范成 `sheets`。单元格保存文本或以 `=` 开头的公式，并可带加粗、斜体、下划线、对齐、颜色、填充和数字格式。演示画布是 960×540，元素目前为文本块。

## 其他

未知的 `/api` 或 `/health` 路径返回 `{"code":404,"msg":"接口不存在"}`。页面路径回退到 `index.html`。缺失的 JS、CSS 和图片返回 HTTP 404。
