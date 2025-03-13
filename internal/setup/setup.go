package setup

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
)

var (
	// vars to make them mockable
	isMountpoint = func(path string) bool {
		return exec.Command("mountpoint", path).Run() == nil
	}
	mount = func(args ...string) error {
		fmt.Println("mount", args)
		if err := exec.Command("mount", args...).Run(); err != nil {
			return fmt.Errorf("mount %v failed: %w", args, err)
		}
		return nil
	}
	umount = func(mnt string) error {
		fmt.Println("umount", mnt)
		if err := exec.Command("umount", mnt).Run(); err != nil {
			return fmt.Errorf("umount of %q failed: %w", mnt, err)
		}
		return nil
	}
)

// EnsureRootmoun ensures that there is a valid "/" mount and if
// missing we create one, kudos to jlebon and
// https://gist.github.com/jlebon/fb6e7c6dcc3ce17d3e2a86f5938ec033
func EnsureRootmount() (cleanup func() error, err error) {
	var cleanupMnts []string
	cleanup = func() error {
		var errs []error
		for i := len(cleanupMnts) - 1; i >= 0; i-- {
			errs = append(errs, umount(cleanupMnts[i]))
		}
		return errors.Join(errs...)
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()

	if !isMountpoint("/") {
		if slices.Contains(os.Environ(), "IMAGE_BUILDER_CLI_REXEC") {
			panic("reexec but no /")
		}
		newRoot, err := os.MkdirTemp("", "ibcli-rootmnt")
		if err != nil {
			return nil, err
		}
		if err := mount("--bind", "/", newRoot); err != nil {
			return nil, err
		}
		cleanupMnts = append(cleanupMnts, newRoot)
		if err := mount("--make-rprivate", newRoot); err != nil {
			return nil, err
		}
		// note no cleanup for --make-rpviate required as its not
		// a mount
		if err := mount("--bind", newRoot, newRoot); err != nil {
			return nil, err
		}
		cleanupMnts = append(cleanupMnts, newRoot)
		for _, mnt := range []string{"/proc", "/sys"} {
			dst := filepath.Join(newRoot, mnt)
			if err := mount("--bind", mnt, dst); err != nil {
				return nil, err
			}
			cleanupMnts = append(cleanupMnts, dst)
		}

		// now reexec inside our chroot
		exe, err := os.Readlink("/proc/self/exe")
		if err != nil {
			return nil, err
		}
		env := os.Environ()
		env = append(env, "IMAGE_BUILDER_CLI_REEXEC=1")
		args := []string{newRoot, exe}
		args = append(args, os.Args[1:]...)
		cmd := exec.Command("chroot", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Env = env
		if err := cmd.Run(); err != nil {
			return nil, err
		}
		if err := cleanup(); err != nil {
			return nil, err
		}
		// we re-execed so all done
		os.Exit(1)
	}
	return cleanup, nil
}
