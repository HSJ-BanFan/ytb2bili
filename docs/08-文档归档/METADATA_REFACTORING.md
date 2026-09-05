# 元数据流程重构 - 实施总结

## 🎯 重构目标

解决"生成元数据"任务状态不明确的问题，让用户和系统都能清楚知道：
1. 元数据的来源（原始/AI生成/用户编辑）
2. AI 是否尝试生成
3. 最终上传使用的是哪个版本

## ✅ 已完成的工作

### Phase 1: 数据库迁移 ✅

**新增字段** (migrations/001_add_upload_metadata.sql):
```sql
upload_title       VARCHAR(500)   -- 实际上传到B站的标题
upload_desc        TEXT           -- 实际上传到B站的描述
upload_tags        VARCHAR(1000)  -- 实际上传到B站的标签
metadata_source    VARCHAR(20)    -- 来源: original/ai_generated/user_edited
metadata_edit_status VARCHAR(20)  -- 状态: auto/pending_review/edited
```

**执行方式**:
```go
// 在 main.go 或数据库初始化时调用
db.RunMigrations(database, cfg)
```

### Phase 2: 数据模型更新 ✅

**pkg/store/model/models.go**:
- 新增 `UploadTitle`, `UploadDesc`, `UploadTags` 字段
- 新增 `MetadataSource`, `MetadataEditStatus` 字段
- 添加字段注释说明用途

### Phase 3: 新增"确认元数据"任务 ✅

**internal/chain_task/handlers/confirm_metadata.go**:
- 职责：根据配置决定最终元数据
- 优先级：配置 > AI生成 > 原始
- 输出：设置 `upload_*` 字段和 `metadata_source`
- 返回详细的 `result_data`:
  ```go
  {
    "metadata_source": "ai_generated",
    "ai_attempted": true,
    "has_ai_data": true,
    "title_source": "✨ AI生成",
    "upload_title": "...",
    ...
  }
  ```

### Phase 4: 任务链重构 ✅

**internal/chain_task/chain_task_handler.go**:
- 优化任务执行顺序，支持并行：
  ```
  Group 1: 获取元数据
  Group 2: [下载封面, 下载视频, AI增强元数据] ← 并行
  Group 3: 下载字幕
  Group 4: 翻译字幕
  Group 5: 确认元数据 ← 等待 Group 2c + Group 4
  ```
- 新增任务：`confirmMetadataTask`

### Phase 5: 上传处理器更新 ✅

**internal/chain_task/handlers/upload_to_bilibili.go**:
- 优先使用 `upload_*` 字段
- 回退逻辑保留（兼容旧数据）
- 日志显示数据来源：
  ```
  ✓ 使用预设标题 [✨ AI生成]: xxx
  ✓ 使用预设描述 [📹 原始]
  ✓ 使用预设标签
  ```

### Phase 6: 前端类型定义 ✅

**web/src/types/index.ts**:
- 更新 `VideoDetail` 接口：
  - 新增 `upload_*` 字段
  - 新增 `metadata_source`, `metadata_edit_status`
  - 区分 `title` (原始) 和 `upload_title` (最终)
- 新增 `failed_permanent` 状态
- 更新 `TASK_STEP_NAMES` 包含新步骤

### Phase 7: 前端组件 ✅

**web/src/components/MetadataSourceBadge.tsx**:
- `MetadataSourceBadge`: 显示数据来源徽章
  - 📹 原始 (灰色)
  - ✨ AI生成 (紫色)
  - 📝 用户编辑 (蓝色)
- `MetadataComparison`: 三栏对比视图
  - 原始 | AI生成 | 最终版本

## 📋 数据流示意图

```
┌─────────────────┐
│  获取元数据     │ → title, description (YouTube原始)
└─────────────────┘
         ↓
┌─────────────────┐
│ AI增强元数据    │ → generated_title, generated_desc, generated_tags
│ (可选,可失败)    │   [失败时状态=skipped, 不影响流程]
└─────────────────┘
         ↓
┌─────────────────┐
│  确认元数据     │ → upload_*, metadata_source
│ (决策逻辑)      │   根据 UseOriginalTitle 配置决定:
│                  │   - AI存在 + 配置AI → upload_* = generated_*
│                  │   - 否则 → upload_* = title, description
└─────────────────┘
         ↓
┌─────────────────┐
│ 上传到Bilibili  │ → 优先使用 upload_*
│                  │   [不存在时回退到旧逻辑，兼容旧数据]
└─────────────────┘
```

## 🔄 状态语义说明

| 任务状态 | 含义 | 前端显示 |
|---------|------|---------|
| `pending` | 等待执行 | 灰色 |
| `running` | 正在执行 | 蓝色 |
| `completed` | 成功完成 | 绿色 |
| `failed` | 失败（可重试） | 红色 |
| `skipped` | 跳过（未启用或无需执行） | 黄色 |
| `failed_permanent` | 永久失败（不可重试） | 深红色 |

**AI增强元数据的特殊处理**:
- AI 失败但回退到原始数据 → 状态=`skipped`, `result_data` 说明原因
- AI 真正失败 → 状态=`failed`

