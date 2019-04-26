// +build !linux

package houdini

import (
	"errors"
	"os/exec"

	"code.cloudfoundry.org/garden"
)

func (container *container) setupPrivileged() error {
	return errors.New("nope")
}

func (container *container) cmdPrivileged(spec garden.ProcessSpec) (*exec.Cmd, error) {
	return nil, errors.New("nope")
}
