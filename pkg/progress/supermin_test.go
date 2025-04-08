package progress_test

import (
	"archive/tar"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/image-builder-cli/pkg/progress"
)

func TestWaitForFilesFile(t *testing.T) {
	tmpdir := t.TempDir()

	// trivial case, no file appears, we error
	start := time.Now()
	canary1 := filepath.Join(tmpdir, "f1.txt")
	err := progress.WaitForFiles(200*time.Millisecond, canary1)
	assert.EqualError(t, err, fmt.Sprintf("files missing after 200ms: [%s]", canary1))
	// ensure we waited untilthe timeout
	assert.True(t, time.Since(start) >= 200*time.Millisecond)

	// trivial case, file is already there before we wait
	err = os.WriteFile(canary1, nil, 0644)
	assert.NoError(t, err)
	// use an absurd high time to ensure test timeouts if we would
	// wait a long time here
	err = progress.WaitForFiles(1*time.Hour, canary1)
	assert.NoError(t, err)

	// new file appears after 100ms
	canary2 := filepath.Join(tmpdir, "f2.txt")
	start = time.Now()
	go func() {
		time.Sleep(100 * time.Millisecond)
		err := os.WriteFile(canary2, nil, 0644)
		assert.NoError(t, err)
	}()
	err = progress.WaitForFiles(1*time.Hour, canary1, canary2)
	assert.NoError(t, err)
	assert.True(t, time.Since(start) >= 100*time.Millisecond)
	// it should take 100-200msec to get the file but to avoid
	// races in heavy loaded CI VMs we are conservative here
	assert.True(t, time.Since(start) <= time.Second)
}

func fileExists(path string) bool {
	_, err := os.Stat("/usr/libexec/virtiofsd")
	return err == nil
}

func ensureExecutables(t *testing.T, exes ...string) {
	var missing []string
	for _, exe := range exes {
		p, err := exec.LookPath(exe)
		if err != nil || p == "" {
			missing = append(missing, exe)
		}
	}
	if len(missing) > 0 {
		t.Skipf("missing executables: %v", missing)
	}
}

func injectFakeOsbuild(t *testing.T, prepareDir, fakeOsbuild string) {
	initTarF, err := os.Create(filepath.Join(prepareDir, "extra.tar.gz"))
	require.NoError(t, err)
	defer initTarF.Close()

	initTar := tar.NewWriter(initTarF)
	defer initTar.Close()
	err = initTar.WriteHeader(&tar.Header{
		Name: "usr/bin/osbuild.fake",
		Size: int64(len(fakeOsbuild)),
		Mode: 0755,
	})
	require.NoError(t, err)
	_, err = initTar.Write([]byte(fakeOsbuild))
	require.NoError(t, err)
}

func TestSuperminSmoke(t *testing.T) {
	ensureExecutables(t, "supermin", "qemu-kvm", "dhclient")
	if !fileExists("/usr/libexec/virtiofsd") {
		t.Skip("need virtiofsd")
	}

	tmpdir := t.TempDir()
	prepareDir := filepath.Join(tmpdir, "prepare")
	runDir := filepath.Join(tmpdir, "run")
	outputDir := filepath.Join(tmpdir, "output")
	storeDir := filepath.Join(tmpdir, "store")
	for _, d := range []string{outputDir, storeDir} {
		err := os.MkdirAll(d, 0755)
		assert.NoError(t, err)
	}

	// XXX: this is slightly ugly, just injecting /usr/bin/osbuild over the existing package
	// will not work because packages seem to be injected by supermin after the {base,extra}.tgz
	restore := progress.MockSuperminInitScriptOsbuildPath("osbuild.fake")
	defer restore()

	fakeExport := "image"
	err := progress.SuperminPrepare(prepareDir, fakeExport)
	assert.NoError(t, err)

	fakeOsbuild := `#!/bin/sh -e
echo "args: $@" >> /output/osbuild_calls.txt
echo osbuild-stdout-output
>&2 echo osbuild-stderr-output
`
	injectFakeOsbuild(t, prepareDir, fakeOsbuild)

	err = progress.SuperminBuild(prepareDir, runDir)
	assert.NoError(t, err)

	err = progress.SuperminQemu(tmpdir, runDir, outputDir, storeDir)
	assert.NoError(t, err)
}
