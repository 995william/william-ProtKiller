# PortKiller Pro - Windows 单文件端口占用查询与强杀工具设计规范

> 本文档规范了 Windows 单文件桌面端小工具 **PortKiller Pro（端口终结者）** 的全套产品交互、Canvas雷达设计机制及底层技术实现方案。

---

## 一、 产品定位与核心目标

1. **核心痛点**：开发者在本地启动 Web 项目、数据库或微服务时，经常遇到 `Error: listen EADDRINUSE: address already in use :::8080 / 3000`。传统做法需手动打开终端执行 `netstat -ano | findstr :8080`，获取 PID 后再执行 `taskkill /F /PID xxxx`，操作繁琐且缺乏直观进程感知。
2. **产品形态**：**Windows 单文件无依赖可执行程序（.exe）**，绿色免安装，双击秒开，极简轻便。
3. **视觉与体验定位**：**赛博朋克深色科技 HUD (Cyberpunk / Deep Space HUD)**，配备高帧率 Canvas 全息雷达扫射探测与端口空间矩阵，提供极具沉浸感的高端大气炫酷交互。

---

## 二、 交互原型界面全景与 Canvas 引擎设计

可交互原型请参阅工作区根目录中的：[prototype.html](../prototype.html)。

### 1. 全息 Canvas 雷达引擎架构
- **坐标映射与空间分布**：
  - 采用极坐标系（Polar Coordinates），以 `(cx, cy)` 为扫描中心；
  - 端口号 $0 \sim 65535$ 经平方根归一化映射到雷达半径 $R$，避免低端口过于拥挤，外环高端口均匀扩散；
  - 采用黄金角分布算法 `angle = (port * 137.5°)` 分布节点，消除重叠碰撞。
- **动态扫射与余晖效果**：
  - 每秒 60FPS 顺时针旋转的扇形雷达高亮光束（Sweep Gradient）；
  - 扫描指针到达目标点时触发脉冲光晕与粒子微震。
- **双向联动交互**：
  - **Canvas 到列表联动**：光标移动到 Canvas 节点，自动激活高科技 HUD 准星悬浮框；点击节点，右侧进程列表自动平滑滚动并闪烁聚焦。
  - **列表到 Canvas 联动**：在右侧筛选、搜索或选择进程，Canvas 对应节点同步变色脉冲。

### 2. 核心功能模块设计

```mermaid
flowchart TD
    A[双击 PortKiller.exe] --> B[自动以管理员权限启动 (UAC Manifest)]
    B --> C[调用 Windows iphlpapi.dll 嗅探端口占用]
    C --> D[加载主界面: HUD状态条 + Canvas雷达 + 进程列表]
    D --> E{用户交互操作}
    E -->|模糊搜索 / 常用标签过滤| F[毫秒级实时过滤响应 + 雷达节点过滤]
    E -->|点击单条 '强杀'| G[唤起赛博朋克确认弹窗 (显示PID/路径/警告)]
    E -->|勾选多项 / 全选| H[激活 '批量终结' 操作栏]
    G -->|确认终结| I[调用 TerminateProcess 强杀进程]
    H -->|一键清理| I
    I --> J[播放科技爆裂音效 + 粒子消散动效]
    J --> K[弹出 Toast 提示 '端口 8080 已释放' 并刷新列表与雷达]
```

### 3. Windows 单体桌面小工具（EXE）专属 6 大异构布局体系

本系统**严格依照桌面客户端原生窗口标准（920px × 650px 紧凑视窗）设计，外层窗口完全无滚动条，仅内部列表支持精细滚动**。设计了 6 套在**窗口布局（Layout）、组件形态（Components）、交互模式（Interactions）与配色体系（Colors）**上彻底迥异的原生桌面小工具体验：

