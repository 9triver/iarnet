Runner 容器目录
================

本目录用于存放各语言 / 运行环境的 Runner 镜像定义和相关代码。

结构约定：

- `core/`：通用 Runner Core（用 Go 实现，只负责拉起应用、连接 Ignis 和 Logger，与具体语言运行时无关）
- `python/`：Python Runner 镜像（多阶段构建，内置 Runner Core 和 Python 运行环境）
- `java/`：Java Runner 镜像（预留，将来同样复用 `core/`）
- 后续如果有其他环境（如 nodejs、cuda 等），也在此目录下新建对应子目录，共享 `core/`。


