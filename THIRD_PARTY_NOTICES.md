# 第三方许可

Go 运行依赖为 go-toml v2.4.3（MIT）；只在配置适配器中使用。源码未复制入仓库。其许可文本如下，分发包含该依赖的二进制时一并保留。

```text
The MIT License (MIT)

go-toml v2
Copyright (c) 2021 - 2023 Thomas Pelletier

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

llama.cpp 引擎及 Qwen 模型独立下载，未纳入 Git 跟踪；各自的许可证与摘要记录在 docs/research/。Python 校验和调用示例依赖由各自的哈希锁文件管理，不改变上游许可。
