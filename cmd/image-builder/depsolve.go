package main

import (
	"io"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/osbuild/image-builder-cli/pkg/progress"
	"github.com/osbuild/images/pkg/dnfjson"
)

type depsolvedYAML struct {
	Packages map[string]pkgsYAML `yaml:"packages"`
}

type pkgYAML struct {
	Name    string
	Version string
	Release string
	Epoch   uint
}

type pkgsYAML struct {
	Packages []pkgYAML
}

func cmdDepsolve(cmd *cobra.Command, args []string) error {
	pbar, err := progress.New("")
	if err != nil {
		return err
	}

	manifestOut := io.Discard
	depsolveWarnings := osStderr
	var allDepsolved map[string]dnfjson.DepsolveResult
	opts := &cmdManifestWrapperOptions{
		depsolveResultCb: func(res map[string]dnfjson.DepsolveResult) error {
			allDepsolved = res
			return nil
		},
	}

	// XXX: add support for args[1:] as extra packages to depsolve
	if _, err = cmdManifestWrapper(pbar, cmd, args, manifestOut, depsolveWarnings, opts); err != nil {
		return err
	}
	// XXX: add multiple formater, allow reuse from "manifest --with-depsolve"
	var out depsolvedYAML
	out.Packages = make(map[string]pkgsYAML)
	for pipelineName, depsolved := range allDepsolved {
		// XXX: add -v or something
		if pipelineName == "build" {
			continue
		}

		var pkgs []pkgYAML
		for _, pkg := range depsolved.Packages {
			pkgs = append(pkgs, pkgYAML{
				Name:    pkg.Name,
				Version: pkg.Version,
				Release: pkg.Release,
				Epoch:   pkg.Epoch,
			})
		}
		out.Packages[pipelineName] = pkgsYAML{
			Packages: pkgs,
		}
	}

	return yaml.NewEncoder(osStdout).Encode(out)
}
