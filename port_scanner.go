package main

import (
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

var (
	modiphlpapi = syscall.NewLazyDLL("iphlpapi.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")
	modpsapi    = syscall.NewLazyDLL("psapi.dll")

	procGetExtendedTcpTable = modiphlpapi.NewProc("GetExtendedTcpTable")
	procGetExtendedUdpTable = modiphlpapi.NewProc("GetExtendedUdpTable")

	procOpenProcess                = modkernel32.NewProc("OpenProcess")
	procTerminateProcess           = modkernel32.NewProc("TerminateProcess")
	procCloseHandle                = modkernel32.NewProc("CloseHandle")
	procCreateToolhelp32Snapshot   = modkernel32.NewProc("CreateToolhelp32Snapshot")
	procProcess32FirstW            = modkernel32.NewProc("Process32FirstW")
	procProcess32NextW             = modkernel32.NewProc("Process32NextW")
	procQueryFullProcessImageNameW = modkernel32.NewProc("QueryFullProcessImageNameW")

	procGetProcessMemoryInfo = modpsapi.NewProc("GetProcessMemoryInfo")
)

const (
	AF_INET                = 2
	TCP_TABLE_OWNER_PID_ALL = 5
	UDP_TABLE_OWNER_PID     = 1

	PROCESS_TERMINATE                  = 0x0001
	PROCESS_QUERY_INFORMATION          = 0x0400
	PROCESS_QUERY_LIMITED_INFORMATION  = 0x1000
	PROCESS_VM_READ                    = 0x0010

	TH32CS_SNAPPROCESS = 0x00000002

	MIB_TCP_STATE_CLOSED     = 1
	MIB_TCP_STATE_LISTEN     = 2
	MIB_TCP_STATE_SYN_SENT   = 3
	MIB_TCP_STATE_SYN_RCVD   = 4
	MIB_TCP_STATE_ESTAB      = 5
	MIB_TCP_STATE_FIN_WAIT1  = 6
	MIB_TCP_STATE_FIN_WAIT2  = 7
	MIB_TCP_STATE_CLOSE_WAIT = 8
	MIB_TCP_STATE_CLOSING    = 9
	MIB_TCP_STATE_LAST_ACK   = 10
	MIB_TCP_STATE_TIME_WAIT  = 11
	MIB_TCP_STATE_DELETE_TCB = 12
)

type ProcessPort struct {
	ID        int    `json:"id"`
	Port      int    `json:"port"`
	Proto     string `json:"proto"`
	Name      string `json:"name"`
	PID       uint32 `json:"pid"`
	State     string `json:"state"`
	Mem       string `json:"mem"`
	Path      string `json:"path"`
	Cat       string `json:"cat"`
	Conflict  bool   `json:"conflict"`
	Protected bool   `json:"protected"`
}

type PROCESS_MEMORY_COUNTERS struct {
	CB                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

type PROCESSENTRY32W struct {
	DwSize              uint32
	CntUsage            uint32
	Th32ProcessID       uint32
	Th32DefaultHeapID   uintptr
	Th32ModuleID        uint32
	CntThreads          uint32
	Th32ParentProcessID uint32
	PcPriClassBase      int32
	DwFlags             uint32
	SzExeFile           [260]uint16
}

type MIB_TCPROW_OWNER_PID struct {
	DwState      uint32
	DwLocalAddr  uint32
	DwLocalPort  uint32
	DwRemoteAddr uint32
	DwRemotePort uint32
	DwOwningPid  uint32
}

type MIB_UDPROW_OWNER_PID struct {
	DwLocalAddr uint32
	DwLocalPort uint32
	DwOwningPid uint32
}

// 获取所有监听与连接的端口信息
func QueryAllPorts() ([]ProcessPort, error) {
	var results []ProcessPort
	idCounter := 1
	portSeen := make(map[string]bool)

	// 1. 获取 TCP 端口表
	tcpPorts, err := queryTcpPorts()
	if err == nil {
		for _, p := range tcpPorts {
			key := fmt.Sprintf("TCP-%d-%d", p.Port, p.PID)
			if !portSeen[key] {
				portSeen[key] = true
				p.ID = idCounter
				idCounter++
				results = append(results, p)
			}
		}
	}

	// 2. 获取 UDP 端口表
	udpPorts, err := queryUdpPorts()
	if err == nil {
		for _, p := range udpPorts {
			key := fmt.Sprintf("UDP-%d-%d", p.Port, p.PID)
			if !portSeen[key] {
				portSeen[key] = true
				p.ID = idCounter
				idCounter++
				results = append(results, p)
			}
		}
	}

	return results, nil
}

func queryTcpPorts() ([]ProcessPort, error) {
	var size uint32 = 0
	procGetExtendedTcpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, AF_INET, TCP_TABLE_OWNER_PID_ALL, 0)
	if size == 0 {
		return nil, fmt.Errorf("failed to query tcp table size")
	}

	buf := make([]byte, size)
	ret, _, _ := procGetExtendedTcpTable.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		AF_INET,
		TCP_TABLE_OWNER_PID_ALL,
		0,
	)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedTcpTable error: %d", ret)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	var list []ProcessPort
	offset := 4
	rowSize := int(unsafe.Sizeof(MIB_TCPROW_OWNER_PID{}))

	for i := uint32(0); i < numEntries; i++ {
		if offset+rowSize > len(buf) {
			break
		}
		row := (*MIB_TCPROW_OWNER_PID)(unsafe.Pointer(&buf[offset]))
		offset += rowSize

		port := int(binary.BigEndian.Uint16([]byte{byte(row.DwLocalPort), byte(row.DwLocalPort >> 8)}))
		pid := row.DwOwningPid

		stateStr := "UNKNOWN"
		if row.DwState == MIB_TCP_STATE_LISTEN {
			stateStr = "LISTENING"
		} else if row.DwState == MIB_TCP_STATE_ESTAB {
			stateStr = "ESTABLISHED"
		} else if row.DwState == MIB_TCP_STATE_TIME_WAIT {
			stateStr = "TIME_WAIT"
		} else if row.DwState == MIB_TCP_STATE_CLOSE_WAIT {
			stateStr = "CLOSE_WAIT"
		}

		name, path, mem, isProtected := getProcessDetails(pid)
		cat := categorizePort(port, name)
		conflict := (port == 8080 || port == 3000 || port == 5173 || port == 8000) && !isProtected

		list = append(list, ProcessPort{
			Port:      port,
			Proto:     "TCP",
			Name:      name,
			PID:       pid,
			State:     stateStr,
			Mem:       mem,
			Path:      path,
			Cat:       cat,
			Conflict:  conflict,
			Protected: isProtected,
		})
	}

	return list, nil
}

