@echo off
cd /d "%~dp0backend"
echo Starting FollowITup v0.8.0...
start http://localhost:8080
followitup.exe config.yaml
pause
