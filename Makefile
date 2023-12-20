all: reaction ip46tables

clean:
	rm -f reaction ip46tables reaction.deb

ip46tables: ip46tables.d/ip46tables.c
	gcc -static ip46tables.d/ip46tables.c -o ip46tables

reaction: app/* reaction.go go.mod go.sum
	CGO_ENABLED=0 go build -buildvcs=false .

reaction.deb: reaction ip46tables
	chmod +x reaction ip46tables
	mkdir -p deb/reaction/usr/bin/ deb/reaction/usr/sbin/ deb/reaction/lib/systemd/system/
	cp reaction deb/reaction/usr/bin/
	cp ip46tables deb/reaction/usr/sbin/
	cp config/reaction.debian.service deb/reaction/lib/systemd/system/reaction.service
	cp -r DEBIAN/ deb/reaction/DEBIAN
	sed -e "s/LAST_TAG/`git tag --sort=v:refname | tail -n1`/" -e "s/Version: v/Version: /" -i deb/reaction/DEBIAN/*
	cd deb && dpkg-deb --root-owner-group --build reaction
	mv deb/reaction.deb reaction.deb
	rm -rf deb/
