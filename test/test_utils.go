package test

import (
	"go/types"
	"golang.org/x/tools/go/packages"
	"strings"
)

type PackageInspector struct {
	PackageName string
	Fields map[string]*types.TypeName
}
func (p *PackageInspector) HasField(name string) bool {
	_,ok := p.Fields[name]
	return ok
}
func (p *PackageInspector) HasFieldWithPrefix(prefix string) bool {
	for s := range p.Fields {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

func GetPackageStructs(name string) PackageInspector {
	config := &packages.Config{
		Mode:  packages.NeedTypes,
	}
	pkgs, err := packages.Load(config, name)
	if err != nil {
		panic(err)
	}

	res := make(map[string]*types.TypeName)
	scope := pkgs[0].Types.Scope()
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		if tn, ok := obj.(*types.TypeName); ok {
			res[name] = tn
		}
	}
	return PackageInspector{
		PackageName: name,
		Fields: res,
	}
}
