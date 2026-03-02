//go:build !dev

package main

import (
	"embed"
	"io/fs"
	"log"

	"github.com/fllint/fllint/cmd"
)

//go:embed all:frontend/build
var frontendFiles embed.FS

func main() {
	frontendFS, err := fs.Sub(frontendFiles, "frontend/build")
	if err != nil {
		log.Fatalf("Failed to access embedded frontend: %v", err)
	}
	cmd.Run(frontendFS)
}
