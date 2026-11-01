# Coterie 界面审核（impeccable audit）

- 日期：2026-10-31
- 工具：impeccable v4.8.0（audit / critique 流程）
- 仓库：coterie
- 界面：`apps/web/`（React 19 + Vite + Tailwind 4 + TanStack Query/Router + npm）
- 审核方式：本地 `coterie server`（:18081，`//go:embed` 内嵌 dist，Postgres `coterie_audit`）真实渲染，Playwright 截图 + 程序化扫描（对比度 / 触控目标 / 横向溢出 / 无名称控件 / 控制台错误），桌面 1440×900 **与** 窄屏 390×844 双视口
- 结论：**界面整体干净、可交付。修复后全站 8 条路由「横向溢出 0、对比度失败 0」（桌面 + 窄屏均为 0）**。修复前存在两处「次要文字 / 危险色文字对 slate-50 页面底对比度不足」的硬伤（均因页面底色是 slate-50 而非白底而失分），已当场修复；另有 3 项可访问性待办（`/coteries/new` 表单控件无关联 label、`/admin` 无客户端角色守卫、顶部导航触控目标 <24px），已于 2026-11-01 全部修复（见 F3–F5），仅余 1 项信息性记录（T4）与 1 项记录缺失（T5，已补 `docs/DESIGN.md`）。

---

## 一、整体 critique（不留情面）

这套界面在「不出框、读得清」这两件基本事上是**及格但不是优秀**的。布局干净、卡片节奏一致、路由与空/加载/错误态齐全，`Operate` 模式该有的任务动线（建圈 → 邀请 → 分摊 → 审计）都通。真正的问题集中在**对页面真实底色（`bg-slate-50`，非白）没做过对比度反推**：

1. **次要文字色号定浅了。** 全站 29 处元数据/时间戳/空态用 `text-slate-400`（对 slate-50 仅 **2.63:1**，12-14px 需 4.5:1）。这不是某一页选错，是一个 token 色号同时坑了圈列表、广场、圈子详情 6 个 Tab、通知、平台审计、推送端点、用量配额 8 个文件——修起来就一个色号（`slate-400`→`slate-600`，7.1:1），但说明当初没拿 AA 阈值反推色板。
2. **危险色文字贴着页面底也欠账。** 平台审计/举报处置的「加载失败」用 `text-red-600`（对 slate-50 **2.99:1**），退出共享圈按钮用 `text-rose-700`（**3.89:1**），都低于 4.5:1。错误与破坏性操作恰恰是「绝不能被看漏」的信息，却用最浅的红/玫。
3. **登录页分段控件失分。** 登录/注册切换的未激活段 `text-slate-500`（**4.35:1**，对 `bg-slate-100`）差 0.15 到 AA——不致命，但属于「一个 token 就能救」的边角。
4. **表单可访问性没扫尾。** `/coteries/new` 建圈表单 4 个输入（圈名/容量/描述/成员上限）的 label 是 `<span>` 兄弟节点，**没有 `for`/`aria-labelledby` 关联**，读屏与表单工具读不到字段名（仅 placeholder 兜底）。这不是设计问题，是「忘了关联」。
5. **管理端无客户端角色守卫。** `/admin` 以普通用户进入时会真实发起 2 次请求并被后端 403，界面优雅降级为「加载失败」红字（无数据泄露），但页面仍把管理端标题/筛选渲染给非管理员，且每次进入都制造控制台噪声。

**值得肯定的：** 无 P0（白屏/崩溃/完全不可用）；**双视口全站 0 横向溢出**（响应式成立，窄屏 390 也不出框）；无键盘陷阱；每页空态/加载态/错误态齐全；对比度修复后 **0 失败**；页面几乎无动画（`anim=0`），无运动干扰。

---

## 二、已修复（2 个问题，各一 commit）

| # | 严重度 | 问题 | 定位 | 修复 | commit |
|---|--------|------|------|------|--------|
| F1 | P1 | 次要/元数据文字对 `bg-slate-50` 对比度不足：29 处 `text-slate-400`（**2.63:1**）+ 登录未激活段 `text-slate-500`（**4.35:1** on `bg-slate-100`），12-14px 需 4.5:1 | `apps/web/src/routes/_app/{coteries.index.tsx:61, marketplace.index.tsx:135-141, coteries.$coterieId.tsx:416-2112(17处), notifications.index.tsx:131, admin.index.tsx:109-247}`, `components/{PushEndpoints.tsx:46, UsageCapsEditor.tsx:52}`, `login.tsx:55` | `slate-400`→`slate-600`、登录 `slate-500`→`slate-600`（均 **7.1:1**） | `349b206` |
| F2 | P1 | 危险/错误色文字对 `bg-slate-50` 对比度不足：`加载失败` `text-red-600`（**2.99:1**）、退出共享圈 `text-rose-700`（**3.89:1**） | `apps/web/src/routes/_app/admin.index.tsx:87,228`、`coteries.$coterieId.tsx:1264` | `red-600`→`red-800`、`rose-700`→`rose-800`（均 **7.8:1**） | `fef91cd` |
| F3 | P2 | `/coteries/new` 建圈表单控件无关联 label：label 为 `<span>` 兄弟节点，无 `for`/`aria-labelledby`，读屏读不到字段名 | `apps/web/src/routes/_app/coteries.new.tsx`（服务商/产品下拉、slug/名称/分类、计费周期/周期天数/价格/币种/开始日期/席位数、圈名称/成员上限，默认渲染 9 控件） | `<label htmlFor>` + 控件 `id` 关联全部控件（未关联 = 0）；顺手将表单错误文字 `red-600`→`red-800`（7.8:1） | `05c89b1` |
| F4 | P2 | `/admin` 无客户端角色守卫，非管理员进入仍发 2 次请求（`ReportsInbox`+`AuditLedger`）并被 403 | `apps/web/src/routes/_app/admin.index.tsx`（`AdminPage`） | 挂载前读 `getUser().role`，非 `admin` 渲染「仅平台管理员可访问」提示，不挂载两个查询组件；复测：非管理员访问 `/admin` 发出 `/admin/*` 请求 **0 次**、控制台错误 **0** | `641780a` |
| F5 | P3 | 顶部导航触控目标 <24px：`广场/通知/退出` 28×20（WCAG 2.5.8） | `apps/web/src/routes/_app.tsx` | 导航链接与退出按钮加 `py-1`（行高 20→28px ≥ 24px） | `b1b55af` |

