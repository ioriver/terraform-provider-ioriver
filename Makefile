TEST?=$$(go list ./...)
HOSTNAME=ioriver.io
NAMESPACE=ioriver
NAME=ioriver
SWEEP=ioriver
BINARY=terraform-provider-${NAME}
VERSION=0.0.1
OS_ARCH=darwin_arm64

default: install

build:
	go build -o ${BINARY}

# release:
# 	goreleaser release --rm-dist --snapshot --skip-publish  --skip-sign

install: build
	mkdir -p ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}
	mv ${BINARY} ~/.terraform.d/plugins/${HOSTNAME}/${NAMESPACE}/${NAME}/${VERSION}/${OS_ARCH}

test: 
	go test -i $(TEST) || exit 1                                                   
	echo $(TEST) | xargs -t -n4 go test $(TESTARGS) -timeout=30s -parallel=4                    

testacc: 
	TF_ACC=1 go test $(TEST) -v $(TESTARGS) -timeout 120m   

update:
	export GOPROXY="github.com/ioriver/ioriver-go@v0.16.0,https://proxy.golang.org,direct"
	go get -u

sweep:
	@echo "WARNING: This will destroy infrastructure. Use only in development accounts."
	go test $(TEST) -v -sweep=$(SWEEP) $(SWEEPARGS)