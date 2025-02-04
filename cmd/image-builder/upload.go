package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/cheggaaa/pb/v3"
	"github.com/spf13/cobra"

	"github.com/osbuild/images/pkg/cloud"
	"github.com/osbuild/images/pkg/cloud/awscloud"
	"github.com/osbuild/images/pkg/cloud/gcp"
)

var ErrNoUploadConfig = fmt.Errorf("missing or incomplete upload configuration")

type UploadTypeUnsupportedError struct {
	typ string
}

func (e *UploadTypeUnsupportedError) Error() string {
	return fmt.Sprintf("unsupported upload type %q", e.typ)
}

var awscloudNewUploader = awscloud.NewUploader

func uploadImageWithProgress(uploader cloud.Uploader, imagePath string) error {
	f, err := os.Open(imagePath)
	if err != nil {
		return err
	}
	defer f.Close()

	// setup basic progress
	st, err := f.Stat()
	if err != nil {
		return fmt.Errorf("cannot stat upload: %v", err)
	}
	pbar := pb.New64(st.Size())
	pbar.Set(pb.Bytes, true)
	pbar.SetWriter(osStdout)
	r := pbar.NewProxyReader(f)
	pbar.Start()
	defer pbar.Finish()

	return uploader.UploadAndRegister(r, osStderr)
}

func uploaderFor(cmd *cobra.Command, typ, distro, imagePath string) (cloud.Uploader, error) {
	// auto-detect image type based on image file
	if typ == "" {
		switch {
		case strings.HasSuffix(imagePath, ".raw"):
			typ = "ami"
		case strings.HasSuffix(imagePath, ".tar.gz"):
			typ = "gce"
		}
	}

	switch typ {
	case "ami":
		return uploaderForCmdAWS(cmd)
	case "gce":
		return uploaderForCmdGCE(cmd, distro)
	default:
		return nil, &UploadTypeUnsupportedError{typ}
	}

}

func uploaderForCmdAWS(cmd *cobra.Command) (cloud.Uploader, error) {
	amiName, err := cmd.Flags().GetString("aws-ami-name")
	if err != nil {
		return nil, err
	}
	bucketName, err := cmd.Flags().GetString("aws-bucket")
	if err != nil {
		return nil, err
	}
	region, err := cmd.Flags().GetString("aws-region")
	if err != nil {
		return nil, err
	}
	if amiName == "" || bucketName == "" || region == "" {
		return nil, ErrNoUploadConfig
	}

	return awscloudNewUploader(region, bucketName, amiName, nil)
}

var gcpNewUploader = gcp.NewUploader

func uploaderForCmdGCE(cmd *cobra.Command, imgDistroName string) (cloud.Uploader, error) {
	objName, err := cmd.Flags().GetString("gce-obj-name")
	if err != nil {
		return nil, err
	}
	bucketName, err := cmd.Flags().GetString("gce-bucket")
	if err != nil {
		return nil, err
	}
	region, err := cmd.Flags().GetString("gce-region")
	if err != nil {
		return nil, err
	}
	distroName, err := cmd.Flags().GetString("gce-distro-name")
	if err != nil {
		return nil, err
	}
	if objName == "" || bucketName == "" || region == "" {
		return nil, ErrNoUploadConfig
	}
	// cmdline flag can override provided distro name
	if distroName == "" {
		distroName = imgDistroName
	}

	return gcpNewUploader(region, bucketName, objName, distroName)
}

func cmdUpload(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	uploader, err := uploaderFor(cmd, "", "", imagePath)
	if err != nil {
		return err
	}

	return uploadImageWithProgress(uploader, imagePath)
}
