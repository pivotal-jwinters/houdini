package houdini

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"code.cloudfoundry.org/garden"
)

func (container *container) setupPrivileged() error {
	if container.hasRootfs {
		for _, dir := range []string{"/proc", "/dev", "/sys"} {
			dest := filepath.Join(container.workDir, dir)

			err := os.MkdirAll(dest, 0755)
			if err != nil {
				return fmt.Errorf("failed to create target for bind mount: %s", err)
			}

			err = syscall.Mount(dir, dest, "none", syscall.MS_BIND|syscall.MS_RDONLY, "")
			if err != nil {
				return err
			}
		}

		for _, file := range []string{"/etc/resolv.conf", "/etc/hosts"} {
			dest := filepath.Join(container.workDir, file)

			f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				return fmt.Errorf("failed to create target for bind mount: %s", err)
			}

			err = f.Close()
			if err != nil {
				return err
			}

			err = syscall.Mount(file, dest, "none", syscall.MS_BIND|syscall.MS_RDONLY, "")
			if err != nil {
				return err
			}
		}
	}

	for _, bm := range container.spec.BindMounts {
		dest := filepath.Join(container.workDir, bm.DstPath)

		err := os.MkdirAll(dest, 0755)
		if err != nil {
			return fmt.Errorf("failed to create target for bind mount: %s", err)
		}

		flags := uintptr(syscall.MS_BIND)
		if bm.Mode == garden.BindMountModeRO {
			flags |= syscall.MS_RDONLY
		}

		err = syscall.Mount(bm.SrcPath, dest, "none", flags, "")
		if err != nil {
			return err
		}
	}

	return nil
}

func (container *container) cmdPrivileged(spec garden.ProcessSpec) (*exec.Cmd, error) {
	var cmd *exec.Cmd

	if container.hasRootfs {
		path := spec.Path

		if !strings.Contains(path, "/") {
			// find executable within container's $PATH

			absPath, err := lookPath(path, container.path())
			if err != nil {
				return nil, garden.ExecutableNotFoundError{
					Message: err.Error(),
				}
			}

			// correct path so that it's absolute from the rootfs
			path = strings.TrimPrefix(absPath, container.workDir)
		}

		cmd = exec.Command(path, spec.Args...)

		if spec.Dir != "" {
			cmd.Dir = spec.Dir
		} else {
			cmd.Dir = "/"
		}

		cmd.SysProcAttr = &syscall.SysProcAttr{
			Chroot: container.workDir,
		}
	} else {
		cmd = exec.Command(spec.Path, spec.Args...)
		cmd.Dir = filepath.Join(container.workDir, spec.Dir)
	}

	cmd.Env = append(os.Environ(), append(container.env, spec.Env...)...)

	return cmd, nil
}