func queryUdpPorts() ([]ProcessPort, error) {
	var size uint32 = 0
	procGetExtendedUdpTable.Call(0, uintptr(unsafe.Pointer(&size)), 0, AF_INET, UDP_TABLE_OWNER_PID, 0)
	if size == 0 {
		return nil, fmt.Errorf("failed to query udp table size")
	}

	buf := make([]byte, size)
	ret, _, _ := procGetExtendedUdpTable.Call(
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
		0,
		AF_INET,
		UDP_TABLE_OWNER_PID,
		0,
	)
	if ret != 0 {
		return nil, fmt.Errorf("GetExtendedUdpTable error: %d", ret)
	}

	numEntries := binary.LittleEndian.Uint32(buf[0:4])
	var list []ProcessPort
	offset := 4
	rowSize := int(unsafe.Sizeof(MIB_UDPROW_OWNER_PID{}))

	for i := uint32(0); i < numEntries; i++ {
		if offset+rowSize > len(buf) {
			break
		}
		row := (*MIB_UDPROW_OWNER_PID)(unsafe.Pointer(&buf[offset]))
		offset += rowSize

		port := int(binary.BigEndian.Uint16([]byte{byte(row.DwLocalPort), byte(row.DwLocalPort >> 8)}))
		pid := row.DwOwningPid

		name, path, mem, isProtected := getProcessDetails(pid)
		cat := categorizePort(port, name)

		list = append(list, ProcessPort{
			Port:      port,
			Proto:     "UDP",
			Name:      name,
			PID:       pid,
			State:     "LISTENING",
			Mem:       mem,
			Path:      path,
			Cat:       cat,
			Conflict:  false,
			Protected: isProtected,
		})
	}

	return list, nil
}

