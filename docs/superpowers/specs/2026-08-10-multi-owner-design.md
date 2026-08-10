# 任务与项目多负责人(multi-owner)设计

> 日期:2026-08-10 · 状态:已确认 · 版本:应用于 v1.8.10(用户测试阶段)

## 背景与目标

现系统中任务的负责人(`tasks.assignee`)与项目的负责人(`projects.owner`)都是**单值 TEXT 列**,存显示名或邮箱,靠文本匹配关联用户。实际场景中一个任务常由多人负责(如 A 与 B 协作),项目也可能有多个负责人。本设计将两处负责人升级为**多负责人**,并补齐对应的视图、导入、提醒与防呆能力。

**用户拍板的关键决策:**

| 决策点 | 结论 |
|--------|------|
| 编辑权限 | 保持现状:任何登录用户可编辑任何任务(任务 owner 可编辑是其子集),不引入权限体系 |
| 数据模型 | 关联表:`task_assignees` + `project_owners`(user_id 外键,根治重名/改名/删号悬空) |
| 我的待办 | 现有结构不变,加**视角过滤**:`view=task`(我名下的任务)/ `view=project`(我名下项目的任务) |
| CSV 多值分隔 | 分号 `;` |
| 弹窗交互 | 多选下拉 + 已选标签(可点 X 移除),重复选择自动去重 |
| 行内/批量条 | 负责人编辑改为只读展示,编辑统一走详情弹窗 |
| 看板卡片 | 两行 → **三行**:第二行独立显示 owner(分号分隔) |

## 1. 数据模型

```sql
CREATE TABLE IF NOT EXISTS task_assignees (
  task_id    INTEGER NOT NULL,
  user_id    INTEGER NOT NULL,
  PRIMARY KEY (task_id, user_id)          -- 主键天然去重
);

CREATE TABLE IF NOT EXISTS project_owners (
  project_id INTEGER NOT NULL,
  user_id    INTEGER NOT NULL,
  PRIMARY KEY (project_id, user_id)
);
```

- **权威数据是关联表**;`tasks.assignee` / `projects.owner` 列**保留但不维护**(迁移时一次性写入分号分隔显示名作为最终快照,之后写路径不再触碰),所有读路径改走关联表 JOIN 拼最新显示名。理由:权威单一,避免双写失配。
- 无外键约束(与项目现有 SQLite 风格一致),级联清理由应用层负责。

### 迁移(启动时自动执行,幂等)

1. 对 `tasks.assignee` 非空且 `task_assignees` 中无此任务记录的行:按 `email` 精确匹配活跃用户 → 其次 `display_name` 精确匹配(重名取 id 最小者)→ 都失败则跳过(该任务归未分配,不丢其他字段)。
2. 对 `projects.owner` 执行同规则,写入 `project_owners`。
3. 迁移完成后写回快照列:`UPDATE tasks SET assignee = <分号分隔的最新 display_name>`(仅迁移过的行),`projects.owner` 同理。
4. 全流程幂等:重复启动不重复插入(写前检查)。

### 级联与复制

- 删除任务 → 删 `task_assignees` 该任务行
- 删除项目 → 删 `task_assignees`(该项目所有任务)+ `project_owners`
- 复制项目(CopyProject)→ 复制任务关联表 + 项目关联表(新 user_id 原样带)

## 2. API

### 请求体(写接口)

- `CreateTask` / `UpdateTask` 接受 `assignee_ids: []int64`(新前端传);兼容旧 `assignee` 字符串(分号/逗号分隔的显示名或邮箱,脚本兼容)。统一解析为 `[]int64` 后**去重**写入关联表。
- `CreateProject` / `UpdateProject` 接受 `owner_ids: []int64`;兼容旧 `owner` 文本(含分号/逗号分隔)。
- 校验:每个 id 必须存在且 `is_active=1`,否则 400(错误码沿用 INVALID_OWNER 语义,提示具体人名);文本解析失败同样 400。允许空集合(现状 owner 可为空)。
- 乐观锁 `version` 语义不变。

### 响应体(读接口)

- 任务对象新增 `assignee_ids: []int64`;`assignee` 字段 = 关联表 JOIN users 现拼的分号分隔显示名(如 `张三;李四`),停用/已删用户仍显示(LEFT JOIN),用户不存在则跳过该名字。
- 项目对象新增 `owner_ids: []int64`;`owner` 字段同上规则。
- 实现方式:子查询 `(SELECT GROUP_CONCAT(u.display_name, '; ') FROM task_assignees ta JOIN users u ON u.id=ta.user_id WHERE ta.task_id=t.id)`;项目同构。单项目任务量级下性能无虞。

### GetMyTasks 视角

`GET /api/tasks/mine?view=task|project&days=N`

