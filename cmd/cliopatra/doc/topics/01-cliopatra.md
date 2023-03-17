---
Title: Cliopatra - A tool to run CLI applications
Slug: cliopatra
Short: Cliopatra is a tool to run, test, render and document CLI applications.
IsTemplate: false
IsTopLevel: true
ShowPerDefault: true
SectionType: GeneralTopic
---

## Overview

Cliopatra helps to run, test and manage the output of CLI applications.

The GO GO GOLEMS organization builds a lot of command-line tools, 
and provides a lot of different flags to customize the behaviour of these 
applications. 

There is a lot that can be done with command-line applications, and cliopatra 
helps with the following areas:

- testing to make sure that flags work as they used before
- automatically render the output of CLI flags as part of documentation
- capture and create tests for CLI applications
- provide aliases for existing command-line applications

## Testing

GO GO GOLEMS puts some effort into maintaining unit tests for the many layers
of functionality offered, but nothing comes close to doing integration testing
at the final level of the application. With CLI applications, that is very easy to
do:

- provide flags and arguments
- provide environment variables
- provide input files and data
- run the application and record its output (stdout, stderr, potentially files)
- compare the output with the expected output


## Rendering

Cliopatra can load text files and render embedded data by calling an external 
CLI programs and inserting its output.

This is useful for example to render the output of a CLI application as part of
a documentation page or a website.