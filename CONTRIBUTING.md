# 贡献指南

感谢你考虑为 nazhi-cli 贡献代码！🎉

## 开发流程

1. Fork 本仓库
2. 创建功能分支：`git checkout -b feat/your-feature`
3. 提交变更（遵循[提交规范](#提交规范)）
4. 推送分支并创建 Pull Request

## 环境要求

- Go 1.26+
- 交叉编译无需额外依赖（ONNX Runtime DLL 已内嵌）

## 本地开发

```bash
# 构建
make build

# 运行测试（race 检测）
make test

# 代码检查
make vet
make lint
```

## 提交规范

提交信息遵循 [Conventional Commits](https://www.conventionalcommits.org/)：

```
<type>(<scope>): <description>

feat(pkg/client): 添加 XXX 接口
fix(cmd/nazhi): 修复 YYY 问题
chore(deps): 更新 ZZZ 依赖
docs: 更新 README
```

## Pull Request

- PR 标题同样遵循 Conventional Commits
- 附上变更说明和测试结果
- 确保 `make test` 全量通过
- 保持单次 PR 只聚焦一个功能 / 修复

## 协议

贡献的代码将遵循 MIT 协议。
