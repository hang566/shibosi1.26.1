@echo off
chcp 936 >nul 2>&1
title 市舶司 - 统一启动器
color 0B

set "ROOT=%~dp0"
set "ROOT=%ROOT:~0,-1%"

:: 默认运维 Token（如需自定义，请设置环境变量 ADMIN_TOKEN）
if "%ADMIN_TOKEN%" == "" set "ADMIN_TOKEN=shibosi-admin-2026"

:: ============ 环境检测 ============
set "GO_CMD="
where go >nul 2>&1
if not errorlevel 1 set "GO_CMD=go"

set "PYTHON_CMD="
where python >nul 2>&1
if not errorlevel 1 (
    set "PYTHON_CMD=python"
) else (
    where py >nul 2>&1
    if not errorlevel 1 set "PYTHON_CMD=py -3"
)

:: ============ 服务配置 ============
set "S1_NAME=搜索引擎"
set "S1_PORT=8081"
set "S1_ADMIN_PORT=8091"
set "S1_EXE=%ROOT%\Service\SearchEngine\searchengine.exe"
set "S1_DIR=%ROOT%\Service\SearchEngine"

set "S2_NAME=晚报服务"
set "S2_PORT=8082"
set "S2_ADMIN_PORT=8092"
set "S2_EXE=%ROOT%\Service\Newspaper\newspaper.exe"
set "S2_DIR=%ROOT%\Service\Newspaper"

set "S3_NAME=管理后台"
set "S3_PORT=8084"
set "S3_EXE=%ROOT%\Service\admin-core\admin-core.exe"
set "S3_DIR=%ROOT%\Service\admin-core"

:MENU
cls
echo ============================================
echo          市舶司 统一启动器 v3.2
echo ============================================
echo.
echo   +-- 快速操作 ----------------------------+
echo   ^|  [1] 启动全部服务（含静态文件）         ^|
echo   ^|  [2] 启动后端服务                       ^|
echo   ^|  [3] 停止全部服务                       ^|
echo   ^|  [4] 重启全部服务                       ^|
echo   +-- 单服务启动/停止切换 -------------------+
echo   ^|  [5] 搜索引擎    (8081)                 ^|
echo   ^|  [6] 晚报服务    (8082)                 ^|
echo   ^|  [7] 管理后台    (8084)                 ^|
echo   ^|  [8] 静态文件    (8080)                 ^|
echo   +-- 运维面板（独立端口+Token）-------------+
echo   ^|  [E] 搜索引擎运维 (127.0.0.1:8091)      ^|
echo   ^|  [F] 晚报运维     (127.0.0.1:8092)      ^|
echo   +-- 工具 -------------------------------+
echo   ^|  [A] 查看服务状态                       ^|
echo   ^|  [B] 编译全部服务                       ^|
echo   ^|  [C] 打开管理后台                       ^|
echo   ^|  [D] 当前首页                           ^|
echo   ^|  [0] 退出                               ^|
echo   +----------------------------------------+
echo.

call :STATUS_LINE %S1_PORT% "%S1_NAME%"
call :STATUS_LINE %S2_PORT% "%S2_NAME%"
call :STATUS_LINE %S3_PORT% "%S3_NAME%"
call :STATUS_LINE 8080 "静态文件"
echo   运维面板:
call :STATUS_LINE_LOCAL %S1_ADMIN_PORT% "  搜索引擎运维"
call :STATUS_LINE_LOCAL %S2_ADMIN_PORT% "  晚报运维"
echo.

set "CHOICE="
set /p "CHOICE=请选择操作: "

if "%CHOICE%" == "1" goto START_ALL
if "%CHOICE%" == "2" goto START_BACKEND
if "%CHOICE%" == "3" goto STOP_ALL_MENU
if "%CHOICE%" == "4" goto RESTART_ALL
if "%CHOICE%" == "5" goto TOGGLE_8081
if "%CHOICE%" == "6" goto TOGGLE_8082
if "%CHOICE%" == "7" goto TOGGLE_8084
if "%CHOICE%" == "8" goto TOGGLE_8080
if /i "%CHOICE%" == "A" goto CHECK_STATUS
if /i "%CHOICE%" == "B" goto BUILD_ALL
if /i "%CHOICE%" == "C" goto OPEN_ADMIN
if /i "%CHOICE%" == "D" goto OPEN_FRONTEND
if /i "%CHOICE%" == "E" goto OPEN_SE_ADMIN
if /i "%CHOICE%" == "F" goto OPEN_NP_ADMIN
if "%CHOICE%" == "0" goto END
goto MENU

