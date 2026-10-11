# TODO

> 生成于 2026-10-11（无 todo 文件；来源：docs/project-plan.md 全文 **0 个未完成 checkbox**，Phase 4 愿景 14 项全部 [x]，近期走 PR 流 #7-#20 已合）。

## 待用户审核（本仓疑似已清空，需人工确认后决定是否重新派活）
- [ ] 用户逐仓审核：确认 Phase 1 / Phase 2 是否存在 project-plan 之外的未落地项（本轮自动扫描未发现未完成 checkbox）
- [ ] 审核通过后由用户分配下一批任务，再重新加入监控

## 维护项（常规）
- [ ] 依赖与安全告警按需处理（dependabot；code scanning 见下条，暂不启用）
- [ ] 每批 PR 合并后 README/文档站同步

最近一次分诊（2026-10-11）：dependabot 开放告警 0；npm audit 两棵树 0 漏洞；
govulncheck 可达漏洞清零——工具链钉到 go1.27.2 修掉 10 个 stdlib 漏洞，
仅剩 x/crypto 的 openpgp 模块级公告（未导入该子包，Fixed: N/A，常驻噪音）；
已合 PR #21 无用户可见 API 变化，README 无需同步。

## 暂不开（等用户明令）
- [ ] CodeQL / code scanning：暂不开。无用户明令，且会引入持续告警分诊面，按默认从简口径挂起。

## 等外部密钥/凭据（暂挂）
- [ ] Web Push 公钥端点：推送通道与订阅端点已就绪，SPA 还需服务端 VAPID 公钥才能订阅；等外部密钥。
- [ ] Stripe 支付渠道（D24）：等凭据。

验收：以用户审核结论为准；新任务由用户分配后按测试绿才 push 执行。
