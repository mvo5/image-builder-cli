package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/imagefilter"
	"github.com/osbuild/images/pkg/osbuild"
)

const (
	bundleMeta = "osbuild-bundle.json"
	bundleExt  = ".osbuild-bundle"
)

// XXX: waaaaay to shallow
type bundleMetadata struct {
	Exports []string `json:"exports"`
}

func makeBundle(outputDir string, img *imagefilter.Result, osbuildManifest []byte) error {
	if outputDir == "" {
		outputDir = outputNameFor(img)
	}

	// XXX: this is sad, we cannot use an exiting cacheDir right
	// now, need to create a fresh "cache-dir" so that we get
	// *exactly* the files in the bundle we need. This needs
	// thinking how to re-use the existing store/cache but for now
	// its good enough
	bundleDir, err := os.MkdirTemp("", fmt.Sprintf("bundle-for-%s", outputNameFor(img)))
	if err != nil {
		return err
	}
	defer os.RemoveAll(bundleDir)
	bundleStoreDir := filepath.Join(bundleDir, "store")

	// XXX: this will produce a lot of output currently
	if _, err := osbuild.RunOSBuild(osbuildManifest, bundleStoreDir, "", nil, nil, nil, false, osStderr); err != nil {
		return err
	}

	// write bundle metadata
	// XXX: could we avoid this by using https://github.com/osbuild/osbuild/pull/1930 ?
	f, err := os.Create(filepath.Join(bundleDir, bundleMeta))
	if err != nil {
		return err
	}
	defer f.Close()
	meta := bundleMetadata{Exports: img.ImgType.Exports()}
	if err := json.NewEncoder(f).Encode(meta); err != nil {
		return err
	}

	// write manifest
	if err := os.WriteFile(filepath.Join(bundleDir, "manifest.json"), osbuildManifest, 0644); err != nil {
		return err
	}
	// tar it all up
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	outputFile := filepath.Join(outputDir, outputNameFor(img)+bundleExt)
	tarCmd := exec.Command(
		"tar",
		"-C", bundleDir,
		"-c",
		"-z",
		"-f", outputFile,
		".",
	)
	// XXX: proper error catture/reporting
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return err
	}

	return nil
}

func buildBundle(outputDir, bundlePath string) error {
	if outputDir == "" {
		outputDir = strings.TrimSuffix(filepath.Base(bundlePath), filepath.Ext(bundlePath))
	}

	bundleDir, err := os.MkdirTemp("", fmt.Sprintf("bundle-for-%s", filepath.Base(bundlePath)))
	if err != nil {
		return err
	}
	defer os.RemoveAll(bundleDir)
	// XXX: duplicated from above
	bundleStoreDir := filepath.Join(bundleDir, "store")

	// untar it
	tarCmd := exec.Command(
		"tar",
		"-C", bundleDir,
		"-x",
		"-z",
		"-f", bundlePath,
	)
	// XXX: proper error catture/reporting
	tarCmd.Stdout = os.Stdout
	tarCmd.Stderr = os.Stderr
	if err := tarCmd.Run(); err != nil {
		return err
	}

	// get exports
	f, err := os.Open(filepath.Join(bundleDir, bundleMeta))
	if err != nil {
		return err
	}
	defer f.Close()
	var meta bundleMetadata
	if err := json.NewDecoder(f).Decode(&meta); err != nil {
		return err
	}

	// read manifest
	osbuildManifest, err := os.ReadFile(filepath.Join(bundleDir, "manifest.json"))
	if err != nil {
		return err
	}

	// build it
	if _, err := osbuild.RunOSBuild(osbuildManifest, bundleStoreDir, outputDir, meta.Exports, nil, nil, false, osStderr); err != nil {
		return err
	}

	return nil
}
