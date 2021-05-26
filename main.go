package main

import "flag"

var (
	readFile = flag.String("file", "", "File to be used with ")
)

func init() {
	flag.Parse()
}
