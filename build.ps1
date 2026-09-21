# effort 빌드 스크립트 (Windows PowerShell 5.1)
#
#   .\build.ps1            bin\effort.exe 를 만든다
#   .\build.ps1 -Test      만들기 전에 go vet · go test 까지 돌린다
#
# 서브모듈을 당긴 뒤에는 이걸 한 번 돌린다. 옛 exe 는 시각을 UTC 로 찍고 옛 버그를 그대로 갖는다.

[CmdletBinding()]
param(
    [switch]$Test
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path

# go 는 **지금 폴더**의 모듈을 본다. 다른 Go 모듈 안에서 이 스크립트를 부르면
# outside main module 로 죽으므로 제 폴더로 옮겨 간다.
Push-Location $root
try {
    $go = Get-Command go -ErrorAction SilentlyContinue
    if ($null -eq $go) {
        Write-Host "go 를 찾을 수 없다. https://go.dev/dl/ 에서 Go 를 깔고 새 터미널을 연다." -ForegroundColor Red
        exit 1
    }
    Write-Host (& go version)

    $env:CGO_ENABLED = '0'

    if ($Test) {
        & go vet ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
        & go test -count=1 ./...
        if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    }

    $exe = Join-Path $root 'bin\effort.exe'
    $stamp = (Get-Date).ToString('yyyy-MM-ddTHH:mm:ssK')

    # 코드를 마지막으로 바꾼 커밋을 박는다. git 이 없거나 .git 이 없으면 빈 값 → 'dev' 로 찍는다.
    # PS 5.1 은 $ErrorActionPreference='Stop' 에서 네이티브 stderr 리디렉션을 예외로 터뜨린다. try 로 감싼다.
    $commit = ''
    if (Get-Command git -ErrorAction SilentlyContinue) {
        try { $commit = (& git -C $root log -1 --format=%h -- cmd internal go.mod) } catch { $commit = '' }
        if ($LASTEXITCODE -ne 0) { $commit = '' }
        $global:LASTEXITCODE = 0
    }
    if ($null -eq $commit) { $commit = '' }
    $commit = ($commit | Out-String).Trim()

    # 미커밋 변경이 있으면 -dirty 를 붙인다. 구운 exe 가 어느 소스인지 애매하지 않게.
    if ($commit -ne '') {
        $dirty = ''
        try { $dirty = (& git -C $root status --porcelain -- cmd internal go.mod | Out-String) } catch { $dirty = '' }
        if ($LASTEXITCODE -ne 0) { $dirty = '' }
        $global:LASTEXITCODE = 0
        if ($dirty.Trim() -ne '') { $commit += '-dirty' }
    }

    $ldflags = "-s -w -X main.buildTime=$stamp -X main.buildCommit=$commit"
    Write-Host "빌드 : $exe"
    # -ldflags 와 값을 한 토큰으로 붙이면 PowerShell 5.1 이 변수를 안 푼다. 따로 넘긴다.
    & go build -trimpath '-ldflags' $ldflags -o $exe (Join-Path $root 'cmd\effort')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

    $size = [math]::Round((Get-Item $exe).Length / 1MB, 1)
    Write-Host "됐다. $exe ($size MB)" -ForegroundColor Green
    exit 0
}
finally { Pop-Location }
