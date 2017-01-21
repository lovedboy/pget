export GOPATH=$(PWD)


MODULES := pget
BIN := pget

vendor:
	for m in $(MODULES) ; do \
	cd src/$$m && go get -insecure -v && cd -;\
	done
	go get github.com/stretchr/testify


test:
	echo ==================================; \
	for m in $(MODULES); do \
		cd $(PWD)/src/$$m && go test --race -cover; \
		echo ==================================; \
	done

fmt:
	find . -name "*.go" -type f -exec echo {} \; | grep -v -E "github.com|gopkg.in"|\
	while IFS= read -r line; \
	do \
		echo "$$line";\
		goimports -w "$$line" "$$line";\
	done

build:
	echo ==================================; \
	for m in $(BIN); do \
		cd $(PWD)/cmd && go build --race -o $$m $$m.go; \
	done
	echo ==================================; \



