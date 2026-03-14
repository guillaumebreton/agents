BINARY := agents
MODULE := notb.re/agent

.PHONY: build run clean install

build:
	go build -o $(BINARY) .

run:
	go run .

clean:
	rm -f $(BINARY)

install:
	go install .