| 风格序号 | 风格流派与色彩 | 专属窗口布局架构 (Desktop Layout) | 核心组件形态 (Components) | Canvas 动态微控件 |
| :--- | :--- | :--- | :--- | :--- |
| **🍎 1. 苹果瓷白侧边栏** | **纯净亮色** / 珠光瓷白 (`#f2f5fa`)、柔和高光毛玻璃、天蓝强调色 | **左右经典分栏 (Sidebar + Table)**：左侧 220px 快捷分类侧边栏，右侧主数据表 | • 苹果健康三环能量微组件<br>• 快速搜索与批量多选操作栏<br>• 单击即强杀胶囊按键 | **Apple 环形能量仪** (实时计算健康分与冲突率) |
| **🥽 2. 空间灵动胶囊** | **极光浅色** / 晨曦极光半透、悬浮圆角质感 | **上下三段式沉浸流 (Island + 3D Orbit + Cards)**：顶部灵动岛，中部对称分体 | • **顶部灵动岛状态胶囊** (强杀时弹性变大反馈)<br>• 右侧浮动折叠卡片流<br>• 微重力悬浮质感 | **3D 浮动全息量子球** (自转量子粒子，红色冲突报警) |
| **🛸 3. 赛博深空战术** | **纯黑 OLED** / 极深黑夜 (`#060913`)、赛博青 (`#00f5d4`)、霓虹赤红 (`#ff2a6d`) | **战术仪表台 (Radar Header + Matrix Table)**：上部雷达遥测卡，下部紧凑矩阵表 | • 战术 HUD 仪表头与状态卡<br>• 一键终止所有冲突端口按钮<br>• 高对比度紧凑数据流 | **战术毫米波雷达** (60FPS 顺时针旋转光束与余晖衰减) |
| **⚡ 4. 光子纯白机甲** | **工业冷白科技** / 纯白机甲 (`#e5e8ef`)、电光橙 (`#ff6b00`)、硬朗斜切角 | **频谱示波指挥台 (Oscilloscope + Split Console)**：横向示波器 + 左表右终端 | • **实时内核指令日志终端 (Terminal)**，滚动打印底层 Win32 API 追踪<br>• 紧凑机甲表格<br>• 斜切角 KILL 键 | **0~65535 全频段示波器** (正弦尖峰跳动波形) |
| **🎨 5. 极速新丑看板** | **高能亮黄撞色** / 活力明黄 (`#fcf6e8` / `#ffe600`)、2.5px 粗黑边框、立体硬阴影 | **四列等宽状态看板 (Four-column Kanban)**：冲突 / Web / 数据库 / 系统保护 | • 四列独立纵向滚动卡片<br>• **"KILL 印章" 盖章碎裂动效**<br>• 直观分状态看板管理 | 模块状态条指示器 |
| **💎 6. 黑曜石瑞士极简** | **高奢暗夜黑金** / 黑曜石纯黑 (`#0f1115`)、琥珀金 (`#d4af37`)、极细微光线 | **Spotlight 指令流 + 检查器 (Command + Split Inspector)**：左列表右详情抽屉 | • **顶部全局 Spotlight 搜索条** (支持快捷键)<br>• **右侧 Inspector 检查器抽屉** (显示完整路径、PID、状态)<br>• 单手快捷终结 | 极简点阵状态波纹 |

---

## 三、 单文件 Windows EXE 落地技术方案选型

为了实现成品为**单个独立的 Windows .exe 文件**，同时兼顾**高科技 UI 渲染**与**原生系统底层性能**，提供以下对比评估：

```mermaid
graph LR
    subgraph 方案对比
        A["方案 1: Go + Wails v2 (强烈推荐 ⭐⭐⭐⭐⭐)"]
        B["方案 2: Rust + Tauri v2 (备选首选 ⭐⭐⭐⭐)"]
        C["方案 3: C# .NET 9 AOT + Webview/WinUI (⭐⭐⭐)"]
        D["方案 4: Python + PyQt + PyInstaller (⭐⭐)"]
    end
```

### 综合对比决策表

