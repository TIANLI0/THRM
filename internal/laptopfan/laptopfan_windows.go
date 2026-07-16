//go:build windows

package laptopfan

import (
	"fmt"
	"runtime"
	"strconv"
	"sync"

	"github.com/TIANLI0/THRM/internal/types"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

const (
	// 转速合理性上限，超过视为无效读数。
	maxReasonableRPM = 12000

	// 连续失败多少次后永久标记为不支持，停止继续尝试。
	maxConsecutiveFailures = 3
)

// fanBackend 一种机型的风扇转速读取后端。
type fanBackend struct {
	name string
	read func() (FanSpeeds, error)
}

// fanBackends 按探测顺序排列的后端列表。
var fanBackends = []fanBackend{
	{"Uniwill WMI EC", readUniwillFanSpeeds},
	{"ASUS ATK WMI", readAsusFanSpeeds},
	{"Lenovo Legion WMI", readLenovoFanSpeeds},
}

type windowsReader struct {
	logger types.Logger

	mutex       sync.Mutex
	backendIdx  int // 已选定的后端下标；-1 表示尚未探测成功
	failures    int
	unsupported bool
}

func newPlatformReader(logger types.Logger) readerImpl {
	return &windowsReader{logger: logger, backendIdx: -1}
}

func (r *windowsReader) read() (FanSpeeds, bool) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	if r.unsupported {
		return FanSpeeds{}, false
	}

	// 已锁定后端：只用它读取，连续失败则标记不支持。
	if r.backendIdx >= 0 {
		backend := fanBackends[r.backendIdx]
		speeds, err := backend.read()
		if err != nil {
			r.failures++
			if r.failures >= maxConsecutiveFailures {
				r.unsupported = true
				if r.logger != nil {
					r.logger.Info("笔记本风扇转速读取已停用（%s）: %v", backend.name, err)
				}
			}
			return FanSpeeds{}, false
		}
		r.failures = 0
		return speeds, true
	}

	// 探测阶段：依次尝试所有后端，命中即锁定。
	var lastErr error
	for i, backend := range fanBackends {
		speeds, err := backend.read()
		if err != nil {
			lastErr = err
			continue
		}
		r.backendIdx = i
		r.failures = 0
		if r.logger != nil {
			r.logger.Info("已启用笔记本内置风扇转速读取（%s）: CPU=%d RPM, GPU=%d RPM", backend.name, speeds.CPUFanRPM, speeds.GPUFanRPM)
		}
		return speeds, true
	}

	r.failures++
	if r.failures >= maxConsecutiveFailures {
		r.unsupported = true
		if r.logger != nil {
			r.logger.Info("笔记本风扇转速读取不可用（本机无受支持的 WMI 接口）: %v", lastErr)
		}
	}
	return FanSpeeds{}, false
}

// withWMIService 在当前 OS 线程上完成 COM 初始化并连接 root\WMI，
// 执行 fn 后释放全部资源。每次调用独立完成 COM 初始化，避免跨
// goroutine 的公寓线程问题；调用频率为温度采样节奏（≥1s），开销可忽略。
func withWMIService(fn func(service *ole.IDispatch) error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if initErr := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED); initErr != nil {
		oleErr, ok := initErr.(*ole.OleError)
		// S_FALSE / RPC_E_CHANGED_MODE：线程已初始化，可继续使用。
		if !ok || (oleErr.Code() != 0x00000001 && oleErr.Code() != 0x80010106) {
			return fmt.Errorf("CoInitializeEx: %w", initErr)
		}
	} else {
		defer ole.CoUninitialize()
	}

	locatorObj, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return fmt.Errorf("创建 SWbemLocator 失败: %w", err)
	}
	defer locatorObj.Release()

	locator, err := locatorObj.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return fmt.Errorf("SWbemLocator IDispatch: %w", err)
	}
	defer locator.Release()

	serviceRaw, err := oleutil.CallMethod(locator, "ConnectServer", ".", `root\WMI`)
	if err != nil {
		return fmt.Errorf("连接 root\\WMI 失败: %w", err)
	}
	service := serviceRaw.ToIDispatch()
	defer service.Release()

	return fn(service)
}

// wmiMethodCaller 绑定某 WMI 类首个实例的一个方法，可多次以不同参数调用。
type wmiMethodCaller struct {
	service *ole.IDispatch
	relPath string
	method  string
	inDef   *ole.IDispatch
}

