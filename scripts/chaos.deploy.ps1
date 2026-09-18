$uiDir = "D:\project\chaos\chaos-lib\chaos-ui"
$serverDir = "D:\project\chaos\chaos-lib\chaos-go"
$extDir = "D:\project\chaos\chaos-lib\chaos-browser-extension"
$outputPath = "D:\data\chaos\chaos-go.exe"
$uiOutputDir = Join-Path (Split-Path $outputPath -Parent) "ui"
$extOutputDir = "D:\data\chaos\chrome-mv3"

$totalSw = [System.Diagnostics.Stopwatch]::StartNew()

# ── 前端构建：后台作业，与后端 go build 并行 ───────────────────────────
$uiJob = Start-Job -Name ui-build -ScriptBlock {
    param($dir)
    Set-Location $dir
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $global:LASTEXITCODE = $null
    pnpm install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) { throw "pnpm install 失败 (exit $LASTEXITCODE)" }
    $global:LASTEXITCODE = $null
    pnpm run build
    if ($LASTEXITCODE -ne 0) { throw "pnpm build 失败 (exit $LASTEXITCODE)" }
    $sw.Stop()
    [pscustomobject]@{ Seconds = [math]::Round($sw.Elapsed.TotalSeconds, 1) }
} -ArgumentList $uiDir

# ── 浏览器扩展构建：后台作业，与前端、后端并行 ──────────────────────────
$extJob = Start-Job -Name ext-build -ScriptBlock {
    param($dir)
    Set-Location $dir
    $sw = [System.Diagnostics.Stopwatch]::StartNew()
    $global:LASTEXITCODE = $null
    pnpm install --frozen-lockfile
    if ($LASTEXITCODE -ne 0) { throw "pnpm install 失败 (exit $LASTEXITCODE)" }
    $global:LASTEXITCODE = $null
    pnpm run build
    if ($LASTEXITCODE -ne 0) { throw "pnpm build 失败 (exit $LASTEXITCODE)" }
    $sw.Stop()
    [pscustomobject]@{ Seconds = [math]::Round($sw.Elapsed.TotalSeconds, 1) }
} -ArgumentList $extDir

# ── 后端构建：主线程执行，与前端并行 ───────────────────────────────────
$goSw = [System.Diagnostics.Stopwatch]::StartNew()
Push-Location $serverDir
$global:LASTEXITCODE = $null
go build -o $outputPath .\cmd\server\main.go
$goExit = $LASTEXITCODE
Pop-Location
$goSeconds = [math]::Round($goSw.Elapsed.TotalSeconds, 1)
if ($goExit -ne 0) {
    Write-Error "后端构建失败 (exit $goExit)，中止部署"
    Stop-Job $uiJob | Out-Null
    Remove-Job $uiJob -Force
    Stop-Job $extJob | Out-Null
    Remove-Job $extJob -Force
    exit 1
}

# ── 等待前端构建完成（pnpm 的 WARN 走 stderr，不能靠 -ErrorAction 判失败）──
try {
    $uiResult = Receive-Job -Job $uiJob -Wait
    $uiFailed = ($uiJob.State -eq 'Failed')
} finally {
    Remove-Job $uiJob -Force
}
if ($uiFailed) {
    Write-Error "前端构建失败，中止部署（详见上方 pnpm 错误输出）"
    Stop-Job $extJob | Out-Null
    Remove-Job $extJob -Force
    exit 1
}
if (-not $uiResult) {
    Write-Error "前端构建未产出结果，中止部署"
    Stop-Job $extJob | Out-Null
    Remove-Job $extJob -Force
    exit 1
}
$uiSeconds = $uiResult.Seconds

# ── 等待扩展构建完成 ─────────────────────────────────────────────────
try {
    $extResult = Receive-Job -Job $extJob -Wait
    $extFailed = ($extJob.State -eq 'Failed')
} finally {
    Remove-Job $extJob -Force
}
if ($extFailed) {
    Write-Error "浏览器扩展构建失败，中止部署（详见上方 pnpm 错误输出）"
    exit 1
}
if (-not $extResult) {
    Write-Error "浏览器扩展构建未产出结果，中止部署"
    exit 1
}
$extSeconds = $extResult.Seconds

# ── 复制前端产物 dist → ui（robocopy 更快且无 PowerShell 通配符展开问题）──
$copySw = [System.Diagnostics.Stopwatch]::StartNew()
if (Test-Path $uiOutputDir) {
    Remove-Item -Path $uiOutputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $uiOutputDir -Force | Out-Null
$global:LASTEXITCODE = $null
robocopy "$uiDir\dist" $uiOutputDir /E /NFL /NDL /NJH /NJS /NP | Out-Null
$copyExit = $LASTEXITCODE
$copySeconds = [math]::Round($copySw.Elapsed.TotalSeconds, 1)
if ($copyExit -ge 8) {
    Write-Error "复制前端产物失败 (robocopy exit $copyExit)"
    exit 1
}

# ── 复制扩展产物 chrome-mv3 → D:\data\chaos\chrome-mv3 ────────────────
$extCopySw = [System.Diagnostics.Stopwatch]::StartNew()
if (Test-Path $extOutputDir) {
    Remove-Item -Path $extOutputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $extOutputDir -Force | Out-Null
$global:LASTEXITCODE = $null
robocopy "$extDir\.output\chrome-mv3" $extOutputDir /E /NFL /NDL /NJH /NJS /NP | Out-Null
$extCopyExit = $LASTEXITCODE
$extCopySeconds = [math]::Round($extCopySw.Elapsed.TotalSeconds, 1)
if ($extCopyExit -ge 8) {
    Write-Error "复制扩展产物失败 (robocopy exit $extCopyExit)"
    exit 1
}

# ── 重启服务 ──────────────────────────────────────────────────────────
$restartSw = [System.Diagnostics.Stopwatch]::StartNew()
nssm restart chaos
$restartSeconds = [math]::Round($restartSw.Elapsed.TotalSeconds, 1)

$totalSeconds = [math]::Round($totalSw.Elapsed.TotalSeconds, 1)

Write-Host ""
Write-Host "── 部署计时 ────────────────────────────"
Write-Host ("  前端构建 (pnpm install+build): {0,6}s" -f $uiSeconds)
Write-Host ("  后端构建 (go build):           {0,6}s" -f $goSeconds)
Write-Host ("  复制前端产物 (dist→ui):        {0,6}s" -f $copySeconds)
Write-Host ("  扩展构建 (pnpm install+build): {0,6}s" -f $extSeconds)
Write-Host ("  复制扩展产物 (chrome-mv3):     {0,6}s" -f $extCopySeconds)
Write-Host ("  重启服务 (nssm restart):       {0,6}s" -f $restartSeconds)
Write-Host ("  总体耗时:                      {0,6}s" -f $totalSeconds)
Write-Host "────────────────────────────────────────"
Write-Host "✅ 部署完成: $outputPath + $uiOutputDir + $extOutputDir"
