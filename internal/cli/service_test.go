package cli

import (
	"bytes"
	"context"
	"testing"
)

func TestServiceLifecycleCommands(t *testing.T) {
	t.Parallel()

	manager := &fakeServiceManager{}
	opts := Options{
		Stdout: &bytes.Buffer{},
		Stderr: &bytes.Buffer{},
		newServiceManager: func() (serviceManager, error) {
			return manager, nil
		},
	}

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "start", args: []string{"service", "start"}, want: "start"},
		{name: "stop", args: []string{"service", "stop"}, want: "stop"},
		{name: "restart", args: []string{"service", "restart"}, want: "restart"},
		{name: "status", args: []string{"service", "status"}, want: "status"},
	}

	for _, tt := range cases {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cmd := NewRootCommand(opts)
			cmd.SetArgs(tt.args)
			if err := cmd.Execute(); err != nil {
				t.Fatalf("Execute() error = %v", err)
			}

			if !manager.called(tt.want) {
				t.Fatalf("expected %q to be called, calls=%v", tt.want, manager.calls)
			}
		})
	}
}

func TestServiceInstallAndUninstallCommands(t *testing.T) {
	t.Parallel()

	manager := &fakeServiceManager{}
	var stdout bytes.Buffer
	opts := Options{
		Stdout: &stdout,
		Stderr: &bytes.Buffer{},
		newServiceManager: func() (serviceManager, error) {
			return manager, nil
		},
	}

	for _, args := range [][]string{
		{"service", "install"},
		{"service", "uninstall"},
	} {
		cmd := NewRootCommand(opts)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("Execute(%v) error = %v", args, err)
		}
	}

	if !manager.called("install") {
		t.Fatalf("expected install to be called, calls=%v", manager.calls)
	}
	if !manager.called("uninstall") {
		t.Fatalf("expected uninstall to be called, calls=%v", manager.calls)
	}
}

type fakeServiceManager struct {
	calls []string
}

func (f *fakeServiceManager) Install(context.Context) (string, error) {
	f.calls = append(f.calls, "install")
	return "/tmp/callmeback.service", nil
}

func (f *fakeServiceManager) Uninstall(context.Context) (string, error) {
	f.calls = append(f.calls, "uninstall")
	return "/tmp/callmeback.service", nil
}

func (f *fakeServiceManager) Status(context.Context) (string, error) {
	f.calls = append(f.calls, "status")
	return "running", nil
}

func (f *fakeServiceManager) Start(context.Context) error {
	f.calls = append(f.calls, "start")
	return nil
}

func (f *fakeServiceManager) Stop(context.Context) error {
	f.calls = append(f.calls, "stop")
	return nil
}

func (f *fakeServiceManager) Restart(context.Context) error {
	f.calls = append(f.calls, "restart")
	return nil
}

func (f *fakeServiceManager) called(name string) bool {
	for _, call := range f.calls {
		if call == name {
			return true
		}
	}
	return false
}
