package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/hosting-panel/panel/internal/inventory"
)

func main() {
	class := flag.String("class", "all", "feed, install-only, or all")
	format := flag.String("format", "names", "names, package-copy, or feed-copy")
	flag.Parse()

	assets := inventory.RuntimeAssets()
	switch *class {
	case "feed":
		assets = inventory.FeedAssets()
	case "install-only":
		assets = inventory.InstallOnlyAssets()
	case "all":
	default:
		fmt.Fprintf(os.Stderr, "unknown class %q\n", *class)
		os.Exit(2)
	}

	for _, asset := range assets {
		switch *format {
		case "names":
			fmt.Println(asset.Name)
		case "package-copy":
			if asset.Package == "" {
				continue
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", asset.Source, asset.Package, modeString(asset.Mode), asset.Class)
		case "feed-copy":
			if asset.Class != inventory.ClassFeed || asset.FeedPath == "" {
				continue
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", asset.Source, asset.FeedPath, modeString(asset.Mode), asset.Kind)
		default:
			fmt.Fprintf(os.Stderr, "unknown format %q\n", *format)
			os.Exit(2)
		}
	}
}

func modeString(mode uint32) string {
	if mode == 0 {
		return "0644"
	}
	return fmt.Sprintf("%#o", mode)
}
