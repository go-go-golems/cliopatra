package render

import (
	"context"
	"fmt"
	"github.com/go-go-golems/cliopatra/pkg"
	"github.com/go-go-golems/glazed/pkg/cmds/layers"
	"github.com/go-go-golems/glazed/pkg/helpers"
	"io"
	"strings"
	"text/template"
)

type Renderer struct {
	programs             map[string]*pkg.Program
	withGoTemplate       bool
	withYamlMarkers      bool
	delimiters           []string
	allowProgramCreation bool
}

type Option func(r *Renderer)

func WithPrograms(programs map[string]*pkg.Program) Option {
	return func(r *Renderer) {
		r.programs = programs
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

func NewRenderer(options ...Option) *Renderer {
	r := &Renderer{}
	for _, option := range options {
		option(r)
	}
	return r
}

// template functions to quickly address cliopatra programs

type cliopatraTemplateOption func(p *pkg.Program) error

func (r *Renderer) clioLookupProgram(name string) (*pkg.Program, error) {
	program, ok := r.programs[name]
	if !ok {
		return nil, fmt.Errorf("program %s not found", name)
	}
	return program, nil
}

func (r *Renderer) CreateTemplate(name string) (*template.Template, error) {
	// create template
	t := helpers.CreateTemplate(name).
		Funcs(template.FuncMap{
			"lookup": func(name string) (*pkg.Program, error) {
				return r.clioLookupProgram(name)
			},
			"program": func(name string, options ...cliopatraTemplateOption) (*pkg.Program, error) {
				if r.allowProgramCreation {
					return &pkg.Program{
						Name: name,
					}, nil
				} else {
					return nil, fmt.Errorf("program creation is not allowed")
				}
			},
			"path": func(s string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					p.Path = s
					return nil
				}
			},
			"verbs": func(s ...string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					p.Verbs = s
					return nil
				}
			},
			"stdin": func(s string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					p.Stdin = s
					return nil
				}
			},
			"add_raw_flag": func(s ...string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					p.AddRawFlag(s...)
					return nil
				}
			},
			"raw_flags": func(s ...string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					p.RawFlags = s
					return nil
				}
			},
			"flag": func(name string, value interface{}) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					return p.SetFlagValue(name, value)
				}
			},
			"flag_raw": func(name string, raw string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					return p.SetFlagRaw(name, raw)
				}
			},
			"arg": func(name string, value interface{}) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					return p.SetArgValue(name, value)
				}
			},
			"arg_raw": func(name string, raw string) cliopatraTemplateOption {
				return func(p *pkg.Program) error {
					return p.SetArgRaw(name, raw)
				}
			},
			"run": func(p interface{}, options ...interface{}) (string, error) {
				var p_ *pkg.Program
				var err error

				switch p := p.(type) {
				case *pkg.Program:
					p_ = p
				case string:
					p_, err = r.clioLookupProgram(p)
					if err != nil {
						return "", err
					}
				default:
					return "", fmt.Errorf("invalid program type: %T", p)
				}

				options_ := []cliopatraTemplateOption{}

				for _, option := range options {
					switch option := option.(type) {
					case cliopatraTemplateOption:
						options_ = append(options_, option)

					case string:
						// NOTE(manuel, 2023-03-18) What we really want here is to actually do proper flag parsing
						options_ = append(options_, func(p *pkg.Program) error {
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

				ps := map[string]interface{}{}
				parsedLayers := map[string]*layers.ParsedParameterLayer{}
				buf := strings.Builder{}

				ctx := context.Background()
				err = p_.RunIntoWriter(ctx, parsedLayers, ps, &buf)
				if err != nil {
					return "", err
				}

				return buf.String(), nil
			},
		})

	if r.delimiters != nil {
		if len(r.delimiters) != 2 {
			return nil, fmt.Errorf("invalid delimiters: %v", r.delimiters)
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
