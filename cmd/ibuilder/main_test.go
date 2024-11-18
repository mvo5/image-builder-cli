package main_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	testrepos "github.com/osbuild/images/test/data/repositories"

	"github.com/osbuild/image-builder-cli/cmd/ibuilder"
)

func TestListImagesSmoke(t *testing.T) {
	restore := main.MockNewRepoRegistry(testrepos.New)
	defer restore()

	restore = main.MockOsArgs([]string{"list-images"})
	defer restore()

	var fakeStdout bytes.Buffer
	restore = main.MockOsStdout(&fakeStdout)
	defer restore()

	err := main.Run()
	assert.NoError(t, err)
	// output is sorted
	println(fakeStdout.String())
	assert.Regexp(t, `(?ms)rhel-8.9.*rhel-8.10`, fakeStdout.String())
}
