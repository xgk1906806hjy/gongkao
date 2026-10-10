$ErrorActionPreference = "Stop"
$Root = Resolve-Path (Join-Path $PSScriptRoot "..")
$ServerDir = Join-Path $Root "server"
$RedisExe = "C:\Program Files\Redis\redis-server.exe"
$RedisConf = Join-Path $Root "deploy\redis.windows.conf"
$RedisCli = "C:\Program Files\Redis\redis-cli.exe"
$LogDir = Join-Path $ServerDir "logs"
New-Item -ItemType Directory -Force -Path $LogDir | Out-Null

function Test-Redis {
  if (-not (Test-Path $RedisCli)) { return $false }
  try {
    $r = & $RedisCli ping 2>$null
    return ($r -eq "PONG")
  } catch { return $false }
}

Write-Host "==> check Redis"
if (Test-Redis) {
  Write-Host "Redis already running (PONG)"
} elseif (Test-Path $RedisExe) {
  Write-Host "Starting Redis..."
  Start-Process -FilePath $RedisExe -ArgumentList "`"$RedisConf`"" -WorkingDirectory (Split-Path $RedisExe) -WindowStyle Hidden
  Start-Sleep -Seconds 2
  if (Test-Redis) { Write-Host "Redis started" } else { Write-Host "WARN: Redis not ready; remind falls back to DB" }
} else {
  Write-Host "WARN: Redis not installed; remind falls back to DB"
}

Push-Location $ServerDir
try {
  if (-not (Test-Path ".\bin\api.exe") -or -not (Test-Path ".\bin\worker.exe")) {
    Write-Host "==> building"
    New-Item -ItemType Directory -Force -Path bin | Out-Null
    go build -o bin\api.exe ./cmd/api
    go build -o bin\worker.exe ./cmd/worker
  }
} finally { Pop-Location }

function Stop-NamedOnPort([int]$Port, [string]$Name) {
  $lines = netstat -ano | Select-String ":$Port\s+.*LISTENING"
  foreach ($line in $lines) {
    $parts = ($line.ToString() -split "\s+") | Where-Object { $_ -ne "" }
    $procId = [int]$parts[-1]
    $p = Get-Process -Id $procId -ErrorAction SilentlyContinue
    if ($p -and $p.ProcessName -eq $Name) {
      Write-Host "Stopping $Name pid=$procId"
      Stop-Process -Id $procId -Force -ErrorAction SilentlyContinue
    }
  }
}

Write-Host "==> restart API"
Stop-NamedOnPort 8080 "api"
Get-Process -Name "worker" -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

Start-Process -FilePath (Join-Path $ServerDir "bin\api.exe") -WorkingDirectory $ServerDir `
  -RedirectStandardOutput (Join-Path $LogDir "api.out.log") `
  -RedirectStandardError (Join-Path $LogDir "api.err.log") -WindowStyle Hidden
Start-Process -FilePath (Join-Path $ServerDir "bin\worker.exe") -WorkingDirectory $ServerDir `
  -RedirectStandardOutput (Join-Path $LogDir "worker.out.log") `
  -RedirectStandardError (Join-Path $LogDir "worker.err.log") -WindowStyle Hidden

$ok = $false
for ($i = 0; $i -lt 15; $i++) {
  Start-Sleep -Milliseconds 400
  try {
    $h = Invoke-RestMethod "http://127.0.0.1:8080/health"
    if ($h.code -eq 0) { $ok = $true; break }
  } catch {}
}
if (-not $ok) {
  Write-Host "API failed. Logs:"
  Get-Content (Join-Path $LogDir "api.err.log") -ErrorAction SilentlyContinue
  Get-Content (Join-Path $LogDir "api.out.log") -ErrorAction SilentlyContinue | Select-Object -Last 30
  exit 1
}
Write-Host "API OK  http://127.0.0.1:8080/health"

# frontend preview (browser). Real miniapp still opens in WeChat DevTools.
$webDir = Join-Path $Root "web"
$webPort = 5173
$webListen = netstat -ano | Select-String ":$webPort\s+.*LISTENING"
if (-not $webListen) {
  Write-Host "==> start Web preview :$webPort"
  Start-Process -FilePath "python" -ArgumentList "-m","http.server","$webPort","--bind","127.0.0.1" `
    -WorkingDirectory $webDir -WindowStyle Hidden
  Start-Sleep -Seconds 1
} else {
  Write-Host "Web preview already on :$webPort"
}
Start-Process "http://127.0.0.1:$webPort/"
Write-Host "Web OK  http://127.0.0.1:$webPort/"
Write-Host "Miniapp: open folder miniapp/ in WeChat DevTools"
Write-Host "Smoke:  powershell -File scripts\smoke.ps1"