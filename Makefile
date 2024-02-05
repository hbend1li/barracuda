CC ?= gcc
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
SYSTEMDDIR ?= /etc/systemd

all: reaction ip46tables nft46

clean:
	rm -f reaction ip46tables reaction.deb deb reaction.minisig ip46tables.minisig reaction.deb.minisig

ip46tables: helpers_c/ip46tables.c
	$(CC) -s -static helpers_c/ip46tables.c -o ip46tables

nft46: helpers_c/nft46.c
	$(CC) -s -static helpers_c/nft46.c -o nft46

reaction: app/* reaction.go go.mod go.sum
	CGO_ENABLED=0 go build -buildvcs=false -ldflags "-s -X main.version=`git tag --sort=v:refname | tail -n1` -X main.commit=`git rev-parse --short HEAD`"

reaction.deb: reaction ip46tables nft46
	chmod +x reaction ip46tables nft46
	mkdir -p deb/reaction/usr/bin/ deb/reaction/usr/sbin/ deb/reaction/lib/systemd/system/
	cp reaction ip46tables nft46 deb/reaction/usr/bin/
	cp config/reaction.debian.service deb/reaction/lib/systemd/system/reaction.service
	cp -r DEBIAN/ deb/reaction/DEBIAN
	sed -e "s/LAST_TAG/`git tag --sort=v:refname | tail -n1`/" -e "s/Version: v/Version: /" -i deb/reaction/DEBIAN/*
	cd deb && dpkg-deb --root-owner-group --build reaction
	mv deb/reaction.deb reaction.deb
	rm -rf deb/

signatures: reaction.deb reaction ip46tables nft46
	minisign -Sm ip46tables nft46 reaction reaction.deb

install: all
	install -m755 reaction $(DESTDIR)$(BINDIR)
	install -m755 ip46tables $(DESTDIR)$(BINDIR)
	install -m755 nft46 $(DESTDIR)$(BINDIR)

install_systemd: install
	install -m644 config/reaction.debian.service $(SYSTEMDDIR)/system/reaction.service
	sed -i 's#/usr/bin#$(DESTDIR)$(BINDIR)#' $(SYSTEMDDIR)/system/reaction.service
