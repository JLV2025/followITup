# Task 6 实施报告：甘特图基线绘制层 + 工具栏基线下拉

## 改动文件

| 文件 | 改动类型 | 说明 |
|------|---------|------|
| `frontend/src/pages/ProjectGantt.tsx` | 修改 | 基线绘制层(onGanttRender)、工具栏基线下拉、state 声明 |
| `frontend/src/styles/components.css` | 修改 | 基线下拉菜单样式（8条规则） |
| `frontend/src/stores/ganttStore.ts` | 修改 | fetchBaselineMeta 适配 API 字段映射 + task_count |
| `backend/internal/api/tasks.go` | 修改 | ListTasks SELECT 补全 baseline 四列 + Scan 参数 |

## 关键技术发现：addTaskLayer 不可用

dhtmlx-gantt v10.0.0 MIT Edition 在初始化阶段通过 `mo(e)` 函数删除 `addTaskLayer` 和 `addLinkLayer`（见 `dhtmlxgantt.es.js` 第 18317 行）。这些是 PRO 版特性，MIT 版注释明确列出"baselines"为 PRO 功能。

**解决方案**：改用 `gantt.attachEvent("onGanttRender", ...)` 回调，在每次渲染后遍历 `gantt.eachTask()` 手动创建 DOM 元素：
- 清除上一轮 `.baseline-layer-bar` / `.actual-layer-bar` 
- 为有 baseline 数据的任务在 `.gantt_task_row[task_id="N"]` 内添加基线条
- 为有 actual_start 的任务添加实际执行条

## 浏览器实测结果

### 环境
- 后端：localhost:8080 (Go build 无错误)
- 前端：localhost:3000 (Vite dev server, HMR)
- 测试项目：新房装修 (ID=6)，14 个任务

### 测试用例

| # | 操作 | 预期 | 结果 |
|---|------|------|------|
| 1 | 打开项目，点击"基线 ▾" | 下拉显示"创建基线（快照当前计划）" | 通过 |
| 2 | 点击"创建基线" | 工具栏按钮变为"基线 08-03 ▾"（has-baseline 高亮 #2C6E6A）；Gantt 渲染 14 条灰色基线条 | 通过 |
| 3 | 检查基线条 DOM | position:absolute, top:0px, height:4px, background:#6B7280 | 通过（14/14 任务） |
| 4 | PUT 任务 27 status→in_progress，刷新 | 任务 27 底边出现浅绿实际条 | 通过（1 条 actual-layer-bar） |
| 5 | 检查实际条 DOM | position:absolute, bottom:0px, height:4px, background:#86EFAC | 通过 |
| 6 | 再次打开基线菜单 | 显示创建时间/创建者/快照任务数 + "重新创建基线" + "清除基线" | 通过 |
| 7 | 点击"清除基线"→确认 | 按钮恢复"基线 ▾"，基线条消失 | 通过 |
| 8 | tsc --noEmit | 无错误 | 通过 |
| 9 | 浏览器 console | 0 errors | 通过 |

### 像素微调

- 使用 `top:0px` / `bottom:0px`，4px 高度细条紧贴任务条边缘
- dhtmlx-gantt 任务条在 28px 行内居中渲染（约 20px 高），4px 细条位于任务条边界内侧
- 浏览器实测无像素偏差，无需微调 top/bottom 值

## tsc 输出

```
cd frontend && npx tsc --noEmit
```
无错误，无警告。

## 自审结论

- 基线绘制层在 onGanttRender 中正确清除/重建，避免重复元素
- 工具栏下拉三种状态（无基线/有基线/下拉展开）均正常工作
- 实际执行条仅在 actual_start 非空时渲染（符合简报语义）
- 样式使用简报指定的设计系统颜色：#6B7280（基线条）、#86EFAC（实际条）、#2C6E6A（基线态按钮）
- 额外修复了 tasks.go ListTasks 漏选 baseline 列的 bug（Task 1 遗留问题）
- 额外修复了 ganttStore 字段映射（API 返回 baseline_created_at 而非 created_at）

## 风险/疑虑

1. **onGanttRender 性能**：每次 gantt 渲染都遍历全部任务 + DOM 操作。14 个任务无性能问题，但 >200 任务场景需要关注。后续可优化为只处理可视区域任务。
2. **addTaskLayer 丢失**：协作聚焦层（`drawFocus`）也依赖 addTaskLayer，该功能可能一直未生效。建议整体评估是否改用 onGanttRender 统一实现所有层。
3. **任务 27 冲突**：PUT 时版本号不匹配（version=0 vs 实际=1），需配合前端的版本管理逻辑。

---

## Fix Report: 基线层 top 定位修复（2026-08-03 10:50）

### 问题描述

初始实现将基线条 append 到 `.gantt_task_row` 内，top:0px 时恰好对齐行顶（紧贴任务条顶边）。后改为 append 到 `.gantt_bars_area`（共享容器），但 top 仍硬编码为 0px/bottom 0px，导致**所有基线/实际条堆叠在 bars_area 容器顶部**（所有条 top 相同）。

