.PHONY: smp2p ans off

CC					= gcc
OS					:= $(shell uname -s)
ARCH				?= amd64
CFLAGS				= GOOS=linux
GOFLAGS				= -mod=vendor -ldflags "-s -w"
SRC					= ./cmd/smp2p
DOC					= $(shell pwd)/sdk/python
NAME				= tpfunc
BUILD  				= build/$(ARCH)
DRONE_BUILD_NUMBER	?= unknown
BUILD_NUMBER		?= $(DRONE_BUILD_NUMBER)
VERSION				:= 1.0.0-$(BUILD_NUMBER)

ifeq ($(ARCH), armhf)
CC	:=arm-linux-gnueabihf-gcc
CFLAGS  += GOARCH=arm GOARM=7 CGO_ENABLED=1
endif


dev:
	go build $(GOFLAGS) -o smp2p $(SRC)

smp2p: clean
	mkdir -p $(BUILD)
	$(CFLAGS) CC=$(CC) go build $(GOFLAGS) -o $(BUILD)/$@ $(SRC)

clean:
	rm -rf $(BUILD)