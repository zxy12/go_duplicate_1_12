
all:
	export GOROOT_BOOTSTRAP=/Users/zhouxinyu/www/localhost/src/github.com/golang/go1.4 && cd ./src && ./make.bash
run:
	./src/cmd/dist/dist

.PHONY:
	all test
