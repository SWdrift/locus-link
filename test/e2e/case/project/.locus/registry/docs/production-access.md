# 生产访问

Shell Route 在执行 SSH safe Probe 前复用现有 FRP endpoint。

## 验证

先 Resolve；仅在 evidence 需要刷新时 Probe；然后再次 Resolve，检查更新后的 evidence。
