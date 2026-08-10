# 允许使用者覆盖模型

默认情况下，Step 始终使用 `defaultModelRef`。只有业务确实要求每行或每次运行选择模型时，才开放覆盖。

## 1. 在 Step 开启覆盖

```json
"allowModelOverride": true
```

## 2. 创建选择字段

字段 key 不能直接使用保留字 `model`，可使用 `text_model`。通常用 enum 或 string，并通过当前模型目录维护合法值。

## 3. 绑定 model 参数

```json
{
  "fieldKey": "text_model",
  "stepId": "stp_write01",
  "paramKey": "model",
  "bindMode": "shared"
}
```

`model` 只支持 shared FieldBinding。ParamBinding、expanded、provider 和 mode 都不支持模板路由覆盖。

## 验证

本地 check 后还需服务端确认每个候选 model ID 支持目标 execution unit。若不需要用户选择，删除字段和 Binding，保留固定默认模型可减少错误。
