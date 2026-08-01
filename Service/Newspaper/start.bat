@echo off
chcp 65001 >nul
echo ========================================
echo   市舶司 - 昨日晚报服务启动脚本
echo ========================================
echo.

cd /d "%~dp0"

if not exist "go.sum" (
    echo 正在初始化依赖...
    go mod tidy
    if errorlevel 1 (
        echo 依赖初始化失败，请检查Go环境
        pause
        exit /b 1
    )
)

echo 正在编译...
go build -o newspaper.exe .
if errorlevel 1 (
    echo 编译失败！
    pause
    exit /b 1
)

echo.
echo 编译成功！正在启动服务...
echo.
start "昨晚晚报" /D "%~dp0." newspaper.exe

timeout /t 2 /nobreak >nul

echo.
echo ========================================
echo   服务已启动！
echo   访问地址: http://localhost:8082
echo   按任意键打开浏览器...
echo ========================================
echo.

pause >nul
start http://localhost:8082
