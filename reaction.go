package main

import (
	"framagit.org/ppom/reaction/app"
)

func main() {
	app.Main(version, commit)
}

var (
	// Must be passed when building
	// go build -ldflags "-X app.commit XXX -X app.version XXX"
	version string
	commit  string
)