:: ============================================
:: 状态行（监听 0.0.0.0 的端口）
:: ============================================
:STATUS_LINE
netstat -ano | findstr ":%~1 " | findstr LISTENING >nul 2>&1
if not errorlevel 1 (
    echo     [运行中] %~2
) else (
    echo     [未启动] %~2
)
exit /b 0

:: 仅本机端口状态检测（127.0.0.1）
:STATUS_LINE_LOCAL
netstat -ano | findstr "127.0.0.1:%~1 " | findstr LISTENING >nul 2>&1
if not errorlevel 1 (
    echo     [运行中] %~2 (127.0.0.1:%~1)
) else (
    echo     [未启动] %~2 (127.0.0.1:%~1)
)
exit /b 0

:: ============================================
:: 端口检测（返回 errorLevel: 0=运行中, 1=未运行）
:: ============================================
:IS_RUNNING
netstat -ano | findstr ":%~1 " | findstr LISTENING >nul 2>&1
if not errorlevel 1 ( exit /b 0 ) else ( exit /b 1 )

:: 释放端口（杀掉指定端口的监听进程）
:: 使用临时文件避免 for /f 内联管道的编码问题
:FREE_PORT
set "FP=%~1"
set "FOUND_PID="
set "TMPFILE=%TEMP%\portcheck_%RANDOM%.txt"
netstat -ano | findstr ":%FP% " | findstr LISTENING > "%TMPFILE%" 2>&1
for /f "tokens=5" %%a in ("%TMPFILE%") do (
    set "FOUND_PID=%%a"
    goto :FP_DONE
)
:FP_DONE
del "%TMPFILE%" >nul 2>&1
if not "%FOUND_PID%" == "" (
    echo   [释放] 端口 %FP% (PID: %FOUND_PID%)
    taskkill /F /PID %FOUND_PID% >nul 2>&1
)
exit /b 0

:: ============================================
:: 启动单个服务
:: 参数: %1=端口 %2=服务名 %3=exe路径 %4=工作目录
:: ============================================
:START_ONE
echo [启动] %~2 (端口 %~1)...
call :FREE_PORT %~1

:: 方式1: 指定的 exe 直接启动
if not "%~3" == "" if exist "%~3" (
    start "%~2" /D "%~4" "%~3"
    goto :WAIT_ONE
)

:: 方式2: 扫描目录下的 exe（排除 .exe~ 临时文件）
setlocal enabledelayedexpansion
set "FOUND_EXE="
for /f "delims=" %%f in ('dir /b /a-d "%~4\*.exe" 2^>nul') do (
    set "fname=%%f"
    if "!fname:~-1!" NEQ "~" (
        if not "!fname!" == "" (
            set "FOUND_EXE=%~4\%%f"
            goto :FOUND_ONE
        )
    )
)
:FOUND_ONE
if not "!FOUND_EXE!" == "" (
    start "%~2" /D "%~4" "!FOUND_EXE!"
    endlocal
    goto :WAIT_ONE
)
endlocal

:: 方式3: go run
if not "%GO_CMD%" == "" if exist "%~4\main.go" (
    start "%~2" /D "%~4" cmd /c "%GO_CMD% run main.go"
    goto :WAIT_ONE
)

echo   [失败] 未找到可执行文件，请先编译（菜单 [B]）
pause
exit /b 0

:WAIT_ONE
set /a RETRY=0
:CHECK_ONE
timeout /t 1 /nobreak >nul
call :IS_RUNNING %~1
if not errorlevel 1 (
    echo   [成功] %~2 已启动 - http://localhost:%~1
    exit /b 0
)
set /a RETRY+=1
if %RETRY% LSS 8 goto CHECK_ONE
echo   [警告] %~2 启动较慢，请稍后查看状态
exit /b 0

