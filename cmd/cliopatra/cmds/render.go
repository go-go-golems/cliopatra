package cmds

import (
	"context"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/fsnotify/fsnotify"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"os"
	"path/filepath"
	"strings"
)

type Watcher struct {
	Paths    []string
	Mask     string
	Callback func(path string)
}

func (w *Watcher) Run(ctx context.Context) error {
	// Create a new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()

	// Add each path to the watcher
	for _, path := range w.Paths {
		log.Debug().Str("path", path).Msg("Adding recursive path to watcher")
		err = addRecursive(watcher, path)
		if err != nil {
			return err
		}
	}

	log.Info().Strs("paths", w.Paths).Str("mask", w.Mask).Msg("Watching paths")

	// Listen for events until the context is cancelled
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}
			log.Debug().Str("event", event.String()).Msg("Received fsnotify event")

			// if it is a deletion, remove the directory from the watcher
			if event.Op&fsnotify.Remove == fsnotify.Remove {
				log.Debug().Str("path", event.Name).Msg("Removing directory from watcher")
				err = removePathsWithPrefix(watcher, event.Name)
				if err != nil {
					return err
				}
				continue
			}

			// if a new directory is created, add it to the watcher
			if event.Op&fsnotify.Create == fsnotify.Create {
				info, err := os.Stat(event.Name)
				if err != nil {
					return err
				}
				if info.IsDir() {
					log.Debug().Str("path", event.Name).Msg("Adding new directory to watcher")
					err = addRecursive(watcher, event.Name)
					if err != nil {
						return err
					}
					continue
				}
			}

			doesMatch, err := doublestar.Match(w.Mask, event.Name)
			if err != nil {
				return err
			}

			if !doesMatch {
				log.Debug().Str("path", event.Name).Str("mask", w.Mask).Msg("Skipping event because it does not match the mask")
				continue
			}
			if event.Op&fsnotify.Write != fsnotify.Write && event.Op&fsnotify.Create != fsnotify.Create {
				log.Debug().Str("path", event.Name).Msg("Skipping event because it is not a write or create event")
				continue
			}
			log.Info().Str("path", event.Name).Msg("File modified")
			if w.Callback != nil {
				w.Callback(event.Name)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			log.Error().Err(err).Msg("Received fsnotify error")
		}
	}
}

// removePathsWithPrefix removes `name` and all subdirectories from the watcher
func removePathsWithPrefix(watcher *fsnotify.Watcher, name string) error {
	// we do the "recursion" by checking the watchlist of the watcher for all watched directories
	// that has name as prefix
	watchlist := watcher.WatchList()
	for _, path := range watchlist {
		if strings.HasPrefix(path, name) {
			log.Debug().Str("path", path).Msg("Removing path from watcher")
			err := watcher.Remove(path)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// Recursively add a path to the watcher
func addRecursive(watcher *fsnotify.Watcher, path string) error {
	if !strings.HasSuffix(path, string(os.PathSeparator)) {
		path += string(os.PathSeparator)
	}

	addPath := strings.TrimSuffix(path, string(os.PathSeparator))

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	// check if we have permissions to watch
	if info.Mode()&os.ModeSymlink != 0 {
		log.Debug().Str("path", addPath).Msg("Skipping symlink")
		return nil
	}

	// open and then close to check if we can actually read from the file
	f, err := os.Open(addPath)
	if err != nil {
		log.Warn().Str("path", addPath).Msg("Skipping path because we cannot read it")
		return nil
	}
	_ = f.Close()

	log.Debug().Str("path", addPath).Msg("Adding path to watcher")
	err = watcher.Add(addPath)
	if err != nil {
		return err
	}

	if info.IsDir() {
		log.Debug().Str("path", path).Msg("Walking path to add subpaths to watcher")
		err = filepath.Walk(path, func(subpath string, info os.FileInfo, err error) error {
			if err != nil {
				log.Warn().Err(err).Str("path", subpath).Msg("Error walking path")
				return nil
			}
			if subpath == path {
				return nil
			}
			log.Trace().Str("path", subpath).Msg("Testing subpath to watcher")
			if info.IsDir() {
				log.Debug().Str("path", subpath).Msg("Adding subpath to watcher")
				err = addRecursive(watcher, subpath)
				if err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Match a path against a glob mask
func match(mask, path string) bool {
	if mask == "" {
		return true
	}
	match, err := filepath.Match(mask, path)
	if err != nil {
		return false
	}
	return match
}

func runWatcher(args []string) {
	w := &Watcher{
		Paths: args,
		Mask:  "**/*.txt",
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eg := errgroup.Group{}

	// Add the watcher to the errgroup
	eg.Go(func() error {
		return w.Run(ctx)
	})

	// Wait for the watcher to finish or the context to be cancelled
	// Wait for the errgroup to complete
	err := eg.Wait()
	// check that the error wasn't a cancel
	if err != nil && err != context.Canceled {
		log.Error().Err(err).Msg("Error running watcher")
	}
	cobra.CheckErr(err)

}

func NewRenderCommand() *cobra.Command {
	renderCommand := &cobra.Command{
		Use:   "render",
		Short: "Render a go template file by expanding cliopatra calls",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			runWatcher(args)
		},
	}

	return renderCommand
}