| 维度指标 | 方案 1: Go + Wails v2 (推荐) | 方案 2: Rust + Tauri v2 | 方案 3: C# (.NET 9 AOT) | 方案 4: Python (PyInstaller) |
| :--- | :--- | :--- | :--- | :--- |
| **单 EXE 文件体积** | **~8 MB - 12 MB** | **~5 MB - 8 MB** | ~15 MB - 25 MB | ~40 MB - 60 MB |
| **运行内存占用** | ~35 MB (借用系统 Edge/WebView2) | ~30 MB (WebView2) | ~50 MB | ~100 MB+ |
| **启动速度** | **0.2 秒 极速冷启动** | **0.1 秒 极速冷启动** | 0.3 秒 | 1.5 - 3.0 秒 (解压慢) |
| **Windows API 调用** | 简便 (syscall / golang.org/x/sys) | 极佳 (windows-rs crate) | 原生 P/Invoke | ctypes / psutil |
| **单文件打包难度** | `wails build` 一键出单文件 | `cargo tauri build` | `dotnet publish -p:PublishSingleFile=true` | 打包黑盒、杀软易误报 |

### 结论与推荐技术栈：
- **首选推荐：Go + Wails v2**。
  - 前端：使用我们刚刚设计的高保真 HTML5 + Canvas + Tailwind 原型，无需任何重构，直接作为 Wails 的前端资源静态编译进二进制文件；
  - 后端：Go 语言通过 Windows 动态链接库调用 Win32 原生 API，编译后**直接生成单一双击即运行的 `PortKiller.exe`**，干净利落。

---

## 四、 后端 Win32 核心底层接口实现规范

### 1. 端口与进程查询 (纳秒级嗅探，不依赖外部命令行)
- **避免使用**：在后台反复调用 `cmd /c netstat -ano`，因为每次起子进程会有明显卡顿与控制台闪烁。
- **推荐使用**：Windows 操作系统原生提供的 `iphlpapi.dll`：
  - `GetExtendedTcpTable(pTcpTable, &pdwSize, bOrder, AF_INET, TCP_TABLE_OWNER_PID_ALL, 0)`
  - `GetExtendedUdpTable(pUdpTable, &pdwSize, bOrder, AF_INET, UDP_TABLE_OWNER_PID, 0)`
  - 返回每一个端口对应的准确 PID、本地 IP、远端 IP 与连接状态。

### 2. 进程信息解析与白名单校验
- 根据 PID 调用 `kernel32.dll!OpenProcess(PROCESS_QUERY_LIMITED_INFORMATION, FALSE, pid)`
- 使用 `psapi.dll!GetProcessImageFileNameW` 或 `QueryFullProcessImageNameW` 获取进程完整路径（如 `node.exe`、`java.exe`）；
- 计算占用物理内存；
- **系统核心保护机制**：
  - PID 0 (Idle) 与 PID 4 (System)、`smss.exe`、`csrss.exe`、`wininit.exe` 等系统关键服务自动锁定，禁用强杀按钮，防止系统蓝屏。

### 3. 强力杀进程 (TerminateProcess)
- 调用 `OpenProcess(PROCESS_TERMINATE, FALSE, pid)` 获取句柄；
- 调用 `TerminateProcess(hProcess, 1)`；
- 针对常见的多进程架构（如 Chrome、Node、Worker 等），支持使用 `CreateToolhelp32Snapshot` 进行**进程树级联终止（Tree Kill）**，彻底避免残留死锁端口。

### 4. Windows 管理员 UAC 自动提权清单 (app.manifest)
在 exe 构建时嵌入 application manifest：
```xml
<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security>
      <requestedPrivileges>
        <requestedExecutionLevel level="requireAdministrator" uiAccess="false"/>
      </requestedPrivileges>
    </security>
  </trustInfo>
</assembly>
```
确保双击执行时自带盾牌标，可终结由后台服务、SYSTEM 或 WSL 占据的高权限端口。