:: ============================================
:: 启动静态文件服务
:: ============================================
:START_STATIC
if "%PYTHON_CMD%" == "" (
    echo   [失败] 未检测到 Python，无法启动静态文件服务
    exit /b 1
)
call :FREE_PORT 8080
start "静态文件服务" /D "%ROOT%" %PYTHON_CMD% http_server.py 8080
timeout /t 1 /nobreak >nul
call :IS_RUNNING 8080
if not errorlevel 1 (
    echo   [成功] 静态文件服务 - http://localhost:8080
    exit /b 0
)
echo   [失败] 静态文件服务启动失败
exit /b 1

:: ============================================
:: [1] 启动全部服务
:: ============================================
:START_ALL
cls
echo ============================================
echo           启动全部服务
echo ============================================
echo.
call :START_ONE %S1_PORT% "%S1_NAME%" "%S1_EXE%" "%S1_DIR%"
call :START_ONE %S2_PORT% "%S2_NAME%" "%S2_EXE%" "%S2_DIR%"
call :START_ONE %S3_PORT% "%S3_NAME%" "%S3_EXE%" "%S3_DIR%"
call :START_STATIC
echo.
echo [提示] 运维面板（独立端口+Token）：
echo   搜索引擎: http://127.0.0.1:%S1_ADMIN_PORT%/admin
echo   晚报服务: http://127.0.0.1:%S2_ADMIN_PORT%/admin
echo   Token: %ADMIN_TOKEN%
echo.
pause
goto MENU

:: ============================================
:: [2] 启动后端服务
:: ============================================
:START_BACKEND
cls
echo ============================================
echo         启动后端服务（不含静态文件）
echo ============================================
echo.
call :START_ONE %S1_PORT% "%S1_NAME%" "%S1_EXE%" "%S1_DIR%"
call :START_ONE %S2_PORT% "%S2_NAME%" "%S2_EXE%" "%S2_DIR%"
call :START_ONE %S3_PORT% "%S3_NAME%" "%S3_EXE%" "%S3_DIR%"
echo.
echo [提示] 运维面板随业务端口自动启动（127.0.0.1:8091 / 8092）
echo.
pause
goto MENU

:: ============================================
:: [3] 停止全部服务（菜单入口）
:: ============================================
:STOP_ALL_MENU
cls
call :STOP_ALL
echo.
pause
goto MENU

:: 停止全部服务（内部调用）
:STOP_ALL
echo ============================================
echo           停止全部服务
echo ============================================
echo.
call :STOP_PORT 8084 "管理后台"
call :STOP_PORT 8082 "晚报服务"
call :STOP_PORT 8081 "搜索引擎"
call :STOP_PORT 8080 "静态文件"

:: 兜底：按进程名清理
for %%N in (admin-core.exe newspaper.exe searchengine.exe) do (
    tasklist /FI "IMAGENAME eq %%N" 2>nul | find /i "%%N" >nul 2>&1
    if not errorlevel 1 (
        taskkill /F /IM "%%N" >nul 2>&1
    )
)
echo.
echo [完成] 全部服务已停止
exit /b 0

:: 停止监听指定端口的服务
:: 使用临时文件避免 for /f 内联管道的编码问题
:STOP_PORT
set "FOUND_PID="
set "SPTMP=%TEMP%\stopport_%RANDOM%.txt"
netstat -ano | findstr ":%~1 " | findstr LISTENING > "%SPTMP%" 2>&1
for /f "tokens=5" %%a in ("%SPTMP%") do (
    set "FOUND_PID=%%a"
    goto :SP_DONE
)
:SP_DONE
del "%SPTMP%" >nul 2>&1
if not "%FOUND_PID%" == "" (
    echo   [停止] %~2 (端口 %~1) - PID: %FOUND_PID%
    taskkill /F /PID %FOUND_PID% >nul 2>&1
) else (
    echo   [跳过] %~2 (端口 %~1) - 未运行
)
exit /b 0

:: ============================================
:: [4] 重启全部服务
:: ============================================
:RESTART_ALL
cls
echo ============================================
echo           重启全部服务
echo ============================================
echo.
echo [1/2] 停止全部服务...
call :STOP_ALL
echo.
echo [2/2] 重新启动全部服务...
timeout /t 2 /nobreak >nul
goto START_ALL

