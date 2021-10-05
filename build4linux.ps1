$Env:GOOS="linux"
$Env:GOARCH="amd64"
go build -o .\build\

Remove-Item ".\build\public" -Recurse
Copy-Item -Path ".\public" -Destination ".\build\" -Recurse

Remove-Item "C:\Users\Y\Nextcloud\scytec-deploy\messenger" -Recurse
Copy-Item -Path ".\build\" -Destination "C:\Users\Y\Nextcloud\scytec-deploy\messenger\" -Recurse
