@echo off
setlocal
set PID_FILE=%APPDATA%\CursorPro3\control_server.pid
if not exist "%PID_FILE%" (
  echo CursorPro3 control server is not running.
  exit /b 0
)
set /p PID=<"%PID_FILE%"
taskkill /PID %PID% /F
echo Stopped CursorPro3 control server.
