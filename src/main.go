package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

var version = "dev"

type logger struct {
	verbose bool
	debug   bool
}

func (l *logger) infof(format string, args ...any) {
	if l.verbose || l.debug {
		log.Printf(format, args...)
	}
}

func (l *logger) debugf(format string, args ...any) {
	if l.debug {
		log.Printf("[debug] "+format, args...)
	}
}

func main() {
	var (
		daemon       = flag.Bool("daemon", false, "run continuously, processing new mail via IMAP IDLE")
		listFoldersF = flag.Bool("list-folders", false, "list IMAP folders and exit")
		versionF     = flag.Bool("version", false, "print version and exit")
		verbose      = flag.Bool("v", false, "verbose logging")
		debug        = flag.Bool("d", false, "debug logging")
		noop         = flag.Bool("n", false, "dry run — fetch but do not save or mark seen")
		configPath   = flag.String("config", defaultConfigPath(), "path to config file")
	)
	flag.Parse()

	if *versionF {
		fmt.Println("fetchbox", version)
		return
	}

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchbox: load config: %v\n", err)
		os.Exit(1)
	}

	if *listFoldersF {
		for _, mb := range cfg.Mailboxes {
			fmt.Printf("=== %s ===\n", mb.Name)
			if err := listFolders(mb); err != nil {
				log.Printf("list folders %s: %v", mb.Name, err)
			}
		}
		return
	}

	p := &processor{
		cfg:    cfg,
		noop:   *noop,
		logger: &logger{verbose: *verbose, debug: *debug},
	}

	if !*daemon {
		p.run()
		return
	}

	stop := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		log.Printf("shutting down...")
		close(stop)
	}()

	var wg sync.WaitGroup
	for _, mb := range cfg.Mailboxes {
		for _, folder := range mb.Folders {
			wg.Add(1)
			go func(mb Mailbox, f Folder) {
				defer wg.Done()
				p.watchFolder(mb, f, stop)
			}(mb, folder)
		}
	}
	wg.Wait()
}