### 改动文件

| 文件 | 改动 | 行数 |
|------|------|------|
| `frontend/src/pages/ProjectGantt.tsx` | 基线条 top 改为 `line.offsetTop - 4`，实际条 top 改为 `line.offsetTop + line.offsetHeight` | 2 行 |

### 改动详情

**基线条**（第 351 行）：
```diff
- el.style.cssText = "... top:0px; ..."
+ // 基线条紧贴任务条顶边：任务条 offsetTop - 4px（任务条高 20px 在 28px 行内居中）
+ el.style.cssText = "... top:" + (line.offsetTop - 4) + "px; ..."
```

**实际执行条**（第 379 行）：
```diff
- el.style.cssText = "... bottom:0px; ..."
+ // 实际执行条紧贴任务条底边：任务条 offsetTop + offsetHeight
+ el.style.cssText = "... top:" + (line.offsetTop + line.offsetHeight) + "px; ..."
```

### 像素验证数据（Playwright 实测）

测试环境：localhost:3000/project/6（Vite dev server），新房装修项目。

| 条索引 | 关联任务 | barLeft | lineLeft | leftDiff | barTop | lineTop | topDiff | 判定 |
|--------|---------|---------|----------|----------|--------|---------|---------|------|
| 0 | 27 (汇总) | 536 | 536 | 0 | 229 | 233 | -4 | 通过 |
| 1 | 28 (水电改造) | 835 | 835 | 0 | 257 | 261 | -4 | 通过 |
| 2 | 29 (铺地暖) | 835 | 835 | 0 | 285 | 289 | -4 | 通过 |
| 3 | 47 (布电线) | 416 | 416 | 0 | 593 | 597 | -4 | 通过 |

**验收结论**：
1. baseline==current 的任务（28、29 号等）基线条 left == 任务条 left，偏差 0px（±1px 内通过）
2. 基线条 top 各自不同（229/257/285/593），不再全部堆叠第一行
3. topDiff 统一为 -4（基线条底边恰好紧贴任务条顶边）
4. 任务 27 恢复正常渲染
5. `./frontend/node_modules/.bin/tsc -p ./frontend/tsconfig.json --noEmit` 无错误

### 任务 27 恢复

任务 27 已通过排程引擎自动恢复 start_date/end_date 为 `2026-08-01` ~ `2026-08-03`（API 验证通过，version=4）。

### 构建结果

```
frontend: tsc -b && vite build -- OK (dist/index.html, dist/assets/index-CeqSSPYe.js)
Go exe:  go build -o followitup.exe ./cmd/server/ -- OK (followitup.exe 约 19MB)
嵌入验证: md5sum frontend/dist/index.html == backend/cmd/server/frontend-dist/index.html
```

### 提交

（见本次提交 SHA）

---

## Fix Report: 基线菜单外部点击关闭（2026-08-03 11:15）

### 问题描述

基线下拉菜单打开后，点击页面其他区域不会关闭，用户必须再次点击触发按钮才能收起。

### 改动文件

| 文件 | 改动 | 行数 |
|------|------|------|
| `frontend/src/pages/ProjectGantt.tsx` | 新增 useEffect 外部点击监听 + 菜单容器/按钮 stopPropagation | 9 行 |

### 改动详情

1. **useEffect 监听**（第 96-107 行）：`baselineMenuOpen` 为 true 时注册 `document` 级 click 监听，点击任意处关闭菜单；`baselineMenuOpen` 变为 false 时清理监听器。使用 `setTimeout(0)` 延迟注册，避免 toggle 按钮的同一 click 事件立即触发关闭。

2. **按钮 stopPropagation**（第 568 行）：toggle 按钮 onClick 添加 `e.stopPropagation()`，防止按钮点击被 document 监听器捕获。

3. **菜单容器 stopPropagation**（第 575 行）：`.baseline-menu` div 添加 `onClick={(e) => e.stopPropagation()}`，确保点击菜单内部（如"重新创建基线"按钮）不会冒泡到 document 导致菜单关闭。

### Playwright 验证结果

| 测试场景 | 预期 | 结果 |
|---------|------|------|
| 点击 toggle 按钮打开菜单 | 菜单显示 | 通过 |
| 在菜单内点击（`.baseline-menu-info`） | 菜单保持打开 | 通过 |
| 点击页面外部区域（body 10,10） | 菜单关闭 | 通过 |
| 点击 toggle 按钮关闭菜单 | 菜单关闭 | 通过 |
| tsc --noEmit | 无错误 | 通过 |
| 前端 build（tsc + vite） | 成功 | 通过 |
| Go exe build | 成功 | 通过 |

### 构建结果

```
frontend: tsc -b && vite build -- OK (dist/assets/index-CfZFH_OO.js)
Go exe:  go build -o followitup.exe ./cmd/server/ -- OK
```

### 提交

（见本次提交 SHA）
