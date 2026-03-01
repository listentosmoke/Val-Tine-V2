@echo off
REM NodePulse Multi-Platform Builder for Windows
REM Builds agents for all supported platforms

setlocal

echo ═══════════════════════════════════════════════════════════
echo   NodePulse Multi-Platform Builder
echo ═══════════════════════════════════════════════════════════
echo.

if not exist output mkdir output

echo [*] Building Windows x64...
python builder.py --os windows --arch amd64

echo.
echo [*] Building Windows x86...
python builder.py --os windows --arch 386

echo.
echo [*] Building Linux x64...
python builder.py --os linux --arch amd64

echo.
echo [*] Building Linux ARM64...
python builder.py --os linux --arch arm64

echo.
echo [*] Building macOS x64...
python builder.py --os darwin --arch amd64

echo.
echo [*] Building macOS ARM64...
python builder.py --os darwin --arch arm64

echo.
echo ═══════════════════════════════════════════════════════════
echo   All builds complete!
echo ═══════════════════════════════════════════════════════════
echo.
dir /b output

endlocal