| 视角 | mine(进行中) | starting(未来 N 天开始) |
|------|--------------|--------------------------|
| `task`(默认) | JOIN `task_assignees` 我名下任务的进行中任务 | 我名下任务的未完成且开始日期在窗口内 |
| `project` | JOIN `project_owners` 我名下项目的进行中任务(不管任务 owner) | 我名下项目的未完成且开始日期在窗口内 |

- 行为变化:原 `starting` 不过滤负责人(显示全项目所有人的任务),现按视角过滤——这是用户明确要的"看到名下任务的状态"。
- 两个视角都返回 `mine` + `starting` 两个分区,前端结构不变。

### 到期提醒

`reminder.go` 改为 JOIN `task_assignees` + `users`(email 精确,不再文本匹配),每个活跃负责人各收一封;同一用户同一任务天然只一封(JOIN 去重)。

## 3. 前端

### 复用组件 `components/MultiUserSelect.tsx`(新建)

- Props:`users: {id, display_name}[]`、`selectedIds: number[]`、`onChange(ids: number[])`、placeholder
- 交互:上方标签区(每个已选用户一个标签 + × 移除);下方下拉展开列表,点击用户 toggle 勾选(重复点击=取消),去重由状态保证
- 任务详情弹窗与项目创建/详情弹窗共用

### 看板卡片(三行布局)

- 第一行:状态点 + `#编号` + 项目名 + 延迟徽章 + 复制/详情/删除(owner 撤走)
- 第二行:`👤` + owner 分号分隔文本,超长省略号 + `title` 全名
- 第三行:进度条 + 百分比 + 截止/预计日期(现 body 原样下沉)
- CSS:`.project-owner` 从固定 90px 改弹性布局

### 我的待办

- 区块顶部加两个视角标签:「我负责的任务 / 我负责的项目」,点击切换后重新拉取 `?view=...`
- 默认 `task` 视角

### 资源视图(Resources.tsx)

- 按 `assignee` 文本以 `;` 拆分,多 owner 任务在**每个 owner 分组下重复出现**(工时各计各的);无 owner 归「未分配」

### 甘特图(ProjectGantt.tsx)

- assignee 列:显示分号文本(列宽略加或省略号 + title)
- tooltip:`负责人: 张三;李四`
- 过滤条:ownerFilter 选项 = 全任务负责人并集(拆分);过滤逻辑改为**任一 owner 匹配**即显示
- 批量条:移除 assignee 项(改只读)

### 任务列表(TaskListView.tsx)

- assignee 单元格改**只读显示**(分号文本),移除行内编辑(现 313-329 行的 startEdit/editingCell 逻辑);编辑走详情弹窗

### 项目创建/详情弹窗

- owner 单选下拉 → `MultiUserSelect`(创建弹窗与 ProjectDetail 均替换)
- 保存传 `owner_ids`

### CSV 导入

- 负责人列按 `;` 拆分,逐名解析(display_name 或 email 匹配活跃用户),去重
- 解析失败:该任务负责人归未分配,**不跳过任务**,在导入结果 errors 中提示「负责人『XX』不是系统用户,已归未分配」
- 英文 CSV 模板的负责人列示例值同步更新

## 4. 边界与防呆

| 场景 | 处理 |
|------|------|
| 重复选择同一用户 | 关联表主键兜底 + 前端 toggle 去重 |
| 存量文本解析失败(用户已删/停用) | 归未分配/空,不丢任务其他字段 |
| 用户停用 | 下拉不再可选;已分配的任务展示保留(LEFT JOIN);到期提醒只发活跃用户 |
| 用户被删除 | 关联表行保留,展示时 JOIN 不到则跳过该名字 |
| 用户改名 | 展示走 JOIN 最新显示名,自动更新,无悬空 |
| 旧脚本传 `assignee`/`owner` 文本 | 写接口兼容解析,不破坏 |
| 空负责人集合 | 允许(现状 owner 可为空) |

## 5. 测试

- **后端**:关联表 CRUD(增删改查+去重)、迁移幂等(存量文本→关联表)、`GetMyTasks` 双视角(含 starting 过滤)、CSV 多值+解析失败提示、项目/任务校验 400、CopyProject 带负责人、删除级联、提醒 JOIN
- **前端**:`npm run build` 类型通过;浏览器实测——任务弹窗多选、项目弹窗多选、看板三行卡片、我的待办双视角、资源视图重复分组、甘特图过滤任一匹配、CSV 分号导入

## 6. 已知限制(本次不做)

- `project_members` 表的 role 体系暂不参与编辑权限(权限保持登录即可编辑);项目多 owner 时仅顺带 `INSERT OR IGNORE` 进 project_members(role=owner),为将来权限留口子
- 项目 owner 的历史悬空(用户改名/删号)由关联表根治,存量快照列中的旧名不再更新
