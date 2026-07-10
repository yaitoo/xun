# xun 模板行为与 Go 标准一致性验证

## 验证结果：✅ 完全一致

本文档验证 xun 的模板行为与 Go 标准 `html/template` 的一致性。

---

## 关键差异：`{{block}}` vs `{{template}}`

### 1. `{{block}}` 行为

#### Go 标准行为：
```go
{{block "optional" .}}default content{{end}}
```
- ✅ 自动创建名为 "optional" 的模板存根（stub）
- ✅ 如果未被覆盖，使用默认内容 "default content"
- ✅ 可以通过 `{{define "optional"}}` 覆盖默认内容
- ✅ "optional" 出现在 `Templates()` 返回列表中

#### xun 行为（修复后）：
- ✅ **完全一致** - 所有测试通过

---

### 2. `{{template}}` 行为

#### Go 标准行为：
```go
{{template "required" .}}
```
- ❌ 不创建任何存根
- ❌ 如果 "required" 未定义，**运行时错误**
- ✅ "required" 不会出现在 `Templates()` 中（除非通过 `{{define}}` 定义）

#### xun 行为（修复后）：
- ✅ **完全一致** - 所有测试通过

---

## 验证测试用例

### Test Suite 1: Block 行为验证

| 测试场景 | Go 标准 | xun 行为 | 状态 |
|---------|---------|----------|------|
| Block 未定义时使用默认内容 | ✅ 使用默认 | ✅ 使用默认 | ✅ 一致 |
| Block 被 define 覆盖 | ✅ 使用自定义 | ✅ 使用自定义 | ✅ 一致 |
| Block 创建存根 | ✅ 在 Templates() | ✅ 在 dependencies | ✅ 一致 |

### Test Suite 2: Template 行为验证

| 测试场景 | Go 标准 | xun 行为 | 状态 |
|---------|---------|----------|------|
| Template 未定义 | ❌ 运行时错误 | ❌ 运行时错误 | ✅ 一致 |
| Template 有 define | ✅ 正常工作 | ✅ 正常工作 | ✅ 一致 |
| Template 不创建存根 | ✅ 不在 Templates() | ✅ 不在 dependencies | ✅ 一致 |

### Test Suite 3: 依赖检测验证

| 测试场景 | Go 标准 | xun 行为 | 状态 |
|---------|---------|----------|------|
| `{{block "x"}}` | "x" 在 Templates() | "x" 在 dependencies | ✅ 一致 |
| `{{define "x"}}` | "x" 在 Templates() | "x" 在 dependencies | ✅ 一致 |
| `{{template "x"}}` | "x" 不在 Templates() | "x" 不在 dependencies | ✅ 一致 |

### Test Suite 4: 混合场景验证

```html
Layout:
{{block "blockName" .}}block default{{end}}
{{template "templateName" .}}

Page without definitions:
- block → 使用 "block default" ✅
- template → 运行时错误 ❌（符合预期）
```

**测试结果：** ✅ 所有场景通过

---

## 关键实现细节

### xun 如何实现与 Go 标准的一致性

1. **Block 存根复制** (`template_html.go:96-107`)
   ```go
   for _, lt := range layout.template.Templates() {
       ltName := lt.Name()
       if nt.Lookup(ltName) == nil || ltName == layoutName {
           _, err = nt.AddParseTree(ltName, lt.Tree)
       }
   }
   ```
   - 遍历 layout 的所有模板（包括 block 自动生成的存根）
   - 复制到 page 的模板集中
   - 保留 Go 的 block 默认内容语义

2. **依赖检测** (`template_html.go:63-70`)
   ```go
   for _, it := range nt.Templates() {
       tn := it.Name()
       if !strings.EqualFold(tn, t.name) {
           dependencies[tn] = struct{}{}
       }
   }
   ```
   - 使用 `Templates()` 检测所有已定义的模板
   - Block 存根会被检测到（因为 Go 创建了它们）
   - Template 调用不会被检测到（Go 不为它们创建存根）

3. **优先级处理**
   - Page 的 `{{define}}` 先解析
   - Layout 的模板只在不存在时添加（`Lookup == nil`）
   - 实现了正确的覆盖语义

---

## 为什么这两个行为不一致？

这是 **Go 语言的设计决策**，而非 bug：

### `{{block}}` 的设计目的
- 提供**可选的占位符**机制
- 支持**继承式模板**（layout/base template）
- 允许子模板选择性覆盖

### `{{template}}` 的设计目的
- 提供**强制的包含**机制
- 明确要求模板存在
- 更接近传统的 "include" 语义

---

## 测试覆盖率

- **一致性测试：** 10 个子测试，全部通过 ✅
- **场景覆盖：**
  - ✅ Block 默认内容
  - ✅ Block 覆盖
  - ✅ Template 存在
  - ✅ Template 不存在
  - ✅ 依赖检测
  - ✅ 混合使用

---

## 结论

✅ **xun 的模板行为与 Go 标准 `html/template` 完全一致**

- `{{block}}` 行为：✅ 一致
- `{{template}}` 行为：✅ 一致
- 依赖检测：✅ 一致
- 覆盖语义：✅ 一致

**验证日期：** 2026-07-10  
**测试文件：** `template_html_go_consistency_test.go`  
**测试结果：** 10/10 通过
