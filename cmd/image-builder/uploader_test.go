package main_test

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"

	main "github.com/osbuild/image-builder-cli/cmd/image-builder"
	"github.com/osbuild/image-builder-cli/internal/testutil"
)

type fakeAwsUploader struct {
	checkCalls int

	uploadAndRegisterRead  bytes.Buffer
	uploadAndRegisterCalls int
	uploadAndRegisterErr   error
}

var _ = cloud.Uploader(&fakeAwsUploader{})

func (fa *fakeAwsUploader) Check(status io.Writer) error {
	fa.checkCalls++
	return nil
}

func (fa *fakeAwsUploader) UploadAndRegister(r io.Reader, status io.Writer) error {
	fa.uploadAndRegisterCalls++
	_, err := io.Copy(&fa.uploadAndRegisterRead, r)
	if err != nil {
		panic(err)
	}
	return fa.uploadAndRegisterErr
}

func TestUploadWithAWSMock(t *testing.T) {
	fakeDiskContent := "fake-raw-img"

	fakeImageFilePath := filepath.Join(t.TempDir(), "disk.raw")
	err := os.WriteFile(fakeImageFilePath, []byte(fakeDiskContent), 0644)
	assert.NoError(t, err)

	var regionName, bucketName, amiName string
	var fa fakeAwsUploader
	restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
		regionName = region
		bucketName = bucket
		amiName = ami
		return &fa, nil
	})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	restore = main.MockOsArgs([]string{
		"upload",
		"--aws-region=aws-region-1",
		"--aws-bucket=aws-bucket-2",
		"--aws-ami-name=aws-ami-3",
		fakeImageFilePath,
	})
	err = main.Run()
	assert.NoError(t, err)

	assert.Equal(t, regionName, "aws-region-1")
	assert.Equal(t, bucketName, "aws-bucket-2")
	assert.Equal(t, amiName, "aws-ami-3")

	assert.Equal(t, 0, fa.checkCalls)
	assert.Equal(t, 1, fa.uploadAndRegisterCalls)
	assert.Equal(t, fakeDiskContent, fa.uploadAndRegisterRead.String())
	// progress was rendered
	assert.Contains(t, fakeStdout.String(), "--] 100.00%")
}

var fakeOsbuildScriptAmiFmt = `#!/bin/sh -e
cat - > "$0".stdin
mkdir -p %[1]s/ami
echo -n %[2]s > %[1]s/ami/image.raw
`

func TestBuildAndUploadWithAWSMock(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	var regionName, bucketName, amiName string
	var fa fakeAwsUploader
	restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
		regionName = region
		bucketName = bucket
		amiName = ami
		return &fa, nil
	})
	defer restore()

	fakeDiskContent := "fake-raw-img"
	outputDir := t.TempDir()
	fakeOsbuildScript := fmt.Sprintf(fakeOsbuildScriptAmiFmt, outputDir, fakeDiskContent)
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", fakeOsbuildScript)
	defer fakeOsbuildCmd.Restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"--output-dir", outputDir,
		"--aws-region=aws-region-1",
		"--aws-bucket=aws-bucket-2",
		"--aws-ami-name=aws-ami-3",
		"ami",
		"--distro=centos-9",
	})
	err := main.Run()
	require.NoError(t, err)

	assert.Equal(t, regionName, "aws-region-1")
	assert.Equal(t, bucketName, "aws-bucket-2")
	assert.Equal(t, amiName, "aws-ami-3")
	assert.Equal(t, 1, fa.checkCalls)
	assert.Equal(t, 1, fa.uploadAndRegisterCalls)
	assert.Equal(t, fakeDiskContent, fa.uploadAndRegisterRead.String())
}

func TestBuildAmiButNotUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("manifest generation takes a while")
	}
	if !hasDepsolveDnf() {
		t.Skip("no osbuild-depsolve-dnf binary found")
	}

	fa := fakeAwsUploader{
		uploadAndRegisterErr: fmt.Errorf("upload should not be called"),
	}
	restore := main.MockAwscloudNewUploader(func(region string, bucket string, ami string, opts *awscloud.UploaderOptions) (cloud.Uploader, error) {
		return &fa, nil
	})
	defer restore()

	fakeDiskContent := "fake-raw-img"
	outputDir := t.TempDir()
	fakeOsbuildScript := fmt.Sprintf(fakeOsbuildScriptAmiFmt, outputDir, fakeDiskContent)
	fakeOsbuildCmd := testutil.MockCommand(t, "osbuild", fakeOsbuildScript)
	defer fakeOsbuildCmd.Restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	restore = main.MockOsArgs([]string{
		"build",
		"--output-dir", outputDir,
		"ami",
		"--distro=centos-9",
	})
	err := main.Run()
	require.NoError(t, err)

	assert.Equal(t, 0, fa.uploadAndRegisterCalls)
}
