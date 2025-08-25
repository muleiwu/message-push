# 项目模板初始化说明

这个Go Web模板项目提供了一个自动初始化脚本，可以快速将模板项目转换为你自己的项目。

## 使用方法

### 方法一：自动检测远程仓库地址

如果你已经将项目推送到了远程仓库（如GitHub、GitLab等），脚本会自动检测远程仓库地址：

```bash
./init.sh
```

### 方法二：手动输入模块路径

如果脚本无法自动检测远程仓库地址，或者你想使用自定义的模块路径：

```bash
./init.sh
# 按提示输入你的模块路径，例如：github.com/username/project
```

## 脚本功能

1. **自动检测Git远程仓库地址**
   - 支持SSH格式：`git@github.com:user/repo.git`
   - 支持HTTPS格式：`https://cnb.cool/user/repo.git`

2. **智能批量替换模块路径**
   - 递归扫描所有 `.go` 文件和 `go.mod` 文件
   - 只替换实际包含模板路径的文件
   - 支持未来新增的Go文件，无需手动维护文件列表
   - 支持macOS和Linux系统

3. **清理和更新依赖**
   - 自动运行 `go mod tidy` 清理依赖

## 自动文件扫描

脚本会智能扫描项目中的所有文件：

- **递归扫描所有 `.go` 文件**：无论在哪个目录下
- **自动处理 `go.mod` 文件**：模块声明和依赖
- **智能检测**：只处理实际包含模板路径的文件
- **实时统计**：显示发现的文件数量和处理进度
- **验证结果**：确认所有文件都已正确替换

这意味着即使你后续添加新的Go文件，也无需修改脚本，它会自动处理所有相关文件。

## 完整使用流程

1. **克隆或下载模板项目**
   ```bash
   git clone <template-repo-url>
   cd go-web
   ```

2. **设置你的远程仓库**
   ```bash
   git remote remove origin
   git remote add origin <your-repo-url>
   git push -u origin master
   ```

3. **运行初始化脚本**
   ```bash
   ./init.sh
   ```

4. **开始开发**
   ```bash
   # 复制配置文件
   cp config.yaml.example config.yaml
   
   # 运行项目
   make run
   # 或
   go run main.go
   ```

## 注意事项

- 脚本会直接修改文件，建议在运行前备份或确保代码已提交到版本控制
- 确保Go环境已正确安装
- 脚本会自动扫描所有文件，无需担心项目结构变化
- 如果已经运行过脚本，再次运行会自动检测并跳过

## 手动替换

如果因为某些原因无法使用脚本，你也可以手动替换：

```bash
# 使用find和sed命令批量替换
find . -name "*.go" -type f -exec sed -i 's|cnb.cool/mliev/examples/go-web|your-module-path|g' {} +
sed -i 's|cnb.cool/mliev/examples/go-web|your-module-path|g' go.mod
go mod tidy
```

## 支持

如果在使用过程中遇到问题，请检查：
1. 文件权限是否正确
2. Go环境是否已安装
3. Git仓库配置是否正确 