// ---------------------------------------------------------------------------------------
//
//	main.go
//	-------
//
//	Entry point for the tls-switch binary.
//
//	(c) 2026 WaterJuice — Released under the Unlicense; see LICENSE.
//
// ---------------------------------------------------------------------------------------
package main

import "github.com/WaterJuice/tls-switch/internal"

// Version is set at build time via -ldflags
var Version = "dev"

func main() {
	internal.Run(Version)
}
