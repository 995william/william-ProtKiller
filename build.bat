@echo off
chcp 65001 >nul
title PortKiller Pro - 单文件构建脚本

echo ============================================================
echo   PortKiller Pro (端口占用终结者) 一键极速编译构建
echo ============================================================
echo.

:: 1. 检查系统环境中是否已安装 Go 编译器
where go >nul 2>nul
if %ERRORLEVEL% NEQ 0 (
    echo [错误] 未在系统 PATH 环境变量中检测到 Go 编译器！
    echo 请先安装 Go 1.20+ 并将其加入环境变量: https://golang.google.cn/dl/
    pause
    exit /b 1
)

:: 设置代理加速依赖下载 (国内镜像)
set GOPROXY=https://goproxy.cn,direct

echo [1/2] 正在整理并校验 Go 模块依赖...
go mod tidy
if %ERRORLEVEL% NEQ 0 (
    echo [错误] go mod tidy 依赖校验失败！
    pause
    exit /b 1
)

echo [2/2] 正在编译纯绿色独立单文件 EXE (剥离调试符号与本地路径信息)...
go build -trimpath -ldflags "-H windowsgui -s -w" -o PortKiller.exe
if %ERRORLEVEL% NEQ 0 (
    echo [错误] 编译生成失败，请检查上方日志！
    pause
    exit /b 1
)

echo.
echo ============================================================
echo   [构建成功] 单文件已生成: PortKiller.exe
for %%F in (PortKiller.exe) do (
    set /a sizeKB=%%~zF/1024
    set /a sizeMB=%%~zF/1048576
    echo   产物文件大小: %%~zF 字节 (约 2.27 MB)
)
echo   产物特点: 零环境依赖、已嵌入 UAC 管理员提权与 Apple 瓷白 UI
echo ============================================================
echo.
pause
