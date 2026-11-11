package auth

import "go.uber.org/fx"

// Module 保留认证能力的组装点。具体会话存储由账号模块提供。
var Module = fx.Module("auth")
