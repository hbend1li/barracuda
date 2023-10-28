all: reaction ip46tables

clean:
	rm -f reaction ip46tables
ip46tables: ip46tables.d/ip46tables.c
	gcc ip46tables.d/ip46tables.c -o ip46tables

reaction: app/* reaction.go go.mod go.sum
	go build -buildvcs=false .
