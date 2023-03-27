package pkg

import (
	"context"
	"fmt"
	"github.com/go-go-golems/clay/pkg/watcher"
	"github.com/go-go-golems/glazed/pkg/cli/cliopatra"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type repositoryProgram struct {
	fs_     fs.FS
	path    string
	program *cliopatra.Program
}

func LoadProgramsFromFS(f fs.FS, dir string) ([]*repositoryProgram, error) {
	programs := []*repositoryProgram{}

	entries, err := fs.ReadDir(f, dir)
	if err != nil {
		return nil, errors.Wrapf(err, "could not read dir %s", dir)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		fileName := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			programs_, err := LoadProgramsFromFS(f, fileName)
			if err != nil {
				return nil, errors.Wrapf(err, "could not load programs from dir %s", fileName)
			}
			programs = append(programs, programs_...)
			continue
		}

		if strings.HasSuffix(entry.Name(), ".yaml") ||
			strings.HasSuffix(entry.Name(), ".yml") {
			file, err := f.Open(fileName)
			if err != nil {
				return nil, errors.Wrapf(err, "could not open file %s", fileName)
			}

			defer func() {
				_ = file.Close()
			}()

			program, err := cliopatra.NewProgramFromYAML(file)
			if err != nil {
				return nil, errors.Wrapf(err, "could not load program from file %s", fileName)
			}

			programs = append(programs, &repositoryProgram{
				fs_:     f,
				path:    fileName,
				program: program,
			})
		}
	}

	return programs, nil
}

type Repository struct {
	repositoryPrograms map[string]*repositoryProgram
	pathsToProgramName map[string]string
	lock               sync.RWMutex
	directories        []string
}

func NewRepository(directories []string) *Repository {
	return &Repository{
		repositoryPrograms: map[string]*repositoryProgram{},
		directories:        directories,
		pathsToProgramName: map[string]string{},
	}
}

func (r *Repository) Load() error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, repository := range r.directories {
		_, err := os.Stat(repository)
		if err != nil {
			return errors.Wrapf(err, "could not stat repository %s", repository)
		}

		programs_, err := LoadProgramsFromFS(os.DirFS(repository), ".")
		if err != nil {
			return errors.Wrapf(err, "could not load programs from repository %s", repository)
		}

		for _, rp := range programs_ {
			name := rp.program.Name
			if _, ok := r.repositoryPrograms[name]; ok {
				return fmt.Errorf("program %s already exists", name)
			}
			rp.fs_ = nil
			rp.path = filepath.Join(repository, rp.path)
			r.repositoryPrograms[name] = rp
			r.pathsToProgramName[rp.path] = name
		}
	}

	return nil
}

func (r *Repository) GetPrograms() map[string]*cliopatra.Program {
	r.lock.RLock()
	defer r.lock.RUnlock()

	programs := map[string]*cliopatra.Program{}
	for name, rp := range r.repositoryPrograms {
		programs[name] = rp.program
	}
	return programs
}

func (r *Repository) Watch(
	ctx context.Context,
) error {
	watcherOptions := []watcher.Option{
		watcher.WithWriteCallback(func(path string) error {
			log.Debug().Str("path", path).Msg("watcher write event")

			f, err := os.Open(path)
			if err != nil {
				return errors.Wrapf(err, "could not open file %s", path)
			}

			defer func() {
				_ = f.Close()
			}()

			program, err := cliopatra.NewProgramFromYAML(f)
			if err != nil {
				log.Warn().Err(err).Str("path", path).Msg("could not load program from file")
				return nil
			}

			_, ok := r.pathsToProgramName[path]
			if ok {
				log.Info().Str("name", program.Name).Str("path", path).Msg("updating program")
			} else {
				log.Info().Str("name", program.Name).Str("path", path).Msg("adding program")
			}

			r.lock.Lock()
			defer r.lock.Unlock()
			r.repositoryPrograms[program.Name] = &repositoryProgram{
				fs_:     nil,
				path:    path,
				program: program,
			}
			r.pathsToProgramName[path] = program.Name

			return nil
		}),
		watcher.WithRemoveCallback(func(path string) error {
			log.Debug().Str("path", path).Msg("watcher remove event")
			name, ok := r.pathsToProgramName[path]
			if !ok {
				log.Warn().Str("path", path).Msg("could not find program name for path")
				return nil
			}

			log.Info().Str("name", name).Str("path", path).Msg("removing program")
			r.lock.Lock()
			defer r.lock.Unlock()
			delete(r.repositoryPrograms, name)
			delete(r.pathsToProgramName, path)

			return nil
		}),
		watcher.WithPaths(r.directories...),
		watcher.WithMask("**/*.yaml"),
	}

	watcher_ := watcher.NewWatcher(watcherOptions...)

	return watcher_.Run(ctx)
}
