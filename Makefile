test:
	./src/cmd/dist/dist banner
all:
	export GOROOT_BOOTSTRAP=/Users/zhouxinyu/www/localhost/src/github.com/golang/go1.4 && cd ./src && ./make.bash

clean:

.PHONY:
	all test
