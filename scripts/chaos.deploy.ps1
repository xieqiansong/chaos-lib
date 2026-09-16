$uiDir = "D:\project\chaos\chaos-lib\chaos-ui"
$serverDir = "D:\project\chaos\chaos-lib\chaos-go"
$outputPath = "D:\data\chaos\chaos-go.exe"
$uiOutputDir = Join-Path (Split-Path $outputPath -Parent) "ui"

cd $uiDir
pnpm install --frozen-lockfile
pnpm run build

if (Test-Path $uiOutputDir) {
    Remove-Item -Path $uiOutputDir -Recurse -Force
}
New-Item -ItemType Directory -Path $uiOutputDir -Force | Out-Null
Copy-Item -Path "$uiDir\dist\*" -Destination $uiOutputDir -Recurse -Force

cd $serverDir
go build -o $outputPath .\cmd\server\main.go

nssm restart chaos

Write-Host "✅ 部署完成: $outputPath + $uiOutputDir"