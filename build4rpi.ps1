$Env:GOOS="linux"
$Env:GOARCH="arm"
$Env:GOARM="7"
go build -o .\build\

Remove-Item ".\build\public" -Recurse
Copy-Item -Path ".\public" -Destination ".\build\" -Recurse