:: ============================================
:: [5-8] 单服务启动/停止切换
:: ============================================
:TOGGLE_8081
call :TOGGLE_SERVICE %S1_PORT% "%S1_NAME%" "%S1_EXE%" "%S1_DIR%"
pause
goto MENU

:TOGGLE_8082
call :TOGGLE_SERVICE %S2_PORT% "%S2_NAME%" "%S2_EXE%" "%S2_DIR%"
pause
goto MENU

:TOGGLE_8084
call :TOGGLE_SERVICE %S3_PORT% "%S3_NAME%" "%S3_EXE%" "%S3_DIR%"
pause
goto MENU

:TOGGLE_8080
call :IS_RUNNING 8080
if not errorlevel 1 (
    echo [停止] 静态文件 (端口 8080)...
    call :FREE_PORT 8080
    echo   [完成] 已停止
) else (
    echo [启动] 静态文件 (端口 8080)...
    call :START_STATIC
)
pause
goto MENU

:: 切换：运行中则停止，未运行则启动
:TOGGLE_SERVICE
call :IS_RUNNING %~1
if not errorlevel 1 (
    echo [停止] %~2 (端口 %~1)...
    call :FREE_PORT %~1
    echo   [完成] 已停止
) else (
    call :START_ONE %~1 "%~2" "%~3" "%~4"
)
exit /b 0

:: ============================================
:: [E] 打开搜索引擎运维面板
:: ============================================
:OPEN_SE_ADMIN
cls
call :IS_RUNNING %S1_PORT%
if not errorlevel 1 (
    echo [打开] 搜索引擎运维面板 http://127.0.0.1:%S1_ADMIN_PORT%/admin
    echo        Token: %ADMIN_TOKEN%
    start http://127.0.0.1:%S1_ADMIN_PORT%/admin?token=%ADMIN_TOKEN%
) else (
    echo [提示] 搜索引擎未运行，正在启动...
    call :START_ONE %S1_PORT% "%S1_NAME%" "%S1_EXE%" "%S1_DIR%"
    timeout /t 2 /nobreak >nul
    start http://127.0.0.1:%S1_ADMIN_PORT%/admin?token=%ADMIN_TOKEN%
)
echo.
pause
goto MENU

:: ============================================
:: [F] 打开晚报运维面板
:: ============================================
:OPEN_NP_ADMIN
cls
call :IS_RUNNING %S2_PORT%
if not errorlevel 1 (
    echo [打开] 晚报运维面板 http://127.0.0.1:%S2_ADMIN_PORT%/admin
    echo        Token: %ADMIN_TOKEN%
    start http://127.0.0.1:%S2_ADMIN_PORT%/admin?token=%ADMIN_TOKEN%
) else (
    echo [提示] 晚报服务未运行，正在启动...
    call :START_ONE %S2_PORT% "%S2_NAME%" "%S2_EXE%" "%S2_DIR%"
    timeout /t 2 /nobreak >nul
    start http://127.0.0.1:%S2_ADMIN_PORT%/admin?token=%ADMIN_TOKEN%
)
echo.
pause
goto MENU

:: ============================================
:: [A] 查看服务状态
:: ============================================
:CHECK_STATUS
cls
echo ============================================
echo           服务状态总览
echo ============================================
echo.
echo   业务端口:
call :FULL_STATUS %S1_PORT% "%S1_NAME%"
call :FULL_STATUS %S2_PORT% "%S2_NAME%"
call :FULL_STATUS %S3_PORT% "%S3_NAME%"
call :FULL_STATUS 8080 "静态文件"
echo.
echo   运维端口（仅本机）:
call :FULL_STATUS_LOCAL %S1_ADMIN_PORT% "搜索引擎运维"
call :FULL_STATUS_LOCAL %S2_ADMIN_PORT% "晚报运维"
echo.
echo   运维 Token: %ADMIN_TOKEN%
echo ============================================
echo.
pause
goto MENU

