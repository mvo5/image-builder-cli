package supermin

import (
	"archive/tar"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/osbuild/bootc-image-builder/bib/pkg/progress"
)

var superminInitScriptFmt = `#!/bin/sh
# inspired by https://github.com/coreos/coreos-assembler/blob/main/src/supermin-init-prelude.sh
# we need less because osbuild does most of its work via buildroots

set -e

export PATH=/usr/sbin:/usr/bin:/sbin:/bin

mount -t proc /proc /proc
mount -t sysfs /sys /sys
mount -t cgroup2 cgroup2 -o rw,nosuid,nodev,noexec,relatime,seclabel,nsdelegate,memory_recursiveprot /sys/fs/cgroup
mount -t devtmpfs devtmpfs /dev

# this is also normally set up by systemd in early boot
ln -s /proc/self/fd/0 /dev/stdin
ln -s /proc/self/fd/1 /dev/stdout
ln -s /proc/self/fd/2 /dev/stderr

# need /dev/shm for podman
mkdir -p /dev/shm
mount -t tmpfs tmpfs /dev/shm

# osbuild needs /run
mkdir -p /run
mount -t tmpfs tmpfs /run

# network
/usr/sbin/dhclient eth0


### diverging from the above script

# central for osbuild
mknod /dev/loop-control c 10 237

# prepare work dirs
mkdir /host-output
mount -t 9p osbuild_output /host-output
# output cannot be on 9p because permissions
mkdir -p /output
cp -r /host-output/* /output

mkdir /host-store
mount -t 9p osbuild_store /host-store
# XXX: we cannot put /store on a 9pfs or osbuild becomes very unhappy
echo "Populate /store from host"
mkdir /store
cp -R /host-store/* /store || true
# fetch sources
osbuild \
  --cache /store \
  /output/manifest.json
# copy stuff back to host store
cp -R /store/* /host-store

# XXX: trigger crash on error? how to transmit exit status?
echo "Running osbuild"
osbuild \
  --export %s \
  --output-directory /output \
  --cache /store \
  /host-output/manifest.json
echo "Copy result to host"
cp -r /output/* /host-output/

# trigger clean shutdown via sysreq
echo s > /proc/sysrq-trigger
echo u > /proc/sysrq-trigger
echo o > /proc/sysrq-trigger
sleep 999
`

func addInitTar(superminDir, superminInitScript string) error {
	initTarF, err := os.Create(filepath.Join(superminDir, "init.tgz"))
	if err != nil {
		return err
	}
	defer initTarF.Close()
	initTar := tar.NewWriter(initTarF)
	defer initTar.Close()
	if err := initTar.WriteHeader(&tar.Header{
		Name: "init",
		Size: int64(len(superminInitScript)),
		Mode: 0755,
	}); err != nil {
		return err
	}
	if _, err := initTar.Write([]byte(superminInitScript)); err != nil {
		return err
	}
	return nil
}

func RunOSBuild(pb progress.ProgressBar, manifest []byte, exports []string, opts *progress.OSBuildOptions) error {
	// XXX: check for /dev/kvm and error if not available

	// XXX: /var/tmp ?
	superminPrepareDir, err := os.MkdirTemp("", "supermin-prepare")
	if err != nil {
		return err
	}
	defer os.RemoveAll(superminPrepareDir)

	if len(exports) != 1 {
		return fmt.Errorf("only a single export supported right now")
	}
	superminInitScript := fmt.Sprintf(superminInitScriptFmt, exports[0])

	// XXX: could/should we instead ship a pre-build supermin appliance?
	// prepare supermin
	cmd := exec.Command(
		"supermin", "--prepare", "--use-installed",
		// XXX: external pkglist like COSA?osbuild
		// XXX2: double check list
		//
		// fundamental
		"util-linux",
		// basic networking
		"ca-certificates", "dhcp-client", "iproute",
		// loop-device support
		"kernel-modules",
		// osbuild and friends
		"osbuild", "osbuild-depsolve-dnf", "osbuild-lvm2", "osbuild-luks2", "osbuild-ostree",
		// lvm
		"lvm2",
		// target"
		"-o", superminPrepareDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("supermin prepare failed: %w", err)
	}
	if err := addInitTar(superminPrepareDir, superminInitScript); err != nil {
		return err
	}

	// build supermin
	superminDir, err := os.MkdirTemp("", "supermin-run")
	if err != nil {
		return err
	}
	defer os.RemoveAll(superminDir)

	cmd = exec.Command(
		"supermin",
		"--build", superminPrepareDir,
		// XXX: what is the right size?
		"--size", "10G",
		"-f", "ext2",
		"-o", superminDir,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("supermin-build failed: %w", err)
	}

	// XXX: should we do this here?
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return err
	}
	// XXX: clutters the output dir
	if err := os.WriteFile(filepath.Join(opts.OutputDir, "manifest.json"), manifest, 0644); err != nil {
		return err
	}

	// XXX: haaaaaaaaaaaaaaaaaaack, we cannot write to the root owned
	// /var/cache/image-builder otherwise
	if os.Getuid() != 0 {
		opts.StoreDir = ".store"
		os.MkdirAll(".store", 0755)
	}

	// run qemu
	cmd = exec.Command(
		"qemu-kvm",
		"-nodefaults", "-nographic",
		"-cpu", "host",
		// XXX: what is the right size
		"-m", "2048",
		"-accel", "kvm",
		"-netdev", "user,id=eth0",
		// XXX: see colins osbuildbootc/cosa for $arch options
		"-device", "virtio-net-pci,netdev=eth0",
		"-kernel", filepath.Join(superminDir, "kernel"),
		"-initrd", filepath.Join(superminDir, "initrd"),
		// XXX: double check e.g. security model
		"-virtfs", fmt.Sprintf("local,path=%s,security_model=none,mount_tag=osbuild_output", opts.OutputDir),
		"-virtfs", fmt.Sprintf("local,path=%s,security_model=none,mount_tag=osbuild_store", opts.StoreDir),
		"-hda", filepath.Join(superminDir, "root"),
		// XXX: see colins osbuildbootc/cosa for $arch options
		"-serial", "stdio",
		"-append", "console=ttyS0 root=/dev/sda",
	)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}
