# Minimal Makefile calling Go tool

.PHONY: all install clean

all:
	go build

install:
	go install

clean:
	go clean
