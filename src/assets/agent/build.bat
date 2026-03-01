@echo off
REM NodePulse Quick Builder for Windows
REM Usage: build.bat [os] [arch] [output_name]

setlocal

set OS=%1
set ARCH=%2
set NAME=%3

if "%OS%"=="" set OS=windows
if "%ARCH%"=="" set ARCH=amd64

echo ═══════════════════════════════════════════
echo   NodePulse Agent Builder
echo   Target: %OS%/%ARCH%
echo ═══════════════════════════════════════════

if "%NAME%"=="" (
    python builder.py --os %OS% --arch %ARCH%
) else (
    python builder.py --os %OS% --arch %ARCH% --out %NAME%
)

endlocal