// 获取进程的详细路径与内存占用
func getProcessDetails(pid uint32) (name, path, mem string, isProtected bool) {
	if pid == 0 {
		return "System Idle Process", "ntoskrnl.exe", "0 MB", true
	}
	if pid == 4 {
		return "System", "ntoskrnl.exe", "12 MB", true
	}

	hProcess, _, _ := procOpenProcess.Call(
		PROCESS_QUERY_LIMITED_INFORMATION|PROCESS_VM_READ,
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		// 尝试仅有限查询
		hProcess, _, _ = procOpenProcess.Call(
			PROCESS_QUERY_LIMITED_INFORMATION,
			0,
			uintptr(pid),
		)
	}

	name = fmt.Sprintf("PID_%d", pid)
	path = "N/A (系统/受限权限)"
	mem = "--"
	isProtected = false

	if hProcess != 0 {
		defer procCloseHandle.Call(hProcess)

		// 1. 查询完整可执行文件路径
		var buf [1024]uint16
		var bufLen uint32 = 1024
		ret, _, _ := procQueryFullProcessImageNameW.Call(
			hProcess,
			0,
			uintptr(unsafe.Pointer(&buf[0])),
			uintptr(unsafe.Pointer(&bufLen)),
		)
		if ret != 0 && bufLen > 0 {
			fullPath := syscall.UTF16ToString(buf[:bufLen])
			path = fullPath
			name = filepath.Base(fullPath)
		}

		// 2. 查询内存工作集
		var pmc PROCESS_MEMORY_COUNTERS
		pmc.CB = uint32(unsafe.Sizeof(pmc))
		memRet, _, _ := procGetProcessMemoryInfo.Call(
			hProcess,
			uintptr(unsafe.Pointer(&pmc)),
			uintptr(pmc.CB),
		)
		if memRet != 0 {
			mb := float64(pmc.WorkingSetSize) / (1024 * 1024)
			mem = fmt.Sprintf("%.1f MB", mb)
		}
	}

	// 关键系统进程保护
	lowName := strings.ToLower(name)
	if pid < 100 || lowName == "system" || lowName == "smss.exe" || lowName == "csrss.exe" ||
		lowName == "wininit.exe" || lowName == "services.exe" || lowName == "lsass.exe" ||
		lowName == "svchost.exe" {
		isProtected = true
	}

	return name, path, mem, isProtected
}

func categorizePort(port int, name string) string {
	lowName := strings.ToLower(name)
	if port == 80 || port == 443 || port == 8080 || port == 3000 || port == 8081 {
		return "web"
	}
	if port == 3306 || port == 6379 || port == 5432 || port == 27017 || port == 1433 {
		return "db"
	}
	if port == 5173 || port == 8000 || port == 9000 || port == 4200 || port == 7001 {
		return "dev"
	}
	if strings.Contains(lowName, "node") || strings.Contains(lowName, "nginx") {
		return "web"
	}
	if strings.Contains(lowName, "sql") || strings.Contains(lowName, "redis") || strings.Contains(lowName, "mongo") {
		return "db"
	}
	return "dev"
}

// 强制终止目标进程及其子进程 (Tree-Kill 级联树杀)
func KillProcessTree(pid uint32) (bool, string) {
	if pid <= 4 {
		return false, "系统关键进程受 Windows 内核保护，拒绝终止"
	}

	// 1. 递归查找并杀死所有子进程
	childPids := findChildPIDs(pid)
	for _, cpid := range childPids {
		killSingleProcess(cpid)
	}

	// 2. 终止主进程
	success, msg := killSingleProcess(pid)
	return success, msg
}

func killSingleProcess(pid uint32) (bool, string) {
	hProcess, _, _ := procOpenProcess.Call(
		PROCESS_TERMINATE,
		0,
		uintptr(pid),
	)
	if hProcess == 0 {
		return false, fmt.Sprintf("无法获取进程句柄 (PID: %d)，权限不足或进程已退出", pid)
	}
	defer procCloseHandle.Call(hProcess)

	ret, _, _ := procTerminateProcess.Call(hProcess, 1)
	if ret == 0 {
		return false, fmt.Sprintf("终止进程失败 (PID: %d)", pid)
	}
	return true, "成功"
}

// 查找子进程
func findChildPIDs(parentPid uint32) []uint32 {
	var children []uint32
	hSnapshot, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPPROCESS, 0)
	if hSnapshot == uintptr(syscall.InvalidHandle) {
		return children
	}
	defer procCloseHandle.Call(hSnapshot)

	var entry PROCESSENTRY32W
	entry.DwSize = uint32(unsafe.Sizeof(entry))

	ret, _, _ := procProcess32FirstW.Call(hSnapshot, uintptr(unsafe.Pointer(&entry)))
	for ret != 0 {
		if entry.Th32ParentProcessID == parentPid {
			children = append(children, entry.Th32ProcessID)
			// 递归子级的子级
			children = append(children, findChildPIDs(entry.Th32ProcessID)...)
		}
		ret, _, _ = procProcess32NextW.Call(hSnapshot, uintptr(unsafe.Pointer(&entry)))
	}

	return children
}
