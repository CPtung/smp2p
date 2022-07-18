.PHONY: smp2p ans off

smp2p: clean
	go build -mod=vendor -ldflags '-s -w' -o ./bin/smp2p ./cmd/smp2p

off:
	./bin/smp2p offer

ans:
	./bin/smp2p answer

clean:
	rm -rf ./bin