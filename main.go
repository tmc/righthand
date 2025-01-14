package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"runtime"
	"time"
)

var (
	// flagDumpWAVFile is a flag to dump the audio to a WAV file.
	flagDumpWAVFile = flag.Bool("dump-wav", false, "dump the audio to a WAV file")

	// DefaultTimeout is the default timeout for listening.
	DefaultTimeout = 30 * time.Second
)

// main is the entrypoint.
func main() {
	runtime.LockOSThread()
	flag.Parse()
	ctx := context.Background()

	// load config
	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error loading config: %v\n", err)
	}
	// process flags
	cfg.DumpWAVFile = *flagDumpWAVFile

	// create app
	app, err := newApp(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error initializing app:", err)
		os.Exit(1)
	}
	// run app
	if err := app.run(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error running app:", err)
		os.Exit(2)
	}
}
