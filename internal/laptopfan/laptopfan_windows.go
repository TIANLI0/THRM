//go:build windows

package laptopfan

import (
	"errors"
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
	read func(session *wmiSession) (FanSpeeds, error)
}

// fanBackends 按探测顺序排列的后端列表。
//
// 联想的两种读法拆成独立后端而非"legacy 失败再试 modern"：探测阶段本来就会
// 逐个尝试并锁定命中的那个，拆开后 modern 机型不必每次采样都先撞一次注定失败的
// legacy 查询。
var fanBackends = []fanBackend{
	{"Uniwill WMI EC", readUniwillFanSpeeds},
	{"ASUS ATK WMI", readAsusFanSpeeds},
	{"Lenovo Legion WMI", readLenovoFanSpeedsLegacy},
	{"Lenovo Legion WMI (modern)", readLenovoFanSpeedsModern},
}

type windowsReader struct {
	logger types.Logger

	mutex       sync.Mutex
	worker      *comWorker
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
	if r.worker == nil {
		r.worker = newCOMWorker()
	}

	// 已锁定后端：只用它读取，连续失败则标记不支持。
	if r.backendIdx >= 0 {
		backend := fanBackends[r.backendIdx]
		speeds, err := r.runBackend(backend)
		if err != nil {
			r.failures++
			if r.failures >= maxConsecutiveFailures {
				r.disableLocked("笔记本风扇转速读取已停用（%s）: %v", backend.name, err)
			}
			return FanSpeeds{}, false
		}
		r.failures = 0
		return speeds, true
	}

	// 探测阶段：依次尝试所有后端，命中即锁定。
	var lastErr error
	for i, backend := range fanBackends {
		speeds, err := r.runBackend(backend)
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
		r.disableLocked("笔记本风扇转速读取不可用（本机无受支持的 WMI 接口）: %v", lastErr)
	}
	return FanSpeeds{}, false
}

// runBackend 在 COM 工作线程上执行一次后端读取。
func (r *windowsReader) runBackend(backend fanBackend) (FanSpeeds, error) {
	var speeds FanSpeeds
	err := r.worker.do(func(session *wmiSession) error {
		result, err := backend.read(session)
		if err != nil {
			return err
		}
		speeds = result
		return nil
	})
	return speeds, err
}

// disableLocked 永久停用读取，并释放常驻的 WMI 连接与 COM 线程。
// 不支持的机型占绝大多数，及时收回这些资源才不会白白常驻一个线程和一条 WMI 连接。
func (r *windowsReader) disableLocked(format string, args ...any) {
	r.unsupported = true
	if r.worker != nil {
		r.worker.close()
		r.worker = nil
	}
	if r.logger != nil {
		r.logger.Info(format, args...)
	}
}

// errWMIClassUnavailable 标记"本机没有这个 WMI 类/方法"。这是机型探测的正常
// 结果而非连接故障，不应据此丢弃一条仍然可用的 root\WMI 连接。
var errWMIClassUnavailable = errors.New("WMI 类或方法不可用")

// wmiSession 持有一条常驻的 root\WMI 连接，以及按"类.方法"缓存的方法调用器。
//
// Why: 旧实现每次采样都要 CoInitialize → 创建 SWbemLocator → ConnectServer →
// ExecQuery 取实例 → Get 类定义 → 取 Methods_ / InParameters，一次读数就是八次
// 跨进程 WMI 往返，还会把 WmiPrvSE.exe 反复唤醒；而温度采样每 1~10 秒就调用一次。
// 这些握手结果在进程生命周期内是稳定的，缓存之后每次采样只剩真正的 ExecMethod。
type wmiSession struct {
	service *ole.IDispatch
	callers map[string]*wmiMethodCaller
}

// openSession 建立会话，测试替换它以便在没有 WMI 的环境下验证会话生命周期。
var openSession = newWMISession

