---
Title: Rendering reports with sqleton and evidence
Slug: reports
Short: Using cliopatra, we replace calls to sqleton with the rendered SQL query into a set of 
    evidence files. We then use evidence to render the report.  
Topics:
- sqleton
- evidence
- reports
- cliopatra
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: Application
---

# Overview 

[sqleton](https://github.com/go-go-golems/sqleton) is a CLI tool that leverages glazed
to run and render SQL queries. Templated SQL queries are provided as YAML files and 
exposed as command line applications.

[evidence](https://evidence.dev) is a static site generator that renders markdown files
into dynamic HTML. It has the ability to render embedded SQL queries and expose them
as arrays, barcharts and other graphs.

We use cliopatra to render sqleton calls into their underlying SQL queries, which allows
us to then pass the resulting markdown to evidence to be rendered into a report. Writing 
sqleton calls is easier and more maintainable than having SQL in the markdown, and it
allows a report write to reuse a lot of the queries already written as sqleton commands.

## Workflow

TODO(manuel, 2023-03-17) Actually write this up