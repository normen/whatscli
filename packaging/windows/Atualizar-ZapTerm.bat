@echo off
REM ============================================================
REM  ZapTerm - Atualizador (Windows)
REM ============================================================
REM  Baixa a versao mais recente publicada no GitHub Releases,
REM  substitui os arquivos desta pasta e atualiza o atalho do
REM  Menu Iniciar.
REM
REM  Como usar: feche o ZapTerm e de dois cliques neste arquivo.
REM
REM  Para apontar para outro fork/repositorio, defina a variavel
REM  de ambiente ZAPTERM_REPO=usuario/repo antes de rodar.
REM ============================================================
setlocal
set "ZAP_DIR=%~dp0"
if "%ZAPTERM_REPO%"=="" set "ZAPTERM_REPO=RafaelProfMgz/whatscli"

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$ErrorActionPreference='Stop';" ^
  "$repo=$env:ZAPTERM_REPO;" ^
  "Write-Host ('==> Procurando a versao mais recente em github.com/'+$repo);" ^
  "$rel=Invoke-RestMethod ('https://api.github.com/repos/'+$repo+'/releases/latest');" ^
  "$asset=$rel.assets | Where-Object { $_.name -like 'ZapTerm-*-windows.zip' } | Select-Object -First 1;" ^
  "if(-not $asset){ throw 'Nao achei um ZapTerm-*-windows.zip na ultima release.' };" ^
  "$tmp=Join-Path $env:TEMP 'zapterm-update';" ^
  "Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue;" ^
  "New-Item -ItemType Directory -Path $tmp | Out-Null;" ^
  "$zip=Join-Path $tmp $asset.name;" ^
  "Write-Host ('==> Baixando '+$asset.browser_download_url);" ^
  "Invoke-WebRequest $asset.browser_download_url -OutFile $zip;" ^
  "Expand-Archive $zip -DestinationPath $tmp;" ^
  "$src=Get-ChildItem $tmp -Directory | Where-Object Name -like 'ZapTerm-*' | Select-Object -First 1;" ^
  "if(-not $src){ throw 'O zip baixado nao contem uma pasta ZapTerm-*.' };" ^
  "Write-Host '==> Substituindo os arquivos...';" ^
  "Get-ChildItem $src.FullName | Where-Object Name -ne 'Atualizar-ZapTerm.bat' | Copy-Item -Destination '%ZAP_DIR%.' -Force;" ^
  "Write-Host 'Arquivos atualizados.'"

if errorlevel 1 (
    echo.
    echo  A atualizacao falhou. Veja a mensagem acima.
    echo  Dica: feche o ZapTerm antes de atualizar.
    echo.
    pause
    exit /b 1
)

echo.
echo ==^> Atualizando o atalho do Menu Iniciar...
call "%ZAP_DIR%Instalar-no-menu-iniciar.bat"
endlocal
