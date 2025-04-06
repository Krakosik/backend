.PHONY: clean gen

setup:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest

clean:
	rm -r gen || exit 0

gen:
	protoc --go_out=. --go_opt=paths=source_relative --go-grpc_out=. --go-grpc_opt=paths=source_relative events.proto
	mkdir -p gen
	mv events.pb.go gen/events.pb.go
	mv events_grpc.pb.go gen/events_grpc.pb.go
