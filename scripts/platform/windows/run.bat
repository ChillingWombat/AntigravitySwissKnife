@echo off
set DIR=%~dp0..\..\release\windows\
if exist "%DIR%Antigravity Swiss Knife.exe" (
  start "" "%DIR%Antigravity Swiss Knife.exe" %*
) else if exist "%DIR%win-unpacked\Antigravity Swiss Knife.exe" (
  start "" "%DIR%win-unpacked\Antigravity Swiss Knife.exe" %*
) else (
  start "" "%DIR%app\Antigravity Swiss Knife.exe" %*
)