// newWMIMethodCaller 查询 className 的首个实例，并准备 methodName 的输入参数定义。
func newWMIMethodCaller(service *ole.IDispatch, className, methodName string) (*wmiMethodCaller, error) {
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT * FROM "+className)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 失败: %w", className, err)
	}
	resultSet := resultRaw.ToIDispatch()
	defer resultSet.Release()

	itemRaw, err := oleutil.CallMethod(resultSet, "ItemIndex", 0)
	if err != nil {
		return nil, fmt.Errorf("%s 无实例: %w", className, err)
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	pathRaw, err := oleutil.GetProperty(item, "Path_")
	if err != nil {
		return nil, fmt.Errorf("读取实例 Path_ 失败: %w", err)
	}
	pathObj := pathRaw.ToIDispatch()
	relPathRaw, err := oleutil.GetProperty(pathObj, "RelPath")
	pathObj.Release()
	if err != nil {
		return nil, fmt.Errorf("读取实例 RelPath 失败: %w", err)
	}
	relPath := relPathRaw.ToString()

	classRaw, err := oleutil.CallMethod(service, "Get", className)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 类定义失败: %w", className, err)
	}
	class := classRaw.ToIDispatch()
	defer class.Release()

	methodsRaw, err := oleutil.GetProperty(class, "Methods_")
	if err != nil {
		return nil, fmt.Errorf("读取 Methods_ 失败: %w", err)
	}
	methods := methodsRaw.ToIDispatch()
	defer methods.Release()

	methodRaw, err := oleutil.CallMethod(methods, "Item", methodName)
	if err != nil {
		return nil, fmt.Errorf("%s 未提供 %s 方法: %w", className, methodName, err)
	}
	method := methodRaw.ToIDispatch()
	defer method.Release()

	inDefRaw, err := oleutil.GetProperty(method, "InParameters")
	if err != nil {
		return nil, fmt.Errorf("读取 InParameters 失败: %w", err)
	}

	return &wmiMethodCaller{
		service: service,
		relPath: relPath,
		method:  methodName,
		inDef:   inDefRaw.ToIDispatch(),
	}, nil
}

func (c *wmiMethodCaller) release() {
	if c.inDef != nil {
		c.inDef.Release()
	}
}

// call 以给定输入参数执行方法，返回输出对象上 outProp 属性的 uint32 值。
func (c *wmiMethodCaller) call(params map[string]interface{}, outProp string) (uint32, error) {
	inRaw, err := oleutil.CallMethod(c.inDef, "SpawnInstance_")
	if err != nil {
		return 0, fmt.Errorf("SpawnInstance_ 失败: %w", err)
	}
	in := inRaw.ToIDispatch()
	defer in.Release()

	for name, value := range params {
		if _, err := oleutil.PutProperty(in, name, value); err != nil {
			return 0, fmt.Errorf("设置 %s 参数失败: %w", name, err)
		}
	}

	outRaw, err := oleutil.CallMethod(c.service, "ExecMethod", c.relPath, c.method, in)
	if err != nil {
		return 0, fmt.Errorf("%s 调用失败: %w", c.method, err)
	}
	outObj := outRaw.ToIDispatch()
	defer outObj.Release()

	retRaw, err := oleutil.GetProperty(outObj, outProp)
	if err != nil {
		return 0, fmt.Errorf("%s 缺少输出 %s: %w", c.method, outProp, err)
	}
	defer retRaw.Clear()

	value, err := variantToUint32(retRaw)
	if err != nil {
		return 0, fmt.Errorf("%s 输出 %s 异常: %w", c.method, outProp, err)
	}
	return value, nil
}

func validateSpeeds(speeds FanSpeeds) (FanSpeeds, error) {
	if speeds.CPUFanRPM > maxReasonableRPM || speeds.GPUFanRPM > maxReasonableRPM {
		return FanSpeeds{}, fmt.Errorf("转速读数超出合理范围: %d/%d", speeds.CPUFanRPM, speeds.GPUFanRPM)
	}
	return speeds, nil
}

func variantToUint32(v *ole.VARIANT) (uint32, error) {
	switch value := v.Value().(type) {
	case int32:
		return uint32(value), nil
	case uint32:
		return value, nil
	case int64:
		return uint32(value), nil
	case uint64:
		return uint32(value), nil
	case int:
		return uint32(value), nil
	case string:
		parsed, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			return 0, err
		}
		return uint32(parsed), nil
	default:
		return 0, fmt.Errorf("未知返回类型 %T", value)
	}
}
