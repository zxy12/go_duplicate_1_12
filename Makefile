test:
	./src/cmd/dist/dist list
all:
	export GOROOT_BOOTSTRAP=/Users/zhouxinyu/www/localhost/src/github.com/golang/go1.4 && cd ./src && ./make.bash

clean:

.PHONY:
	all test
