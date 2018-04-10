package handlers

import (
	"github.com/autarch/metagodoc/api/models"
	"github.com/autarch/metagodoc/doc"
	"github.com/autarch/metagodoc/esmodels"

	"github.com/go-openapi/strfmt"
)

func refNames(refs []*esmodels.Ref) []string {
	var items []string
	for _, r := range refs {
		items = append(items, r.Name)
	}
	return items
}

func packages(pkgs []*esmodels.Package) []*models.Package {
	var items []*models.Package
	for _, p := range pkgs {
		items = append(items, onePackage(p))
	}
	return items
}

func onePackage(p *esmodels.Package) *models.Package {
	return &models.Package{
		Consts:       values(p.Consts),
		Doc:          p.Doc,
		Errors:       p.Errors,
		Examples:     examples(p.Examples),
		Files:        files(p.Files),
		Funcs:        funcs(p.Funcs),
		ImportPath:   p.ImportPath,
		Imports:      p.Imports,
		IsCommand:    p.IsCommand,
		Name:         p.Name,
		Synopsis:     p.Synopsis,
		TestImports:  p.TestImports,
		Types:        types(p.Types),
		Vars:         values(p.Vars),
		XTestImports: p.XTestImports,
	}
}

func values(values []*doc.Value) []*models.Value {
	var items []*models.Value
	for _, v := range values {
		items = append(items, &models.Value{
			Decl: code(v.Decl),
			Doc:  v.Doc,
			Pos:  pos(v.Pos),
		})
	}
	return items
}

func code(c doc.Code) *models.Code {
	return &models.Code{
		Annotations: annotations(c.Annotations),
		Paths:       c.Paths,
		Text:        c.Text,
	}
}

func annotations(annotations []doc.Annotation) []*models.Annotation {
	var items []*models.Annotation
	for _, a := range annotations {
		items = append(items, &models.Annotation{
			End:       a.End,
			Kind:      string(a.Kind),
			PathIndex: a.PathIndex,
			Pos:       a.Pos,
		})
	}
	return items
}

func pos(c doc.Pos) *models.Pos {
	return &models.Pos{
		File: c.File,
		Line: c.Line,
		N:    c.N,
	}
}

func examples(examples []*doc.Example) []*models.Example {
	var items []*models.Example
	for _, e := range examples {
		items = append(items, &models.Example{
			Code:   code(e.Code),
			Doc:    e.Doc,
			Name:   e.Name,
			Output: e.Output,
			Play:   e.Play,
		})
	}
	return items
}

func files(files []*doc.File) []*models.File {
	var items []*models.File
	for _, f := range files {
		items = append(items, &models.File{
			Name: f.Name,
			URL:  strfmt.URI(f.URL),
		})
	}
	return items
}

func funcs(funcs []*doc.Func) []*models.Func {
	var items []*models.Func
	for _, f := range funcs {
		items = append(items, &models.Func{
			Decl:     code(f.Decl),
			Doc:      f.Doc,
			Examples: examples(f.Examples),
			Name:     f.Name,
			Orig:     f.Orig,
			Pos:      pos(f.Pos),
			Recv:     f.Recv,
		})
	}
	return items
}

func types(types []*doc.Type) []*models.Type {
	var items []*models.Type
	for _, t := range types {
		items = append(items, &models.Type{
			Consts:   values(t.Consts),
			Decl:     code(t.Decl),
			Doc:      t.Doc,
			Examples: examples(t.Examples),
			Funcs:    funcs(t.Funcs),
			Methods:  funcs(t.Methods),
			Name:     t.Name,
			Pos:      pos(t.Pos),
			Vars:     values(t.Vars),
		})
	}
	return items
}
