package main

import (
	"context"
	"log"
	"os"
)

func Run(ctx context.Context, args []string) error {
	println("Hello otf2vtfont")
	return nil
}

func main() {
	err := Run(context.Background(), os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
}
