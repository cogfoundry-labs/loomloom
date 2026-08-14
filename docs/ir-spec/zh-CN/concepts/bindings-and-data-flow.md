# Binding 与数据流

v2 把“输入是什么”和“输入从哪里来”分开：Template Input 描述可填写值，Step `inputBindings` 把值绑定到合同端口。

一个 binding 只有一个 source。直接来源使用 templateInput、stepOutput、literal 或 platformContext；复合来源使用 composeValue、sequence 或 merge。Artifact 和标量遵循同一套来源模型。

`dependsOn` 控制调度，`inputBindings` 控制数据。引用上游输出时两者必须同时声明。
