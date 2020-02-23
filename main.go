package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strings"

	"github.com/fatih/color"
	"github.com/google/go-jsonnet"
	"github.com/google/go-jsonnet/ast"
)

type Function struct {
	params []string
}

func (fn Function) Signature(name string) string {
	return fmt.Sprintf("fn %s(%s)", name, strings.Join(fn.params, ", "))
}

func main() {
	log.SetFlags(0)
	file := "main.libsonnet"

	data, err := ioutil.ReadFile(file)
	if err != nil {
		log.Fatalln(err)
	}

	root, err := jsonnet.SnippetToAST(file, string(data))
	if err != nil {
		log.Fatalln(err)
	}

	m := funcs(root, Ctx{
		file: file,
		vm:   jsonnet.MakeVM(),
	})
	print(m.(map[string]interface{}), "")
}

var bold = color.New(color.Bold).SprintFunc()

func print(m map[string]interface{}, prefix string) {
	for k, v := range m {
		switch t := v.(type) {
		case Function:
			fmt.Println(t.Signature(prefix + bold(k)))
		case map[string]interface{}:
			print(t, prefix+k+".")
		}
	}
}

func funcs(node ast.Node, ctx Ctx) interface{} {
	switch n := node.(type) {
	case *ast.Local:
		return funcs(n.Body, ctx.withLocals(n.Binds))
	case *ast.DesugaredObject:
		o := make(map[string]interface{})
		for _, f := range n.Fields {
			name := f.Name.(*ast.LiteralString)
			o[name.Value] = funcs(f.Body, ctx.withLocals(n.Locals))
		}
		return o
	case *ast.Function:
		var fn Function
		for _, p := range n.Parameters.Required {
			fn.params = append(fn.params, string(p.Name))
		}
		for _, p := range n.Parameters.Optional {
			fn.params = append(fn.params, string(p.Name))
		}
		return fn
	case *ast.Import:
		imported, at, err := ctx.vm.ImportAST(ctx.file, n.File.Value)
		if err != nil {
			log.Fatalln(err)
		}
		return funcs(imported, ctx.withFile(at))
	case *ast.Var:
		v, ok := ctx.locals[n.Id]
		if !ok {
			log.Fatalln("unknown variable:", n.Id, "at", n.Loc())
		}
		return funcs(v, ctx)
	case *ast.Index:
		parentId := n.Target.(*ast.Var).Id
		p, ok := ctx.locals[parentId]
		if !ok {
			log.Fatalln("unknown variable:", n.Id, "at", n.Loc())
		}

		ref := field(p, n.Index.(*ast.LiteralString).Value)
		return funcs(ref, ctx)
	}
	return fmt.Sprintf("%T", node)
}

func field(o ast.Node, id string) ast.Node {
	switch t := o.(type) {
	case *ast.Local:
		return field(t.Body, id)
	case *ast.DesugaredObject:
		for _, f := range t.Fields {
			if f.Name.(*ast.LiteralString).Value == id {
				return f.Body
			}
		}
	}
	return nil
}

type Ctx struct {
	file   string
	vm     *jsonnet.VM
	locals map[ast.Identifier]ast.Node
}

func (c Ctx) withLocals(binds ast.LocalBinds) Ctx {
	locals := make(map[ast.Identifier]ast.Node)
	for _, b := range binds {
		locals[b.Variable] = b.Body
	}

	for k, v := range c.locals {
		if _, ok := locals[k]; ok {
			continue
		}
		locals[k] = v
	}

	return Ctx{
		vm:     c.vm,
		file:   c.file,
		locals: locals,
	}
}

func (c Ctx) withFile(f string) Ctx {
	c.file = f
	return c
}
