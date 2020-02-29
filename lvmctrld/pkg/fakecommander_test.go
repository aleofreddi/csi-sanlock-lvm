package lvmctrld

import (
	"fmt"
	"testing"
)

type FakeCommand struct {
	exe    string
	args   []string
	rc     int
	stdout string
	stderr string
	err    error
}

type FakeCommander struct {
	t *testing.T

	executions []FakeCommand
	current    int
}

func (c *FakeCommander) Exec(exe string, args ...string) (code int, stdout, stderr []byte, err error) {
	if c.current == len(c.executions) {
		c.t.Errorf("unexpected exec invocation (%s, %v)", exe, args)
		return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
	}
	expected := c.executions[c.current]
	if expected.exe != exe || len(expected.args) != len(args) {
		c.t.Errorf("exec: got invocation (%s, %v), expected (%s, %v)", exe, args, expected.exe, expected.args)
		return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
	}
	for i, v := range expected.args {
		if v != args[i] {
			c.t.Errorf("exec: got invocation (%s, %v), expected (%s, %v)", exe, args, expected.exe, expected.args)
			return 255, []byte{}, []byte{}, fmt.Errorf("unexpected invocation")
		}
	}
	c.current++
	return expected.rc, []byte(expected.stdout), []byte(expected.stderr), expected.err
}
