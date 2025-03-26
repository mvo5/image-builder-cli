package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

var bibUpstreamRef = "quay.io/centos-bootc/bootc-image-builder:latest"

func podmanBibCmd() []string {
	return []string{
		"podman", "run",
		"--rm", "-it",
		"--privileged",
		"--security-opt", "label=type:unconfined_t",
		"--pull=newer",
		"-v", "/var/lib/containers/storage:/var/lib/containers/storage",
	}
}

func cmdNeedsBootc(cmd *cobra.Command) bool {
	bootcRef, err := cmd.Flags().GetString("bootc-ref")
	return bootcRef != "" && err == nil
}

func cmdManifestBootc(cmd *cobra.Command, args []string) error {
	return cmdBootc("manifest", cmd, args)
}

func cmdBuildBootc(cmd *cobra.Command, args []string) error {
	return cmdBootc("build", cmd, args)
}

func cmdBootc(action string, cmd *cobra.Command, args []string) error {
	// XXX2: check for uid=0 here *or* map user container storage
	// when running

	// XXX: validate *all* options and error if options not compatible
	// with bootc are passed:
	// Translate:
	// --target-arch
	// --verbose
	// --progress
	// --rootfs
	//
	// auto-add:
	// --rpmmd /var/cache/image-builder (and double check)
	blueprintPath, err := cmd.Flags().GetString("blueprint")
	if err != nil {
		return err
	}
	outputDir, err := cmd.Flags().GetString("output-dir")
	if err != nil {
		return err
	}
	bootcRef, err := cmd.Flags().GetString("bootc-ref")
	if err != nil {
		return err
	}
	imgTypeStr := args[0]

	bibCmdline := podmanBibCmd()
	if blueprintPath != "" {
		bibCmdline = append(bibCmdline, "-v")
		blueprintExt := filepath.Ext(blueprintPath)
		bibCmdline = append(bibCmdline, fmt.Sprintf("%s:/config.%s:ro", blueprintPath, blueprintExt))
	}
	// XXX: make output dir follow our predictable names
	if outputDir != "" {
		bibCmdline = append(bibCmdline, "-v")
		bibCmdline = append(bibCmdline, fmt.Sprintf("%s:/output", outputDir))
	}
	bibCmdline = append(bibCmdline, bibUpstreamRef)
	bibCmdline = append(bibCmdline, action)
	bibCmdline = append(bibCmdline, bootcRef)
	bibCmdline = append(bibCmdline, fmt.Sprintf("--type=%v", imgTypeStr))

	bibCmd := exec.Command(bibCmdline[0], bibCmdline[1:]...)
	bibCmd.Stdin = os.Stdin
	bibCmd.Stdout = os.Stdout
	bibCmd.Stderr = os.Stderr

	if err := bibCmd.Run(); err != nil {
		return fmt.Errorf("cannot run %q: %w", bibCmdline, err)
	}
	return nil
}
