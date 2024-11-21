# image-builder CLI

Build images from the commandline in a convenient way.

## Installation

This project is under development right now and needs to be run via:
```
$ go run github.com/osbuild/image-builder-cli/cmd/image-builder@main
```
we plan to provide rpm images as well.


## Prerequisites

Make sure to have `osbuild` installed:
```
$ dnf install osbuild
```
for the repositories you need `osbuild-composer` as well (this will
change).

## Example

To see the list of buildable images run:
```
$ ./image-builder list-images
...
centos-9 type:qcow2 arch:x86_64
...
rhel-10.0 type:ami arch:x86_64
...
```

It is possible to filter:
```
$ ./image-buidler list-image --filter ami
...
centos-9 type:ami arch:x86_64
...
rhel-8.5 type:ami arch:aarch64
...
rhel-10.0 type:ami arch:aarch64
```
or be more specific
```
$ ./image-buidler list-image --filter "arch:x86*" --filter "distro:*centos*"
centos-9 type:ami arch:x86_64
...
centos-9 type:qcow2 arch:x86_64
...
```

The following fitlers are currently supported, shell-style globbing is supported:
 * distro: the distro name (e.g. fedora-41)
 * arch: the architecture name (e.g. x86_64)
 * type: the image type name (e.g. qcow2)
 * bootmode: the bootmode (legacy, uefi, hybrid)

The output can also be switched, supported are "text", "json":
```
$ ./image-buidler list-image --output=json|jq
[
  {
    "distro": {
      "name": "centos-9"
    },
    "arch": {
      "name": "aarch64"
    },
    "image_type": {
      "name": "ami"
    }
  },
...
  {
    "distro": {
      "name": "rhel-10.0"
    },
    "arch": {
      "name": "x86_64"
    },
    "image_type": {
      "name": "wsl"
    }
  }
]

```


## FAQ

Q: Does this require a backend.
A: The osbuild binary is used to actually build the images but beyond that
   no setup is required, i.e. no daemons like osbuild-composer.
