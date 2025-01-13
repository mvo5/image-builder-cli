import json
import os
import platform
import subprocess

import pytest


@pytest.mark.parametrize("use_librepo", [False, True])
@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_builds_image(tmp_path, build_container, use_librepo):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "minimal-raw",
        "--distro", "centos-9",
        f"--use-librepo={use_librepo}",
    ])
    arch = "x86_64"
    assert (output_dir / f"centos-9-minimal-raw-{arch}/xz/disk.raw.xz").exists()
    # XXX: ensure no other leftover dirs
    dents = os.listdir(output_dir)
    assert len(dents) == 1, f"too many dentries in output dir: {dents}"


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_manifest_generates_sbom(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_container,
        "manifest",
        "minimal-raw",
        "--distro", "centos-9",
        "--with-sbom",
    ], stdout=subprocess.DEVNULL)
    arch = platform.processor()
    fn = f"centos-9-minimal-raw-{arch}/centos-9-minimal-raw-{arch}.image-os.spdx.json"
    image_sbom_json_path = output_dir / fn
    assert image_sbom_json_path.exists()
    fn = f"centos-9-minimal-raw-{arch}/centos-9-minimal-raw-{arch}.buildroot-build.spdx.json"
    buildroot_sbom_json_path = output_dir / fn
    assert buildroot_sbom_json_path.exists()
    sbom_json = json.loads(image_sbom_json_path.read_text())
    # smoke test that we have glibc in the json doc
    assert "glibc" in [s["name"] for s in sbom_json["Document"]["packages"]]


@pytest.mark.skipif(os.getuid() != 0, reason="needs root")
def test_container_with_host_resources(tmp_path, build_container):
    output_dir = tmp_path / "output"
    output_dir.mkdir()

    # include a file from the "host", because we use a container
    # to build its a bit indirect
    canary_data = "fromHost:canary data"
    canary_path = output_dir / "fromHost_canary.txt"
    canary_path.write_text(canary_data)
    blueprint_path = output_dir / "blueprint.json"
    blueprint_path.write_text("""
        {
      "customizations": {
        "files": [
          {
            "path": "/etc/fromHost.txt",
            "from_host": "/output/fromHost_canary.txt"
          }
        ]
      }
    }
    """)
    subprocess.check_call([
        "podman", "run",
        "--privileged",
        "-v", f"{output_dir}:/output",
        build_container,
        "build",
        "tar",
        # XXX: pass blueprint via stdin?
        "--blueprint=/output/blueprint.json",
        "--distro", "centos-9"
    ])
    arch = "x86_64"
    img_path = output_dir / f"centos-9-tar-{arch}/archive/root.tar.xz"
    output = subprocess.check_output([
        "tar", "xOf", os.fspath(img_path),
        "./etc/fromHost.txt",
    ], text=True)
    assert output == canary_data