func newWMISession() (*wmiSession, error) {
	locatorObj, err := oleutil.CreateObject("WbemScripting.SWbemLocator")
	if err != nil {
		return nil, fmt.Errorf("创建 SWbemLocator 失败: %w", err)
	}
	defer locatorObj.Release()

	locator, err := locatorObj.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return nil, fmt.Errorf("SWbemLocator IDispatch: %w", err)
	}
	defer locator.Release()

	serviceRaw, err := oleutil.CallMethod(locator, "ConnectServer", ".", `root\WMI`)
	if err != nil {
		return nil, fmt.Errorf("连接 root\\WMI 失败: %w", err)
	}

	return &wmiSession{
		service: serviceRaw.ToIDispatch(),
		callers: make(map[string]*wmiMethodCaller, len(fanBackends)),
	}, nil
}

// caller 返回绑定到 className.methodName 的调用器，首次使用时创建并缓存。
func (s *wmiSession) caller(className, methodName string) (*wmiMethodCaller, error) {
	key := className + "." + methodName
	if cached, ok := s.callers[key]; ok {
		return cached, nil
	}
	created, err := newWMIMethodCaller(s.service, className, methodName)
	if err != nil {
		return nil, err
	}
	s.callers[key] = created
	return created, nil
}

func (s *wmiSession) close() {
	for _, caller := range s.callers {
		caller.release()
	}
	s.callers = nil
	if s.service != nil {
		s.service.Release()
		s.service = nil
	}
}

// comTask 是投递给 COM 工作线程的一次调用。
type comTask struct {
	fn   func(session *wmiSession) error
	done chan error
}

// comWorker 把所有 COM/WMI 调用固定在同一个独占 OS 线程上执行。
// COM 公寓与接口指针都绑定线程，只有这样 CoInitializeEx 的结果与缓存的
// SWbemServices/方法句柄才能跨调用复用。
type comWorker struct {
	tasks  chan comTask
	closed bool
}

func newCOMWorker() *comWorker {
	worker := &comWorker{tasks: make(chan comTask)}
	go worker.loop()
	return worker
}

// do 同步执行一次会话调用。调用方（windowsReader.read）已持锁串行化，
// 因此这里不需要额外同步。
func (w *comWorker) do(fn func(session *wmiSession) error) error {
	if w.closed {
		return errors.New("COM 工作线程已关闭")
	}
	done := make(chan error, 1)
	w.tasks <- comTask{fn: fn, done: done}
	return <-done
}

func (w *comWorker) close() {
	if w.closed {
		return
	}
	w.closed = true
	close(w.tasks)
}

func (w *comWorker) loop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	uninit, err := coInitialize()
	if err != nil {
		for task := range w.tasks {
			task.done <- err
		}
		return
	}
	defer uninit()

	var session *wmiSession
	defer func() {
		if session != nil {
			session.close()
		}
	}()

	for task := range w.tasks {
		if session == nil {
			created, createErr := openSession()
			if createErr != nil {
				task.done <- createErr
				continue
			}
			session = created
		}

		taskErr := task.fn(session)
		// 连接与缓存的方法句柄可能因 WMI 服务重启、休眠唤醒而失效，丢弃后下次
		// 调用重建——这保留了旧实现"每次重连"自带的自愈能力。类/方法本就不存在
		// （机型探测的正常结果）不算故障，连接继续复用。
		if taskErr != nil && !errors.Is(taskErr, errWMIClassUnavailable) {
			session.close()
			session = nil
		}
		task.done <- taskErr
	}
}

// coInitialize 在当前线程初始化 COM，返回的函数在需要时释放。
func coInitialize() (func(), error) {
	err := ole.CoInitializeEx(0, ole.COINIT_MULTITHREADED)
	if err == nil {
		return ole.CoUninitialize, nil
	}

	oleErr, ok := err.(*ole.OleError)
	// S_FALSE / RPC_E_CHANGED_MODE：线程已初始化，可继续使用，但不由我们释放。
	if ok && (oleErr.Code() == 0x00000001 || oleErr.Code() == 0x80010106) {
		return func() {}, nil
	}
	return nil, fmt.Errorf("CoInitializeEx: %w", err)
}

