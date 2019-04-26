package houdini

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"code.cloudfoundry.org/garden"
)

func (container *container) setup() error {
	if container.checkPrivileged() {
		return container.setupPrivileged()
	} else {
		return container.setupUnprivileged()
	}
}

func (container *container) setupUnprivileged() error {
	for _, bm := range container.spec.BindMounts {
		if bm.Mode == garden.BindMountModeRO {
			return errors.New("read-only bind mounts are unsupported")
		}

		dest := filepath.Join(container.workDir, bm.DstPath)
		_, err := os.Stat(dest)
		if err == nil {
			err = os.Remove(dest)
			if err != nil {
				return fmt.Errorf("failed to remove destination for bind mount: %s", err)
			}
		}

		err = os.MkdirAll(filepath.Dir(dest), 0755)
		if err != nil {
			return fmt.Errorf("failed to create parent dir for bind mount: %s", err)
		}

		absSrc, err := filepath.Abs(bm.SrcPath)
		if err != nil {
			return fmt.Errorf("failed to resolve source path: %s", err)
		}

		// windows symlinks ("junctions") support directories, but not hard-links
		// darwin hardlinks have strange restrictions
		// symlinks behave reasonably similar to bind mounts on OS X (unlike Linux)
		err = os.Symlink(absSrc, dest)
		if err != nil {
			return fmt.Errorf("failed to create hardlink for bind mount: %s", err)
		}
	}

	return nil
}

func (container *container) cmd(spec garden.ProcessSpec) (*exec.Cmd, error) {
	if container.checkPrivileged() {
		return container.cmdPrivileged(spec)
	} else {
		return container.cmdUnprivileged(spec)
	}
}

func (container *container) cmdUnprivileged(spec garden.ProcessSpec) (*exec.Cmd, error) {
	cmd := exec.Command(filepath.FromSlash(spec.Path), spec.Args...)
	cmd.Env = append(os.Environ(), append(container.env, spec.Env...)...)
	cmd.Dir = filepath.Join(container.workDir, filepath.FromSlash(spec.Dir))

	return cmd, nil
}

func (container *container) checkPrivileged() bool {
	return runtime.GOOS == "linux" && container.privileged
}

const defaultRootPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"

// const defaultPath = "/usr/local/bin:/usr/bin:/bin"

func (container *container) path() string {
	var path string
	for _, env := range container.env {
		segs := strings.SplitN(env, "=", 2)
		if len(segs) < 2 {
			continue
		}

		if segs[0] == "PATH" {
			path = segs[1]
		}
	}

	if !container.hasRootfs {
		if path == "" {
			path = os.Getenv("PATH")
		}

		return path
	}

	if path == "" {
		// assume running as root for now, since Houdini doesn't currently support
		// running as a user
		path = defaultRootPath
	}

	var scopedPath string
	for _, dir := range filepath.SplitList(path) {
		if scopedPath != "" {
			scopedPath += string(filepath.ListSeparator)
		}

		scopedPath += container.workDir + dir
	}

	return scopedPath
}

func findExecutable(file string) error {
	d, err := os.Stat(file)
	if err != nil {
		return err
	}
	if m := d.Mode(); !m.IsDir() && m&0111 != 0 {
		return nil
	}
	return os.ErrPermission
}

// based on exec.LookPath from stdlib
func lookPath(file string, path string) (string, error) {
	if strings.Contains(file, "/") {
		err := findExecutable(file)
		if err == nil {
			return file, nil
		}
		return "", &exec.Error{Name: file, Err: err}
	}

	for _, dir := range filepath.SplitList(path) {
		if dir == "" {
			// Unix shell semantics: path element "" means "."
			dir = "."
		}
		path := filepath.Join(dir, file)
		if err := findExecutable(path); err == nil {
			return path, nil
		}
	}

	return "", &exec.Error{Name: file, Err: exec.ErrNotFound}
}
