$uiDir = "D:\project\chaos\chaos-lib\chaos-ui"
$webDir = "D:\project\chaos\chaos-lib\chaos-server\cmd\server\web"
$serverDir = "D:\project\chaos\chaos-lib\chaos-server"
$outputPath = "D:\data\chaos\chaos-server.exe"

cd $uiDir
pnpm install
pnpm run build

if (Test-Path $webDir) {
    Remove-Item -Path $webDir -Recurse -Force
}
New-Item -ItemType Directory -Path $webDir -Force | Out-Null
Copy-Item -Path "$uiDir\dist\*" -Destination $webDir -Recurse -Force

cd $serverDir
go build -o $outputPath .\cmd\server\main.go

nssm restart chaos

Write-Host "✅ 部署完成: $outputPath"