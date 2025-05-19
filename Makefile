
LANG=en_US.UTF-8
SHELL=/bin/bash
.SHELLFLAGS=--norc --noprofile -e -u -o pipefail -c
# Include the main .env file
#
include config/dev.env

buf_push:
	cd proto && buf push
run:
	go run main.go
buf:
	rm -rf proto_gen/mcpclient/v1/*.pb.go && cd proto && buf lint && buf generate 

