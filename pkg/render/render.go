package render

import (
	"context"
	"fmt"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/go-go-golems/glazed/pkg/cli/cliopatra"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/helpers/templating"
	"github.com/pkg/errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

type Repository interface {
	GetPrograms() map[string]*cliopatra.Program
}

// Renderer renders recursive templates by exposing cliopatra specific template functions.
//
// NOTE(manuel, 2023-03-19) This could actually be a generic component that can be used for arbitrary recursive watching and rendering of templates
// See https://github.com/go-go-golems/glazed/issues/223
type Renderer struct {
	programs             map[string]*cliopatra.Program
	repositories         []Repository
	withGoTemplate       bool
	withYamlMarkers      bool
	delimiters           []string
	allowProgramCreation bool
	masks                []string
	verbose              bool
	renameOutputFiles    map[string]string
}

type Option func(r *Renderer)

func WithPrograms(programs map[string]*cliopatra.Program) Option {
	return func(r *Renderer) {
		r.programs = programs
	}
}

func WithRepositories(repositories ...Repository) Option {
	return func(r *Renderer) {
		r.repositories = repositories
	}
}

func WithVerbose(verbose bool) Option {
	return func(r *Renderer) {
		r.verbose = verbose
	}
}

func WithGoTemplate(withGoTemplate bool) Option {
	return func(r *Renderer) {
		r.withGoTemplate = withGoTemplate
	}
}

func WithYamlMarkers(withYamlMarkers bool) Option {
	return func(r *Renderer) {
		r.withYamlMarkers = withYamlMarkers
	}
}

func WithDelimiters(leftDelimiter string, rightDelimiter string) Option {
	return func(r *Renderer) {
		r.delimiters = []string{leftDelimiter, rightDelimiter}
	}
}

func WithAllowProgramCreation(allowProgramCreation bool) Option {
	return func(r *Renderer) {
		r.allowProgramCreation = allowProgramCreation
	}
}

func WithMasks(masks ...string) Option {
	return func(r *Renderer) {
		r.masks = masks
	}
}

func WithRenameOutputFiles(renameOutputFiles map[string]string) Option {
	return func(r *Renderer) {
		r.renameOutputFiles = renameOutputFiles
	}
}

func NewRenderer(options ...Option) *Renderer {
	r := &Renderer{
		masks:   []string{},
		verbose: false,
	}

	for _, option := range options {
		option(r)
	}
	return r
}

// template functions to quickly address cliopatra programs

type cliopatraTemplateOption func(p *cliopatra.Program) error

func (r *Renderer) clioLookupProgram(name string) (*cliopatra.Program, error) {
	// NOTE(manuel, 2023-03-27) Not sure about the precedence rules for looking up programs in the templates.
	// should we go through the fixed commands first? or through the repositories?
	// and should we go through repositories in reverse order?
	for _, repository := range r.repositories {
		program, ok := repository.GetPrograms()[name]
		if ok {
			return program, nil
		}
	}

	program, ok := r.programs[name]
	if !ok {
		return nil, errors.Errorf("program %s not found", name)
	}
	return program, nil
}

// CreateTemplate creates a standard glazed template (meaning, with all the sprig functions and co)
// and registers a set of custom functions to run and modify cliopatra programs.
//
// These functions are
//
//   - `lookup`: looks up a program by name and returns it
//
//   - `program`: creates a new program. This will fail if program creation is not allowed.
//
//   - `path`: sets the path of a program
//
//   - `verbs`: sets the verbs of a program (a []string)
//
//   - `env`: sets the env of a program (a map[string]string)
//
//   - `add_raw_flag`: adds a raw flag to a program (a string)
//
//   - `raw_flags`: sets the raw flags of a program (a []string)
//
//   - `flag`: sets the value of a flag (a interface{})
//
//   - `flag_raw`: sets the raw value of a flag (a string)
//
//   - `arg`: sets the value of an arg (a interface{})
//
//   - `arg_raw`: sets the raw value of an arg (a string)
//
//   - `run`: runs a program and returns the output. It can take an arbitrary number of options.
//
//     If the program to be run is a string, it will be looked up in the programs passed to the
//     renderer. If it is a *pkg.Program, it will be run as is.
//
//     If a string is passed as an option, it will be appended to the program as a raw flag.
//
//     `run` clones the program before modifying it with the passed options.
func (r *Renderer) CreateTemplate(name string) (*template.Template, error) {
	t := templating.CreateTemplate(name).
		Funcs(template.FuncMap{
			"lookup": func(name string) (*cliopatra.Program, error) {
				return r.clioLookupProgram(name)
			},
			"program": func(name string, options ...interface{}) (*cliopatra.Program, error) {
				if r.allowProgramCreation {
					p := &cliopatra.Program{
						Name: name,
					}

					options_ := []cliopatraTemplateOption{}

					for _, option := range options {
						switch option := option.(type) {
						case cliopatraTemplateOption:
							options_ = append(options_, option)

						case string:
							// NOTE(manuel, 2023-03-18) What we really want here is to actually do proper flag parsing
							options_ = append(options_, func(p *cliopatra.Program) error {
								p.AddRawFlag(option)
								return nil
							})
						}
					}

					for _, option := range options_ {
						err := option(p)
						if err != nil {
							return nil, err
						}
					}

					return p, nil
				} else {
					return nil, errors.Errorf("program creation is not allowed")
				}
			},
			"path": func(s string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.Path = s
					return nil
				}
			},
			"verbs": func(s ...string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.Verbs = s
					return nil
				}
			},
			"stdin": func(s string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.Stdin = s
					return nil
				}
			},
			"env": func(s map[string]string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.Env = s
					return nil
				}
			},
			"add_raw_flag": func(s ...string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.AddRawFlag(s...)
					return nil
				}
			},
			"raw_flags": func(s ...string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					p.RawFlags = s
					return nil
				}
			},
			"flag": func(name string, value interface{}) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					return p.SetFlagValue(name, value)
				}
			},
			"flag_raw": func(name string, raw string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					return p.SetFlagRaw(name, raw)
				}
			},
			"arg": func(name string, value interface{}) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					return p.SetArgValue(name, value)
				}
			},
			"arg_raw": func(name string, raw string) cliopatraTemplateOption {
				return func(p *cliopatra.Program) error {
					return p.SetArgRaw(name, raw)
				}
			},
			"run": func(p interface{}, options ...interface{}) (string, error) {
				var p_ *cliopatra.Program
				var err error

				switch p := p.(type) {
				case *cliopatra.Program:
					p_ = p
				case string:
					p_, err = r.clioLookupProgram(p)
					if err != nil {
						if r.allowProgramCreation {
							p_ = &cliopatra.Program{
								Name: p,
							}
						} else {
							return "", err
						}
					}
				default:
					return "", errors.Errorf("invalid program type: %T", p)
				}

				p_ = p_.Clone()

				options_ := []cliopatraTemplateOption{}

				for _, option := range options {
					switch option := option.(type) {
					case cliopatraTemplateOption:
						options_ = append(options_, option)

					case string:
						// NOTE(manuel, 2023-03-18) What we really want here is to actually do proper flag parsing
						options_ = append(options_, func(p *cliopatra.Program) error {
							p.AddRawFlag(option)
							return nil
						})
					}
				}

				for _, option := range options_ {
					err := option(p_)
					if err != nil {
						return "", err
					}
				}

				parsedLayers := layers.NewParsedLayers()
				buf := strings.Builder{}

				ctx := context.Background()
				err = p_.RunIntoWriter(ctx, parsedLayers, &buf)
				if err != nil {
					return "", err
				}

				return buf.String(), nil
			},
		})

	if r.delimiters != nil {
		if len(r.delimiters) != 2 {
			return nil, errors.Errorf("invalid delimiters: %v", r.delimiters)
		}
		t = t.Delims(r.delimiters[0], r.delimiters[1])
	}

	return t, nil
}

