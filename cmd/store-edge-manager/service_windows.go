//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	managerServiceWin32OwnProcess                            = 0x00000010
	managerServiceStopped                                    = 0x00000001
	managerServiceStartPending                               = 0x00000002
	managerServiceStopPending                                = 0x00000003
	managerServiceRunning                                    = 0x00000004
	managerServiceAcceptStop                                 = 0x00000001
	managerServiceAcceptShutdown                             = 0x00000004
	managerServiceControlStop                                = 0x00000001
	managerServiceControlShutdown                            = 0x00000005
	managerServiceControlInterrogate                         = 0x00000004
	managerErrorFailedServiceControllerConnect syscall.Errno = 1063
)

type managerWinServiceTableEntry struct {
	name *uint16
	proc uintptr
}
type managerWinServiceStatus struct{ serviceType, currentState, controlsAccepted, win32ExitCode, serviceSpecificExitCode, checkPoint, waitHint uint32 }

var (
	managerAdvapi32                          = syscall.NewLazyDLL("advapi32.dll")
	managerProcStartServiceCtrlDispatcherW   = managerAdvapi32.NewProc("StartServiceCtrlDispatcherW")
	managerProcRegisterServiceCtrlHandlerExW = managerAdvapi32.NewProc("RegisterServiceCtrlHandlerExW")
	managerProcSetServiceStatus              = managerAdvapi32.NewProc("SetServiceStatus")
	managerStatusHandle                      uintptr
	managerCancel                            context.CancelFunc
	managerRunner                            func(context.Context, bool) error
	managerMainCallback                      uintptr
	managerCtrlCallback                      uintptr
	managerNameUTF16                         *uint16
)

func runWindowsManagerService(name string, run func(context.Context, bool) error) error {
	if run == nil {
		return errors.New("service runner is nil")
	}
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	managerNameUTF16 = namePtr
	managerRunner = run
	managerMainCallback = syscall.NewCallback(windowsManagerServiceMain)
	managerCtrlCallback = syscall.NewCallback(windowsManagerServiceControl)
	table := [...]managerWinServiceTableEntry{{name: managerNameUTF16, proc: managerMainCallback}, {}}
	r1, _, callErr := managerProcStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
	if r1 == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == managerErrorFailedServiceControllerConnect {
			return fmt.Errorf("Store Edge Manager service mode must be started by Windows Service Control Manager")
		}
		return fmt.Errorf("StartServiceCtrlDispatcherW: %w", callErr)
	}
	return nil
}
func windowsManagerServiceMain(argc uintptr, argv uintptr) uintptr {
	handle, _, _ := managerProcRegisterServiceCtrlHandlerExW.Call(uintptr(unsafe.Pointer(managerNameUTF16)), managerCtrlCallback, 0)
	if handle == 0 {
		return 0
	}
	managerStatusHandle = handle
	setWindowsManagerServiceStatus(managerServiceStartPending, 0, 1, 10000)
	ctx, cancel := context.WithCancel(context.Background())
	managerCancel = cancel
	setWindowsManagerServiceStatus(managerServiceRunning, managerServiceAcceptStop|managerServiceAcceptShutdown, 0, 0)
	err := managerRunner(ctx, true)
	cancel()
	setWindowsManagerServiceStatus(managerServiceStopped, 0, 0, 0)
	if err != nil {
		return 1
	}
	return 0
}
func windowsManagerServiceControl(control, eventType, eventData, handlerContext uintptr) uintptr {
	switch uint32(control) {
	case managerServiceControlStop, managerServiceControlShutdown:
		setWindowsManagerServiceStatus(managerServiceStopPending, 0, 1, 15000)
		if managerCancel != nil {
			managerCancel()
		}
	case managerServiceControlInterrogate:
	}
	return 0
}
func setWindowsManagerServiceStatus(state, accepted, checkpoint, waitHint uint32) {
	if managerStatusHandle == 0 {
		return
	}
	status := managerWinServiceStatus{serviceType: managerServiceWin32OwnProcess, currentState: state, controlsAccepted: accepted, checkPoint: checkpoint, waitHint: waitHint}
	_, _, _ = managerProcSetServiceStatus.Call(managerStatusHandle, uintptr(unsafe.Pointer(&status)))
}
