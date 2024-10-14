@go install github.com/tc-hib/go-winres@latest
@go-winres simply --icon aatracker.png
@go build -trimpath -buildmode=pie -ldflags="-s -w" -o aatracker.exe main.go

go install github.com/akavel/rsrc@latest
rsrc -ico aatracker.ico
go build -trimpath -buildmode=pie -ldflags="-s -w"