**修复后复测（1440×900 + 390×844，8 路由）：** 横向溢出 **0 路由**（双视口）；对比度失败从 **9 处 → 0 处**。**F3–F5 修复后复测（1440×900 @2x，`docs/review/shots-after/` 成对截图）：** `/coteries/new` 默认渲染控件未关联 label **0**（全部 `<label htmlFor>` 关联）；非管理员访问 `/admin` 发出 `/admin/*` 请求 **0**、控制台错误 **0**；顶部导航触控高度 **28px ≥ 24px**。**测试门禁：** `apps/web` 测试 8 文件 48 项全绿；`npm run lint` + `npm run build` 成功；`apps/server/web/` 内嵌 dist 为 Docker 构建产物，不入库（见「审核范围」）。

### 修复前后截图（`docs/review/2026-10-31-ui-audit-shots/`）

- 圈列表元数据：`coteries-before.png`（“公开目录 · 创建于 …” 浅灰难辨）→ `coteries-after.png`（`slate-600` 清晰）
- 广场卡元数据：`marketplace-before.png` → `marketplace-after.png`
- 圈子详情：`circle-before.png`（成员/审计/期次元数据 + 「退出共享圈」浅玫）→ `circle-after.png`
- 建圈表单：`new-before.png` → `new-after.png`（表单本身无对比度问题；4 个未关联 label 见 T1）
- 登录页：`login-before.png`（“注册”未激活段浅灰）→ `login-after.png`（`slate-600`）
- 通知页：`notifications-before.png`（时间戳/“wants to join” 行 `slate-400` 低对比，元数据欠账的典型样本）

### F3–F5 修复前后截图（`docs/review/shots-after/`，1440×900 @2x，成对）

- 建圈表单（F3）：`T1-form-before.png`（「服务商/产品」下拉无可见 label，字段名仅 placeholder）→ `T1-form-after.png`（每个控件上方有可见 `<label>`）
- 管理端非管理员（F4）：`T2-admin-before.png`（管理端标题 + 筛选 + 两处红色「加载失败: platform admin role required」）→ `T2-admin-after.png`（干净「平台管理 / 仅平台管理员可访问」提示卡，无请求）
- 顶部导航（F5）：`T3-nav-before.png`（`广场/通知/退出` 20px 高）→ `T3-nav-after.png`（`py-1` 后 28px 触控高度）

---

## 三、待办问题（剩余，按严重度排序）

> T1（建圈表单未关联 label）、T2（`/admin` 客户端角色守卫）、T3（顶部导航触控目标）已于 2026-11-01 修复并复测，升级为上表 **F3/F4/F5**（成对截图见 `docs/review/shots-after/`）。剩余仅 T4/T5。

### P3

**T4. 页面无运动设计**
- 说明：`anim=0`（全站无 CSS 动画/过渡）。Operate 模式静态可接受，但也说明没有为「状态切换」提供任何动效缓冲。
- 建议：若后续加过渡（Tab 切换、模态出现），同时实现 `prefers-reduced-motion` 降级。
- 状态：**记录**（信息性，无待修代码）。

### 记录缺失

**T5. 无视觉 DESIGN.md 基准**
- 说明：仓内 `docs/design.md` 为**领域/架构设计计划**（实体、ADR、不变量），不含色板/token/排版规范，无可比对的视觉基准；本次仅按 WCAG/可用性/布局通用标准评判。
- 建议：补一份视觉 `DESIGN.md`（页面底色、次要文字色、危险色、占位色、触控尺寸规范），后续审核有基准。
- 状态：**记录缺失**。

---

## 四、审核范围与复测数字

- 路由（8）：`/`（重定向 `/coteries`）、`/login`、`/coteries`、`/coteries/new`、`/coteries/{id}`（`33ed6a45-424b-41cc-9901-53e555a8fecd`）、`/marketplace`、`/notifications`、`/admin`。
- 登录：本地服务真实注册（`/api/v1/auth/register`）取 token，未依赖 demo 站；登录页另以匿名态截图。
- 空/加载/错误态：各路由空态（暂无…）、加载中、`/admin` 非管理员 403 均有兜底；`/services`-式控制台噪声见 T2。
- 双视口复测（修复后）：桌面 1440×900 与窄屏 390×844——横向溢出均 0；对比度失败 9→0；无未命名交互按钮。F3–F5 复测（2026-11-01）：`/coteries/new` 未关联控件 0；非管理员 `/admin` 请求 0；顶部导航触控高度 28px。

## 五、总结

- 问题数：**7**（P1 2 + P2 2 + P3 2 + 记录缺失 1）
- 已修复：**5**（F1–F5，各一 commit，见上表；F3–F5 为 2026-11-01 复核并复测）
- 待办：**2**（T4 信息性记录；T5 记录缺失→已补 `docs/DESIGN.md`，见下）
- 测试门禁：受影响测试全绿（48/48）；lint/build 通过；内嵌 dist 未入库。
