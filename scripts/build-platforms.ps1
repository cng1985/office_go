# 从脚本所在目录定位仓库根目录，构建前端并交叉编译三个目标平台。
$ErrorActionPreference = 'Stop'
$Root = Split-Path -Parent $PSScriptRoot
Set-Location $Root

if (-not (Get-Command npm -ErrorAction SilentlyContinue)) {
    throw '未找到 npm'
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw '未找到 go'
}

Push-Location (Join-Path $Root 'web')
try {
    npm ci
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    npm test
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    npm run build
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
}
finally {
    Pop-Location
}

$version = '0.1.0'
$commit = 'unknown'
try {
    $commit = (& git rev-parse --short HEAD).Trim()
} catch {
    $commit = 'unknown'
}
$buildTime = (Get-Date).ToUniversalTime().ToString('yyyy-MM-ddTHH:mm:ssZ')
$ldflags = "-s -w -X yunjian/internal/platform/buildinfo.Version=$version -X yunjian/internal/platform/buildinfo.Commit=$commit -X yunjian/internal/platform/buildinfo.BuildTime=$buildTime"
$targets = @(
    @{ GOOS = 'windows'; GOARCH = 'amd64'; Name = 'yunjian.exe' },
    @{ GOOS = 'linux'; GOARCH = 'amd64'; Name = 'yunjian' },
    @{ GOOS = 'linux'; GOARCH = 'arm64'; Name = 'yunjian' }
)

$release = Join-Path $Root 'release'
if (Test-Path $release) {
    Remove-Item -Recurse -Force $release
}
New-Item -ItemType Directory -Force -Path $release | Out-Null

foreach ($target in $targets) {
    $dir = Join-Path $release "$($target.GOOS)-$($target.GOARCH)"
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    $env:GOOS = $target.GOOS
    $env:GOARCH = $target.GOARCH
    $env:CGO_ENABLED = '0'
    & go build -trimpath -ldflags $ldflags -o (Join-Path $dir $target.Name) ./cmd/server
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    Copy-Item -Recurse (Join-Path $Root 'configs') (Join-Path $dir 'configs')
    Copy-Item -Recurse (Join-Path $Root 'migrations') (Join-Path $dir 'migrations')
    Copy-Item (Join-Path $Root 'scripts/install-linux.sh') (Join-Path $dir 'install-linux.sh')
    Copy-Item (Join-Path $Root 'README.md') (Join-Path $dir 'README.md')
    $binary = Join-Path $dir $target.Name
    $hash = Get-FileHash -Algorithm SHA256 $binary
    Set-Content -Path (Join-Path $dir 'SHA256SUMS') -Value "$($hash.Hash.ToLower())  $($target.Name)" -Encoding ascii
    @"
yunjian $version
commit: $commit
built: $buildTime
os: $($target.GOOS)
arch: $($target.GOARCH)
"@ | Set-Content -Path (Join-Path $dir 'VERSION.txt') -Encoding utf8
}

Remove-Item Env:GOOS -ErrorAction SilentlyContinue
Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
Write-Host "release packages are in $release"
