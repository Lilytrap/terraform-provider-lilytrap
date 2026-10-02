// Command terraform-provider-lilytrap is the Terraform provider for Lilytrap decoys.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/lilytrap/terraform-provider-lilytrap/internal/provider"
)

// Set by goreleaser.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with support for debuggers like delve")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/lilytrap/lilytrap",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
