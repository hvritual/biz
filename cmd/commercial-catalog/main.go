// Command commercial-catalog validates and derives the CE-03 commercial map.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hvritual/biz/internal/commercial/capabilitymap"
)

func run() error {
	root := flag.String("root", ".", "Biz repository root")
	write := flag.Bool("write", false, "write deterministic derived artifacts after validation")
	baseline := flag.String("baseline", "", "previous published catalog.json, obtained from trusted main")
	flag.Parse()
	if flag.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	document, err := capabilitymap.BuildRepository(*root)
	if err != nil {
		return err
	}
	if *baseline != "" {
		data, err := os.ReadFile(*baseline)
		if err != nil {
			return fmt.Errorf("CE03-BASELINE: %w", err)
		}
		previous, err := capabilitymap.DecodeDocument(data)
		if err != nil {
			return err
		}
		if err := capabilitymap.ValidateEvolution(previous, document); err != nil {
			return err
		}
	}
	if err := capabilitymap.SyncArtifacts(*root, document, *write); err != nil {
		return err
	}
	fmt.Printf("CE03_MAP_CHECK=PASS mapping_version=%s operations=%d capabilities=%d write=%t\n", document.MappingVersion, len(document.Operations), len(document.Capabilities), *write)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
