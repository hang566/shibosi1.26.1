@echo off
chcp 65001 >nul 2>&1
title 市舶司 - 停止管理后台

:: 停止管理后台 (端口 8084)
for /f "tokens=5" %%a in ('netstat -ano ^| findstr ":8084 " ^| findstr LISTENING') do (
    taskkill /F /PID %%a >nul 2>&1
)
taskkill /F /FI "WINDOWTITLE eq 管理后台*" >nul 2>&1
taskkill /F /IM "admin-core.exe" >nul 2>&1