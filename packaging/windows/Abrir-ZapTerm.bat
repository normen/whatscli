@echo off
REM ============================================================
REM  ZapTerm - Abrir agora (Windows)
REM ============================================================
REM  Abre o ZapTerm numa janela de terminal. Basta dar dois
REM  cliques neste arquivo. NAO precisa ter o Go instalado.
REM
REM  Deixe este .bat na MESMA PASTA que o ZapTerm.exe.
REM ============================================================
set "ZAP_DIR=%~dp0"
set "ZAP_EXE=%ZAP_DIR%ZapTerm.exe"

if not exist "%ZAP_EXE%" (
    echo.
    echo  ERRO: nao encontrei "ZapTerm.exe" nesta pasta.
    echo.
    pause
    exit /b 1
)

start "ZapTerm - WhatsApp no Terminal" cmd /k "cd /d "%ZAP_DIR%" && "%ZAP_EXE%""
