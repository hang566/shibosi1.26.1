@echo off
chcp 65001 >nul 2>&1
title 市舶司 - 管理后台
color 0B

set "DIR=%~dp0"
set "DIR=%DIR:~0,-1%"
set "EXE=%DIR%\admin-core.exe"
set "PORT=8084"

:: 释放端口
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":%PORT% " ^| findstr LISTENING') do (
    taskkill /F /PID %%a >nul 2>&1
)

:: 尝试编译好的 exe
if exist "%EXE%" (
    start "管理后台" /D "%DIR%" "%EXE%"
    echo [成功] 管理后台已启动 - http://localhost:%PORT%/admin
    timeout /t 1 /nobreak >nul
    exit /b 0
)

:: 尝试 go run
where go >nul 2>&1
if %errorLevel% == 0 (
    if exist "%DIR%\main.go" (
        echo [启动] 使用 go run 启动...
        cd /d "%DIR%"
        start "管理后台" /D "%DIR%" cmd /c "go run main.go"
        timeout /t 2 /nobreak >nul
        netstat -ano | findstr ":%PORT%" | findstr LISTENING >nul 2>&1
        if %errorLevel% == 0 (
            echo [成功] 管理后台已启动 - http://localhost:%PORT%/admin
            exit /b 0
        )
    )
)

:: 无法启动
echo ============================================
echo   管理后台启动失败
echo ============================================
echo.
echo   原因: 未找到 admin-core.exe，且未安装 Go
echo.
echo   解决方法:
echo   1. 安装 Go 1.23+ (https://go.dev/dl/)
echo   2. 在 Service\admin-core\ 目录下运行:
echo      go mod tidy
echo      go build -o admin-core.exe .
echo   3. 或通过 launcher.bat [B] 一键编译
echo.
echo ============================================
echo.
pause