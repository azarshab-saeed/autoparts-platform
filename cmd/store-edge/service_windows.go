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
	serviceWin32OwnProcess                            = 0x00000010
	serviceStopped                                    = 0x00000001
	serviceStartPending                               = 0x00000002
	serviceStopPending                                = 0x00000003
	serviceRunning                                    = 0x00000004
	serviceAcceptStop                                 = 0x00000001
	serviceAcceptShutdown                             = 0x00000004
	serviceControlStop                                = 0x00000001
	serviceControlShutdown                            = 0x00000005
	serviceControlInterrogate                         = 0x00000004
	errorFailedServiceControllerConnect syscall.Errno = 1063
)

type winServiceTableEntry struct {
	name *uint16
	proc uintptr
}

type winServiceStatus struct {
	serviceType             uint32
	currentState            uint32
	controlsAccepted        uint32
	win32ExitCode           uint32
	serviceSpecificExitCode uint32
	checkPoint              uint32
	waitHint                uint32
}

var (
	advapi32                          = syscall.NewLazyDLL("advapi32.dll")
	procStartServiceCtrlDispatcherW   = advapi32.NewProc("StartServiceCtrlDispatcherW")
	procRegisterServiceCtrlHandlerExW = advapi32.NewProc("RegisterServiceCtrlHandlerExW")
	procSetServiceStatus              = advapi32.NewProc("SetServiceStatus")
	serviceStatusHandle               uintptr
	serviceCancel                     context.CancelFunc
	serviceRunner                     func(context.Context, bool) error
	serviceMainCallback               uintptr
	serviceCtrlCallback               uintptr
	serviceNameUTF16                  *uint16
)

func runWindowsService(name string, run func(context.Context, bool) error) error {
	if run == nil {
		return errors.New("service runner is nil")
	}
	namePtr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		return err
	}
	serviceNameUTF16 = namePtr
	serviceRunner = run
	serviceMainCallback = syscall.NewCallback(windowsServiceMain)
	serviceCtrlCallback = syscall.NewCallback(windowsServiceControl)
	table := [...]winServiceTableEntry{
		{name: serviceNameUTF16, proc: serviceMainCallback},
		{},
	}
	r1, _, callErr := procStartServiceCtrlDispatcherW.Call(uintptr(unsafe.Pointer(&table[0])))
	if r1 == 0 {
		if errno, ok := callErr.(syscall.Errno); ok && errno == errorFailedServiceControllerConnect {
			return fmt.Errorf("Store Edge service mode must be started by Windows Service Control Manager")
		}
		return fmt.Errorf("StartServiceCtrlDispatcherW: %w", callErr)
	}
	return nil
}

func windowsServiceMain(argc uintptr, argv uintptr) uintptr {
	handle, _, callErr := procRegisterServiceCtrlHandlerExW.Call(
		uintptr(unsafe.Pointer(serviceNameUTF16)),
		serviceCtrlCallback,
		0,
	)
	if handle == 0 {
		_ = callErr
		return 0
	}
	serviceStatusHandle = handle
	setWindowsServiceStatus(serviceStartPending, 0, 1, 10000)

	ctx, cancel := context.WithCancel(context.Background())
	serviceCancel = cancel
	setWindowsServiceStatus(serviceRunning, serviceAcceptStop|serviceAcceptShutdown, 0, 0)

	err := serviceRunner(ctx, true)
	cancel()
	if err != nil {
		setWindowsServiceStatus(serviceStopped, 0, 0, 0)
		return 1
	}
	setWindowsServiceStatus(serviceStopped, 0, 0, 0)
	return 0
}

func windowsServiceControl(control uintptr, eventType uintptr, eventData uintptr, handlerContext uintptr) uintptr {
	switch uint32(control) {
	case serviceControlStop, serviceControlShutdown:
		setWindowsServiceStatus(serviceStopPending, 0, 1, 10000)
		if serviceCancel != nil {
			serviceCancel()
		}
	case serviceControlInterrogate:
		// SCM only needs the current status; no state transition is required.
	}
	return 0
}

func setWindowsServiceStatus(state uint32, accepted uint32, checkpoint uint32, waitHint uint32) {
	if serviceStatusHandle == 0 {
		return
	}
	status := winServiceStatus{
		serviceType:      serviceWin32OwnProcess,
		currentState:     state,
		controlsAccepted: accepted,
		checkPoint:       checkpoint,
		waitHint:         waitHint,
	}
	_, _, _ = procSetServiceStatus.Call(serviceStatusHandle, uintptr(unsafe.Pointer(&status)))
}