// NOTE(manuel, 2023-03-18) We should pass in the location of the template file
// This is so that we can provide a way to lookup program files relative to the
// template file. This could actually be done in the renderer itself with an option,
// come to think of it.

// Render renders the template from the given reader and writes the result to the given writer.
func (r *Renderer) Render(in io.Reader, out io.Writer) error {
	// create template from stream
	if r.withGoTemplate {
		// read string
		s, err := io.ReadAll(in)
		if err != nil {
			return err
		}

		t, err := r.CreateTemplate("template")
		if err != nil {
			return err
		}

		t, err = t.Parse(string(s))
		if err != nil {
			return err
		}

		// execute template
		err = t.Execute(out, nil)
		if err != nil {
			return err
		}
	}

	return nil
}

func (r *Renderer) checkMasks(file string) (bool, error) {
	if r.masks == nil || len(r.masks) == 0 {
		return true, nil
	}

	for _, mask := range r.masks {
		isMatch, err := doublestar.Match(mask, file)
		if err != nil {
			return false, err
		}
		if isMatch {
			return true, nil
		}
	}

	return false, nil
}

func (r *Renderer) RenderFile(file string, outputFile string) error {
	for k, v := range r.renameOutputFiles {
		if strings.HasSuffix(outputFile, k) {
			outputFile = strings.TrimSuffix(outputFile, k) + v
		}
	}

	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	var w io.Writer

	if file == "-" {
		w = os.Stdout
	} else {
		// create the output file
		out, err := os.Create(outputFile)
		if err != nil {
			return err
		}
		defer func(out *os.File) {
			_ = out.Close()
		}(out)
		w = out
	}

	if r.verbose {
		fmt.Printf("Rendering %s -> %s\n", file, outputFile)
	}

	err = r.Render(f, w)
	return err
}

func (r *Renderer) recursiveRenderDirectory(currentDirectory string, baseDirectory string, outputDirectory string) error {
	err := filepath.Walk(currentDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if path == currentDirectory {
			return nil
		}

		if info.IsDir() {
			return r.recursiveRenderDirectory(path, baseDirectory, outputDirectory)
		}

		ok, err := r.checkMasks(path)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		// create the output file
		relPath, err := filepath.Rel(baseDirectory, path)
		if err != nil {
			return err
		}

		outputFile := filepath.Join(outputDirectory, relPath)
		err = os.MkdirAll(filepath.Dir(outputFile), 0755)
		if err != nil {
			return err
		}

		return r.RenderFile(path, outputFile)
	})

	return err
}

func (r *Renderer) RenderDirectory(directory string, outputDirectory string) error {
	if !strings.HasSuffix(directory, "/") {
		directory += "/"
	}

	return r.recursiveRenderDirectory(directory, directory, outputDirectory)
}

// ComputeBaseDirectory computes the base directory for the given file.
// If baseDirectory is not empty, it is returned.
// Otherwise, the base directory is computed by finding the shortest common prefix
// of the given file and all files.
//
// Input directories have to end with a slash, as they will otherwise be considered
// files and stripped of their last component.
//
// The returned base directory won't have a / at the end.
func ComputeBaseDirectory(file string, allFiles []string, baseDirectory string) string {
	if baseDirectory != "" {
		return baseDirectory
	}

	// get the base path
	basePath := file
	for _, f := range allFiles {
		if strings.HasPrefix(file, f) {
			if len(f) < len(basePath) {
				basePath = f
			}
		}
	}

	return filepath.Dir(basePath)
}
