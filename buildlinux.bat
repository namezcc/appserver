@echo off
SET CGO_ENABLED=0
SET GOOS=linux
SET GOARCH=amd64

echo now the CGO_ENABLED:
 go env CGO_ENABLED

echo now the GOOS:
 go env GOOS

echo now the GOARCH:
 go env GOARCH

cd goserver/bin
go build -gcflags=all="-N -l" ../server/servicefind/service_find.go
go build -gcflags=all="-N -l" ../server/monitor/monitor.go
go build -gcflags=all="-N -l" ../server/master/master.go
go build -gcflags=all="-N -l" ../server/clientService/clientService.go
go build -gcflags=all="-N -l" ../server/k8service/k8service.go

if "%1" == "" pause