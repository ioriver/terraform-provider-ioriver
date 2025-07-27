export GOPROXY="https://proxy.golang.org,direct"
export GOPRIVATE="github.com/ioriver"
export GONOSUMDB="github.com/ioriver/*"
export GOBIN=$PWD/bin
export PATH=$GOBIN:$PATH

go mod init github.com/ioriver/terraform-provider-ioriver
go mod tidy
go mod vendor
golangci-lint run --fast || exit 1
go build ./...
#go test ./...
go generate ./...