// wmiMethodCaller 绑定某 WMI 类首个实例的一个方法，可多次以不同参数调用。
type wmiMethodCaller struct {
	service *ole.IDispatch
	relPath string
	method  string
	inDef   *ole.IDispatch
}

// newWMIMethodCaller 查询 className 的首个实例，并准备 methodName 的输入参数定义。
//
// 全部失败都归类为 errWMIClassUnavailable：这条路径只在首次绑定某个类时执行，
// 而首次绑定要么发生在刚建立的连接上（探测阶段），要么发生在已被证明可用的连接上，
// 因此"类不存在"是唯一现实的失败原因。连接真正失效会在后续 ExecMethod 上暴露，
// 那里不带这个标记，会正常触发重建。
func newWMIMethodCaller(service *ole.IDispatch, className, methodName string) (*wmiMethodCaller, error) {
	resultRaw, err := oleutil.CallMethod(service, "ExecQuery", "SELECT * FROM "+className)
	if err != nil {
		return nil, fmt.Errorf("查询 %s 失败: %v: %w", className, err, errWMIClassUnavailable)
	}
	resultSet := resultRaw.ToIDispatch()
	defer resultSet.Release()

	itemRaw, err := oleutil.CallMethod(resultSet, "ItemIndex", 0)
	if err != nil {
		return nil, fmt.Errorf("%s 无实例: %v: %w", className, err, errWMIClassUnavailable)
	}
	item := itemRaw.ToIDispatch()
	defer item.Release()

	pathRaw, err := oleutil.GetProperty(item, "Path_")
	if err != nil {
		return nil, fmt.Errorf("读取实例 Path_ 失败: %v: %w", err, errWMIClassUnavailable)
	}
	pathObj := pathRaw.ToIDispatch()
	relPathRaw, err := oleutil.GetProperty(pathObj, "RelPath")
	pathObj.Release()
	if err != nil {
		return nil, fmt.Errorf("读取实例 RelPath 失败: %v: %w", err, errWMIClassUnavailable)
	}
	// RelPath 是 BSTR VARIANT，取出字符串后必须 Clear，否则每次绑定都会漏一个 BSTR。
	relPath := relPathRaw.ToString()
	_ = relPathRaw.Clear()

	classRaw, err := oleutil.CallMethod(service, "Get", className)
	if err != nil {
		return nil, fmt.Errorf("获取 %s 类定义失败: %v: %w", className, err, errWMIClassUnavailable)
	}
	class := classRaw.ToIDispatch()
	defer class.Release()

	methodsRaw, err := oleutil.GetProperty(class, "Methods_")
	if err != nil {
		return nil, fmt.Errorf("读取 Methods_ 失败: %v: %w", err, errWMIClassUnavailable)
	}
	methods := methodsRaw.ToIDispatch()
	defer methods.Release()

	methodRaw, err := oleutil.CallMethod(methods, "Item", methodName)
	if err != nil {
		return nil, fmt.Errorf("%s 未提供 %s 方法: %v: %w", className, methodName, err, errWMIClassUnavailable)
	}
	method := methodRaw.ToIDispatch()
	defer method.Release()

	inDefRaw, err := oleutil.GetProperty(method, "InParameters")
	if err != nil {
		return nil, fmt.Errorf("读取 InParameters 失败: %v: %w", err, errWMIClassUnavailable)
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
		c.inDef = nil
	}
}

// call 以单个输入参数执行方法，返回输出对象上 outProp 属性的 uint32 值。
// 所有后端都只需要传一个参数，直接收标量而不是 map 可以省掉每次采样的映射分配。
func (c *wmiMethodCaller) call(paramName string, paramValue any, outProp string) (uint32, error) {
	inRaw, err := oleutil.CallMethod(c.inDef, "SpawnInstance_")
	if err != nil {
		return 0, fmt.Errorf("SpawnInstance_ 失败: %w", err)
	}
	in := inRaw.ToIDispatch()
	defer in.Release()

	if _, err := oleutil.PutProperty(in, paramName, paramValue); err != nil {
		return 0, fmt.Errorf("设置 %s 参数失败: %w", paramName, err)
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
