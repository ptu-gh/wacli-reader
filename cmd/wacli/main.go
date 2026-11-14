package main

import "os"

func main() {
	configureOutputSignals()
	if err := execute(os.Args[1:]); err != nil {
		os.Exit(1)
	}
}
