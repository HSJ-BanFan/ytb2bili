# 配置文件

此目录包含项目的配置文件。

## 📁 文件说明

### `prompts.yaml`

AI 服务提示词配置文件，用于：

- **元数据生成**: 视频、标题、描述、标签生成
- **翻译优化**: 字幕翻译提示词
- **内容适配**: B站平台规范适配

## 🔧 配置项

```yaml
metadata:
  title:
    max_length: 80
    style: professional

  description:
    max_length: 2000
    include_source: true

  tags:
    count: 10-15
    auto_generate: true
```

## 📝 使用说明

1. **修改提示词**
   ```bash
   编辑 config/prompts.yaml
   ```

2. **重启应用**
   ```bash
   make restart
   ```

3. **验证配置**
   - 提交一个新视频
   - 检查生成的元数据是否符合预期

## 🔗 相关文档

- [元数据重构文档](../docs/08-文档归档/METADATA_REFACTORING.md)
- [AI 服务配置](../docs/02-配置指南/)

---

*最后更新: 2024-12-29*
