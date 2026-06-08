@echo off
REM ============================================================
REM  ZapTerm - Instalador para Windows
REM ============================================================
REM  Cria um atalho "ZapTerm" no Menu Iniciar (tela inicial),
REM  com o icone do app. Ao clicar, abre um terminal e roda o
REM  WhatsApp na linha de comando.
REM
REM  NAO precisa ter o Go instalado: o ZapTerm.exe ja vem
REM  pronto nesta mesma pasta.
REM
REM  Como usar: de dois cliques neste arquivo.
REM ============================================================
setlocal
set "ZAP_DIR=%~dp0"
set "ZAP_EXE=%ZAP_DIR%ZapTerm.exe"

if not exist "%ZAP_EXE%" (
    echo.
    echo  ERRO: nao encontrei "ZapTerm.exe" nesta pasta.
    echo.
    pause
    exit /b 1
)

set "STARTMENU=%APPDATA%\Microsoft\Windows\Start Menu\Programs"
set "LINK=%STARTMENU%\ZapTerm.lnk"

echo ==> Criando atalho no Menu Iniciar...

REM O proprio ZapTerm.exe ja tem o icone embutido (IconLocation = exe,0).
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$w = New-Object -ComObject WScript.Shell;" ^
  "$s = $w.CreateShortcut('%LINK%');" ^
  "$s.TargetPath = '%ZAP_EXE%';" ^
  "$s.WorkingDirectory = '%ZAP_DIR%';" ^
  "$s.Description = 'ZapTerm - use o WhatsApp pelo terminal (mensagens, imagens, audios e documentos)';" ^
  "$s.IconLocation = '%ZAP_EXE%,0';" ^
  "$s.Save()"

if exist "%LINK%" (
    echo.
    echo  Pronto! Procure por "ZapTerm" no Menu Iniciar.
    echo.
) else (
    echo.
    echo  Nao consegui criar o atalho. Tente executar como administrador.
    echo.
)

pause
endlocal