:: 完整状态显示（含 PID）
:: 使用临时文件避免编码问题
:FULL_STATUS
set "FS_FOUND="
set "FSTMP=%TEMP%\fscheck_%RANDOM%.txt"
netstat -ano | findstr ":%~1 " | findstr LISTENING > "%FSTMP%" 2>&1
if not exist "%FSTMP%" (
    echo   [未启动] %~2 (端口 %~1)
    exit /b 0
)
set "FS_LINE="
set /p FS_LINE=<"%FSTMP%"
del "%FSTMP%" >nul 2>&1
if "%FS_LINE%" == "" (
    echo   [未启动] %~2 (端口 %~1)
    exit /b 0
)
echo   [运行中] %~2 (端口 %~1)
echo            ^|  http://localhost:%~1
exit /b 0

:: 完整状态显示（仅本机端口）
:FULL_STATUS_LOCAL
set "FSL_FOUND="
set "FSLTMP=%TEMP%\fslcheck_%RANDOM%.txt"
netstat -ano | findstr "127.0.0.1:%~1 " | findstr LISTENING > "%FSLTMP%" 2>&1
if not exist "%FSLTMP%" (
    echo   [未启动] %~2 (127.0.0.1:%~1)
    exit /b 0
)
set "FSL_LINE="
set /p FSL_LINE=<"%FSLTMP%"
del "%FSLTMP%" >nul 2>&1
if "%FSL_LINE%" == "" (
    echo   [未启动] %~2 (127.0.0.1:%~1)
    exit /b 0
)
echo   [运行中] %~2 (127.0.0.1:%~1)
echo            ^|  http://127.0.0.1:%~1/admin
exit /b 0

:: ============================================
:: [B] 编译全部服务
:: ============================================
:BUILD_ALL
cls
echo ============================================
echo           编译全部服务
echo ============================================
echo.

if "%GO_CMD%" == "" (
    echo [错误] 未检测到 Go，无法编译
    echo.
    pause
    goto MENU
)

echo [1/4] 停止正在运行的服务...
call :STOP_ALL >nul 2>&1
timeout /t 1 /nobreak >nul

echo.
echo [2/4] 编译 搜索引擎...
pushd "%S1_DIR%"
%GO_CMD% build -o searchengine.exe . 2>&1
if not errorlevel 1 (echo   [成功] searchengine.exe) else (echo   [失败] 搜索引擎)
popd

echo [3/4] 编译 晚报服务...
pushd "%S2_DIR%"
%GO_CMD% build -o newspaper.exe . 2>&1
if not errorlevel 1 (echo   [成功] newspaper.exe) else (echo   [失败] 晚报服务)
popd

echo [4/4] 编译 管理后台...
pushd "%S3_DIR%"
%GO_CMD% build -o admin-core.exe . 2>&1
if not errorlevel 1 (echo   [成功] admin-core.exe) else (echo   [失败] 管理后台)
popd

echo.
echo [完成] 编译结束
echo.
pause
goto MENU

:: ============================================
:: [C] 打开管理后台
:: ============================================
:OPEN_ADMIN
cls
call :IS_RUNNING %S3_PORT%
if not errorlevel 1 (
    echo [打开] 管理后台 http://localhost:%S3_PORT%/admin
    start http://localhost:%S3_PORT%/admin
) else (
    echo [提示] 管理后台未运行，正在启动...
    call :START_ONE %S3_PORT% "%S3_NAME%" "%S3_EXE%" "%S3_DIR%"
    timeout /t 1 /nobreak >nul
    start http://localhost:%S3_PORT%/admin
)
echo.
pause
goto MENU

:: ============================================
:: [D] 当前首页
:: ============================================
:OPEN_FRONTEND
cls
call :IS_RUNNING 8080
if not errorlevel 1 (
    echo [打开] 前端首页 http://localhost:8080
    start http://localhost:8080
) else (
    if not "%PYTHON_CMD%" == "" (
        echo [启动] 静态文件服务...
        call :START_STATIC
        timeout /t 1 /nobreak >nul
        start http://localhost:8080
    ) else (
        echo [打开] 本地文件 index.html
        start "" "%ROOT%\index.html"
    )
)
echo.
pause
goto MENU

:: ============================================
:: [0] 退出
:: ============================================
:END
echo.
echo 再见！
timeout /t 1 /nobreak >nul
exit /b 0
