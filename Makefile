CC ?= gcc
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
SYSTEMDDIR ?= /etc/systemd

all: reaction ip46tables nft46

clean:
	rm -f reaction ip46tables nft46 reaction*.deb reaction.minisig ip46tables.minisig nft46.minisig reaction*.deb.minisig
	rm -rf debian-packaging

ip46tables: helpers_c/ip46tables.c
	$(CC) -s -static helpers_c/ip46tables.c -o ip46tables

nft46: helpers_c/nft46.c
	$(CC) -s -static helpers_c/nft46.c -o nft46

reaction: app/* reaction.go go.mod go.sum
	CGO_ENABLED=0 go build -buildvcs=false -ldflags "-s -X main.version=`git tag --sort=v:refname | tail -n1` -X main.commit=`git rev-parse --short HEAD`"

reaction_%-1_amd64.deb:
	apt-get -qq -y update
	apt-get -qq -y install build-essential devscripts debhelper quilt wget
	if [ -e debian-packaging ]; then rm -rf debian-packaging; fi
	mkdir debian-packaging
	wget "https://framagit.org/ppom/reaction/-/archive/v${*}/reaction-v${*}.tar.gz" -O "debian-packaging/reaction_${*}.orig.tar.gz"
	cd debian-packaging && tar xf "reaction_${*}.orig.tar.gz"
	cp -r debian "debian-packaging/reaction-v${*}"
	if [ -e "debian/changelog" ]; then \
		cd "debian-packaging/reaction-v${*}" && \
		DEBFULLNAME=ppom DEBEMAIL=reaction@ppom.me dch --package reaction --newversion "${*}-1" "New upstream release."; \
	else \
		cd "debian-packaging/reaction-v${*}" && \
		DEBFULLNAME=ppom DEBEMAIL=reaction@ppom.me dch --create --package reaction --newversion "${*}-1" "Initial release."; \
	fi
	cd "debian-packaging/reaction-v${*}" && DEBFULLNAME=ppom DEBEMAIL=reaction@ppom.me dch --release --distribution stable --urgency low ""
	cd "debian-packaging/reaction-v${*}" && debuild --prepend-path=/go/bin:/usr/local/go/bin -us -uc
	cp "debian-packaging/reaction-v${*}/debian/changelog" debian/
	cp "debian-packaging/reaction_${*}-1_amd64.deb" .

signatures_%: reaction_%-1_amd64.deb reaction ip46tables nft46
	minisign -Sm nft46 ip46tables reaction reaction_${*}-1_amd64.deb

install: all
	install -m755 reaction $(DESTDIR)$(BINDIR)
	install -m755 ip46tables $(DESTDIR)$(BINDIR)
	install -m755 nft46 $(DESTDIR)$(BINDIR)

install_systemd: install
	install -m644 config/reaction.example.service $(SYSTEMDDIR)/system/reaction.service
	sed -i 's#/usr/bin#$(DESTDIR)$(BINDIR)#' $(SYSTEMDDIR)/system/reaction.service
