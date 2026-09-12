package main

import (
	"fmt"
	"os"

	"obxod/internal/divert"
	"obxod/internal/filter"
)

var voice = []filter.PortRange{{From: 19294, To: 19344}, {From: 50000, To: 50100}}

func main() {
	outbound, err := filter.Outbound(voice)
	if err != nil {
		fail(err)
	}

	fmt.Println(outbound)

	h, err := divert.Open(outbound, divert.Modify)
	if err != nil {
		fail(err)
	}
	defer h.Close()

	fmt.Println("driver opened, filter accepted")
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
