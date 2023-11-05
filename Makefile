all: reaction ip46tables

clean:
	rm -f reaction ip46tables
ip46tables: ip46tables.d/ip46tables.c
	gcc -static ip46tables.d/ip46tables.c -o ip46tables

reaction: app/* reaction.go go.mod go.sum
	CGO_ENABLED=0 go build -buildvcs=false .
