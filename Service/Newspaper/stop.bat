@echo off
chcp 65001 >nul
echo 正在停止昨日晚报服务...
taskkill /f /im newspaper.exe >nul 2>&1
if errorlevel 1 (
    echo 服务未运行
) else (
    echo 服务已停止
)
pause
