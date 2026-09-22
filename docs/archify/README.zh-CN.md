# 架构图（archify）

简体中文 · [English](README.md)

本仓库的系统架构图，由 [archify](https://github.com/tt-a1i/archify) 生成。中英文版本共享同一布局、出自同一管线，结构上始终一致。

## 目录结构

```
docs/archify/
├── zh-CN/                                    # 中文版
│   ├── message-push-system.architecture.json # 可编辑源（唯一事实来源）
│   ├── message-push-system.html              # 独立交互交付物
│   ├── message-push-system.png               # 4x light 导出（7456x2776），README.zh-CN.md 引用
│   └── message-push-system.visual-check.*    # 浏览器验证证据（已 gitignore）
└── en/                                       # 英文版（结构相同，带 .en 后缀）
```

- `*.architecture.json` — 唯一可编辑源：组件、边界、连线、卡片、视图的 typed-IR 描述。
- `*.html` — `deliver` 产物：自包含交互文件，支持主题切换、缩放与视图章节。
- `*.png` — 4x light 主题位图导出，被根目录 README 引用。
- `*.visual-check.*` — `visual-check` 产出的自动化浏览器证据；已 gitignore，可随时重新生成。

## archify skill 说明

[archify](https://github.com/tt-a1i/archify) 是一个开源 agent skill：Node.js 渲染 + 门禁校验系统，把小型 typed JSON 描述变成经过校验的交互式 HTML 架构图。每台机器安装一次：

```bash
npx skills add tt-a1i/archify -g    # 安装到 ~/.claude/skills/archify
```

CLI 位于 `~/.claude/skills/archify/bin/archify.mjs`（直接用 node 运行，skill 内无需 npm install）。

## 更新架构图

在仓库根目录执行。设 `A=~/.claude/skills/archify`，`<lang>` 取 `zh-CN`（文件无后缀）或 `en`（文件带 `.en` 后缀）：

```bash
# 1. 编辑对应语言的源 JSON：
#    docs/archify/<lang>/message-push-system[.en].architecture.json

# 2. 校验；只修诊断指向的对象（subject + supportedFixes）
node $A/bin/archify.mjs validate architecture \
  docs/archify/zh-CN/message-push-system.architecture.json --quality standard --json

# 3. 交付：渲染 + 9 项工件检查 + HTML 原子替换
node $A/bin/archify.mjs deliver architecture \
  docs/archify/zh-CN/message-push-system.architecture.json \
  docs/archify/zh-CN/message-push-system.html --quality standard --json

# 4. 浏览器证据（可选；侧车文件生成在 HTML 同目录）
node $A/bin/archify.mjs visual-check docs/archify/zh-CN/message-push-system.html --json

# 5. 导出 PNG（4x light，覆盖已入库的 PNG）
node scripts/export-archify-diagram.mjs \
  docs/archify/zh-CN/message-push-system.html \
  docs/archify/zh-CN/message-push-system.png 4
```

## 双语同步规则

- 任何**几何变更**（位置、尺寸、路由、viewBox）必须同时改两份 JSON；内容再按语言各自翻译。
- 代码标识符在本地化文案中保留英文：API 路径、协议、流名（`push:stream:dead_letter`）、产品名（Redis、Vue · Vben Admin）。
- `meta.locale`（`zh-CN` / `en`）只本地化查看器外壳——图例、标题、无障碍标注、`<html lang>`；不会翻译作者内容。

## 踩坑速查（实测）

- 连线的 `via` 必须给**折线拐点组**而不是单点——单点会产生斜线段穿越节点。
- 可读性门禁按**视口高度**适配 viewBox：宽高比保持在 ~2.69 附近（当前 1880x700），否则 1440x900 下节点文字投影低于下限。
- 标签摆放工具按优先级：默认位置 → `labelDy` / `labelDx` → `labelSegment` → `labelAt`。
- 修复循环纪律：每轮只改诊断指向的对象；连续两轮错误数不降就停下，如实报告未解决的诊断。
