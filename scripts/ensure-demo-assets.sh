#!/bin/sh
set -eu

root="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
admin_index="$root/static/admin/index.html"
install_index="$root/static/install/index.html"

if [ -f "$admin_index" ] && [ -f "$install_index" ]; then
  exit 0
fi

if ! command -v pnpm >/dev/null 2>&1; then
  echo "缺少 pnpm：首次从源码运行演示需要 Node.js 22+ 与 pnpm。" >&2
  exit 1
fi

if [ ! -f "$root/admin-webui/package.json" ]; then
  git -C "$root" submodule update --init admin-webui
fi

pnpm --dir "$root/admin-webui" install --frozen-lockfile
pnpm --dir "$root/admin-webui" build:antd
pnpm --dir "$root/admin-webui" build:install

if [ ! -f "$admin_index" ] || [ ! -f "$install_index" ]; then
  echo "后台静态资源构建完成，但预期入口文件不存在。" >&2
  exit 1
fi
