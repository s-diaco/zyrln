@echo off
title Zyrln Control Center
echo Starting Zyrln...
if not exist zyrln.exe (
    echo [ERROR] zyrln.exe not found! 
    echo Please make sure you have the zyrln.exe file in this folder.
    pause
    exit /b
)
start "" zyrln.exe -gui
echo Zyrln is running in the background. 
echo Your browser should open automatically.
echo Keep this window open while using Zyrln.
pause