## 🚀 后续工作

### Phase 8: 前端显示优化 (待实现)

**视频详情页增强**:
1. 显示元数据来源徽章
2. 三栏对比视图（原始 | AI | 最终）
3. 任务步骤显示 `result_data` 详情

**示例代码**:
```tsx
// src/pages/VideoDetail.tsx
import { MetadataSourceBadge, MetadataComparison } from '@/components/MetadataSourceBadge';

function VideoDetailPage() {
  const { data } = useVideoDetail();

  return (
    <div>
      {/* 标题显示来源徽章 */}
      <div className="flex items-center gap-2">
        <h1>{data.upload_title || data.title}</h1>
        <MetadataSourceBadge source={data.metadata_source} />
      </div>

      {/* 三栏对比 */}
      <MetadataComparison
        label="标题"
        original={data.title}
        generated={data.generated_title}
        upload={data.upload_title}
        source={data.metadata_source}
      />

      {/* 任务步骤显示详情 */}
      {data.task_steps.map(step => (
        <TaskStepCard
          key={step.id}
          step={step}
          resultData={step.result_data}
        />
      ))}
    </div>
  );
}
```

### Phase 9: 用户编辑功能 (未来)

**API 路由**:
```
PUT /api/v1/videos/:id/metadata
{
  "upload_title": "...",
  "upload_desc": "...",
  "upload_tags": "...",
  "metadata_source": "user_edited"
}
```

**前端编辑界面**:
```tsx
// 编辑模式
function MetadataEditor({ video }) {
  const [editMode, setEditMode] = useState(false);
  const [metadata, setMetadata] = useState({
    upload_title: video.upload_title,
    upload_desc: video.upload_desc,
    upload_tags: video.upload_tags
  });

  const handleSave = async () => {
    await updateVideoMetadata(video.id, {
      ...metadata,
      metadata_source: 'user_edited',
      metadata_edit_status: 'edited'
    });
    setEditMode(false);
  };

  return editMode ? (
    <EditForm metadata={metadata} onChange={setMetadata} onSave={handleSave} />
  ) : (
    <DisplayMetadata metadata={metadata} onEdit={() => setEditMode(true)} />
  );
}
```

## 🧪 测试验证

### 单元测试
```go
// internal/chain_task/handlers/confirm_metadata_test.go
func TestConfirmMetadata_DecisionLogic(t *testing.T) {
  tests := []struct {
    name              string
    hasAITitle        bool
    useOriginalTitle  bool
    expectedSource    string
  }{
    {
      name:             "AI存在且配置使用AI",
      hasAITitle:       true,
      useOriginalTitle: false,
      expectedSource:   "ai_generated",
    },
    {
      name:             "AI存在但配置使用原始",
      hasAITitle:       true,
      useOriginalTitle: true,
      expectedSource:   "original",
    },
    // ... 更多测试用例
  }
  // ...
}
```

### 集成测试
1. 提交视频URL → 检查任务步骤
2. AI生成成功 → 检查 `metadata_source = 'ai_generated'`
3. AI生成失败 → 检查状态=`skipped` + `result_data.fallback_reason`
4. 上传到B站 → 检查使用的是 `upload_title`

## 📊 兼容性说明

### 旧数据处理
- 数据库迁移会自动填充 `upload_*` 字段
- 未执行"确认元数据"的旧任务，上传时回退到旧逻辑
- 用户无感知，系统自动兼容

### 前端兼容
- `VideoDetail` 接口新增可选字段
- 旧API响应不包含新字段也不会报错
- 组件渐进式更新，不影响现有功能

## 🎓 关键设计决策

### 1. 为什么用 `upload_*` 而不是 `final_*`?
**答**: 更语义化，明确表示"实际上传到B站"的数据

### 2. 为什么不新增 `completed_with_fallback` 状态?
**答**: 使用 `result_data` 存储详情更灵活，避免状态爆炸

### 3. 为什么"确认元数据"是独立任务?
**答**: 职责单一，未来可扩展为暂停等待用户审核

### 4. 为什么AI增强可以和下载并行?
**答**: AI只需要"获取元数据"的结果，不依赖视频文件，可以提前开始

## 🔗 相关文件清单

### 后端
- ✅ `migrations/001_add_upload_metadata.sql`
- ✅ `migrations/002_rollback_upload_metadata.sql`
- ✅ `internal/db/migration.go`
- ✅ `pkg/store/model/models.go`
- ✅ `internal/chain_task/handlers/confirm_metadata.go`
- ✅ `internal/chain_task/chain_task_handler.go`
- ✅ `internal/chain_task/handlers/upload_to_bilibili.go`

### 前端
- ✅ `web/src/types/index.ts`
- ✅ `web/src/components/MetadataSourceBadge.tsx`
- ⏳ `web/src/pages/VideoDetail.tsx` (待更新)
- ⏳ `web/src/components/TaskStepCard.tsx` (待更新)

---

**最后更新**: 2025-01-XX
**实施者**: Claude Code
**审核者**: [待填写]
