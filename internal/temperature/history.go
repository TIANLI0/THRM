package temperature

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/TIANLI0/THRM/internal/types"
)

const (
	DefaultHistoryCapacity              = 720
	DefaultHistorySampleInterval        = 5 * time.Second
	DefaultHistoryRetentionHours        = types.DefaultTemperatureHistoryRetentionHours
	MaxHistoryRetentionHours            = types.MaxTemperatureHistoryRetentionHours
	DefaultHistoryRelativePath          = "telemetry/history.bin"
	historyBinaryMagic                  = "THST"
	historyBinaryVersionLegacy   uint16 = 1
	historyBinaryVersionPower    uint16 = 2
	historyBinaryVersionFans     uint16 = 3 // v3: 追加笔记本内置 CPU/GPU 风扇转速
	historyBinaryVersion         uint16 = 4 // v4: 追加时间轴事件
	historyEnabledFlag           uint8  = 1

	dirtyFlushThreshold = 6
	dirtyFlushInterval  = 30 * time.Second

	// 事件比采样点稀疏得多，固定上限即可覆盖最长 24 小时保留窗口；
	// 超出后按时间从最旧的开始丢弃。
	timelineEventCapacity = 240
	// 同类事件在此窗口内重复到达视为同一次（例如 Windows 唤醒会同时发出
	// PBT_APMRESUMESUSPEND 与 PBT_APMRESUMEAUTOMATIC 两个通知）。
	timelineEventDedupeWindow = 3 * time.Second
)

type HistoryRecorder struct {
	mutex          sync.RWMutex
	logger         types.Logger
	filePath       string
	enabled        bool
	capacity       int
	sampleInterval time.Duration
	points         []types.TemperatureHistoryPoint
	next           int
	filled         bool
	lastSampleAt   int64
	events         []types.TimelineEvent // 按时间升序，与 points 共享保留窗口
	lastEventType  string                // 去重用，独立于 events：关闭记录时也要折叠重复通知
	lastEventKey   string
	lastEventAt    int64

	dirtyCount  int
	lastFlushAt time.Time
	flushMutex  sync.Mutex // 串行化磁盘写入，与 mutex 互不持有
}

func NewHistoryRecorder(filePath string, capacity int, sampleInterval time.Duration, logger types.Logger) *HistoryRecorder {
	if capacity <= 0 {
		capacity = DefaultHistoryCapacity
	}
	if sampleInterval <= 0 {
		sampleInterval = DefaultHistorySampleInterval
	}

	recorder := &HistoryRecorder{
		logger:         logger,
		filePath:       filePath,
		capacity:       capacity,
		sampleInterval: sampleInterval,
		enabled:        true,
		points:         make([]types.TemperatureHistoryPoint, capacity),
	}
	recorder.load()
	return recorder
}

func (r *HistoryRecorder) SetEnabled(enabled bool) error {
	r.mutex.Lock()
	r.enabled = enabled
	if !enabled {
		r.clearLocked()
	}
	payload, err := r.serializeLocked()
	r.mutex.Unlock()
	if err != nil {
		return err
	}
	return r.writeFile(payload)
}

func (r *HistoryRecorder) IsEnabled() bool {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.enabled
}

// pointsPerHourLocked 返回按当前采样间隔一小时可容纳的样本数。
func (r *HistoryRecorder) pointsPerHourLocked() int {
	if r.sampleInterval <= 0 {
		return DefaultHistoryCapacity
	}
	per := int(time.Hour / r.sampleInterval)
	if per <= 0 {
		return DefaultHistoryCapacity
	}
	return per
}

// RetentionHours 返回当前后台保留时长(小时)。
func (r *HistoryRecorder) RetentionHours() int {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	hours := r.capacity / r.pointsPerHourLocked()
	if hours < 1 {
		hours = 1
	}
	return hours
}

// SetRetentionHours 调整后台保留时长；扩缩环形缓冲区并保留最新样本后落盘。
func (r *HistoryRecorder) SetRetentionHours(hours int) error {
	if hours < 1 {
		hours = 1
	}
	if hours > MaxHistoryRetentionHours {
		hours = MaxHistoryRetentionHours
	}

	r.mutex.Lock()
	newCapacity := r.pointsPerHourLocked() * hours
	if newCapacity <= 0 || newCapacity == r.capacity {
		r.mutex.Unlock()
		return nil
	}
	ordered := r.snapshotPointsLocked()
	r.capacity = newCapacity
	r.points = make([]types.TemperatureHistoryPoint, newCapacity)
	r.next = 0
	r.filled = false
	r.applyLoadedPointsLocked(ordered)
	// 保留窗口变了，事件的过期界限跟着变（缩短时要立刻丢弃窗口外的旧事件）。
	r.pruneEventsLocked(0)
	payload, err := r.serializeLocked()
	r.mutex.Unlock()
	if err != nil {
		return err
	}
	return r.writeFile(payload)
}

func (r *HistoryRecorder) Flush() error {
	r.mutex.Lock()
	if r.dirtyCount == 0 {
		r.mutex.Unlock()
		return nil
	}
	payload, err := r.serializeLocked()
	r.dirtyCount = 0
	r.lastFlushAt = time.Now()
	r.mutex.Unlock()
	if err != nil {
		return err
	}
	return r.writeFile(payload)
}

func (r *HistoryRecorder) Add(temp types.TemperatureData, fanData *types.FanData) (types.TemperatureHistoryPoint, bool) {
	if temp.CPUTemp <= 0 && temp.GPUTemp <= 0 {
		return types.TemperatureHistoryPoint{}, false
	}

	timestamp := normalizeTimestampMillis(temp.UpdateTime)
	if timestamp <= 0 {
		timestamp = time.Now().UnixMilli()
	}

	fanRPM := 0
	if fanData != nil {
		fanRPM = int(fanData.CurrentRPM)
	}

	point := types.TemperatureHistoryPoint{
		Timestamp: timestamp,
		CPUTemp:   temp.CPUTemp,
		GPUTemp:   temp.GPUTemp,
		CPUPower:  normalizeHistoryPower(temp.CPUPower),
		GPUPower:  normalizeHistoryPower(temp.GPUPower),
		FanRPM:    fanRPM,
		CPUFanRPM: temp.CPUFanRPM,
		GPUFanRPM: temp.GPUFanRPM,
	}

	var flushPayload []byte

	r.mutex.Lock()
	if !r.enabled {
		r.mutex.Unlock()
		return types.TemperatureHistoryPoint{}, false
	}
	if r.lastSampleAt > 0 && timestamp-r.lastSampleAt < r.sampleInterval.Milliseconds() {
		r.mutex.Unlock()
		return types.TemperatureHistoryPoint{}, false
	}

	r.points[r.next] = point
	r.lastSampleAt = timestamp
	r.next = (r.next + 1) % r.capacity
	if r.next == 0 {
		r.filled = true
	}

	r.dirtyCount++
	now := time.Now()
	if r.dirtyCount >= dirtyFlushThreshold || now.Sub(r.lastFlushAt) >= dirtyFlushInterval {
		if payload, err := r.serializeLocked(); err == nil {
			flushPayload = payload
			r.dirtyCount = 0
			r.lastFlushAt = now
		} else {
			r.logError("序列化温度历史失败: %v", err)
		}
	}
	r.mutex.Unlock()

	if flushPayload != nil {
		if err := r.writeFile(flushPayload); err != nil {
			r.logError("保存温度历史失败: %v", err)
		}
	}
	return point, true
}

// AddEvent 记录一次时间轴事件。返回落库后的事件与是否真的记录了。
//
// 记录在核心侧而非 GUI 侧：GUI 多数时间是关闭的，只在前端内存里攒事件意味着
// "关着界面时断的线"永远看不到——温度曲线明明有一小时，标记却只有开界面之后的
// 那几分钟。
func (r *HistoryRecorder) AddEvent(eventType, labelKey string) (types.TimelineEvent, bool) {
	if eventType == "" || labelKey == "" {
		return types.TimelineEvent{}, false
	}

	event := types.TimelineEvent{
		Timestamp: time.Now().UnixMilli(),
		Type:      eventType,
		LabelKey:  labelKey,
	}

	var flushPayload []byte

	r.mutex.Lock()
	// 去重状态独立于 events 维护：关闭后台记录时事件不入缓冲，但重复通知
	// （例如唤醒时 Windows 连发的两个电源事件）依然要折叠成一次，
	// 否则会向 GUI 广播出两个重叠的标记。
	if r.lastEventType == event.Type && r.lastEventKey == event.LabelKey &&
		r.lastEventAt > 0 && event.Timestamp-r.lastEventAt < timelineEventDedupeWindow.Milliseconds() {
		r.mutex.Unlock()
		return types.TimelineEvent{}, false
	}
	r.lastEventType = event.Type
	r.lastEventKey = event.LabelKey
	r.lastEventAt = event.Timestamp

	if r.enabled {
		r.events = append(r.events, event)
		r.pruneEventsLocked(event.Timestamp)

		r.dirtyCount++
		now := time.Now()
		if r.dirtyCount >= dirtyFlushThreshold || now.Sub(r.lastFlushAt) >= dirtyFlushInterval {
			if payload, err := r.serializeLocked(); err == nil {
				flushPayload = payload
				r.dirtyCount = 0
				r.lastFlushAt = now
			} else {
				r.logError("序列化温度历史失败: %v", err)
			}
		}
	}
	r.mutex.Unlock()

	if flushPayload != nil {
		if err := r.writeFile(flushPayload); err != nil {
			r.logError("保存温度历史失败: %v", err)
		}
	}
	return event, true
}

func compareEventTime(a, b types.TimelineEvent) int {
	switch {
	case a.Timestamp < b.Timestamp:
		return -1
	case a.Timestamp > b.Timestamp:
		return 1
	default:
		return 0
	}
}

// retentionMillisLocked 返回采样点覆盖的时间跨度，事件按同一窗口过期，
// 避免图表里出现早于最旧曲线数据、无处可标的孤立标记。
func (r *HistoryRecorder) retentionMillisLocked() int64 {
	if r.sampleInterval <= 0 || r.capacity <= 0 {
		return int64(DefaultHistoryRetentionHours) * time.Hour.Milliseconds()
	}
	return int64(r.capacity) * r.sampleInterval.Milliseconds()
}

// pruneEventsLocked 按保留窗口与容量上限裁剪事件，并保证按时间升序。
func (r *HistoryRecorder) pruneEventsLocked(newestTimestamp int64) {
	if len(r.events) == 0 {
		return
	}
	if !slices.IsSortedFunc(r.events, compareEventTime) {
		slices.SortStableFunc(r.events, compareEventTime)
	}

	if newestTimestamp <= 0 {
		newestTimestamp = r.events[len(r.events)-1].Timestamp
	}
	cutoff := newestTimestamp - r.retentionMillisLocked()
	start := 0
	for start < len(r.events) && r.events[start].Timestamp < cutoff {
		start++
	}
	if len(r.events)-start > timelineEventCapacity {
		start = len(r.events) - timelineEventCapacity
	}
	if start > 0 {
		r.events = append(r.events[:0], r.events[start:]...)
	}
}

func (r *HistoryRecorder) snapshotEventsLocked() []types.TimelineEvent {
	if len(r.events) == 0 {
		return nil
	}
	return slices.Clone(r.events)
}

func (r *HistoryRecorder) Snapshot() types.TemperatureHistoryPayload {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	retentionHours := r.capacity / r.pointsPerHourLocked()
	if retentionHours < 1 {
		retentionHours = 1
	}
	return types.TemperatureHistoryPayload{
		Enabled:               r.enabled,
		SampleIntervalSeconds: int(r.sampleInterval / time.Second),
		RetentionHours:        retentionHours,
		Points:                r.snapshotPointsLocked(),
		Events:                r.snapshotEventsLocked(),
	}
}

func normalizeTimestampMillis(timestamp int64) int64 {
	if timestamp <= 0 {
		return 0
	}
	if timestamp < 1_000_000_000_000 {
		return timestamp * 1000
	}
	return timestamp
}

func normalizeHistoryPower(watts float64) float64 {
	if math.IsNaN(watts) || math.IsInf(watts, 0) || watts <= 0 || watts > 2000 {
		return 0
	}
	return float64(math.Round(watts*10)) / 10
}

func (r *HistoryRecorder) load() {
	if r.filePath == "" {
		return
	}

	if err := r.loadBinaryFile(r.filePath); err == nil {
		return
	} else if !os.IsNotExist(err) {
		r.logError("解析温度历史文件失败: %v", err)
	}
}

func (r *HistoryRecorder) loadBinaryFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return r.loadBinaryData(data)
}

func (r *HistoryRecorder) loadBinaryData(data []byte) error {
	reader := bytes.NewReader(data)
	magic := make([]byte, len(historyBinaryMagic))
	if _, err := io.ReadFull(reader, magic); err != nil {
		return err
	}
	if string(magic) != historyBinaryMagic {
		return io.ErrUnexpectedEOF
	}

	var version uint16
	if err := binary.Read(reader, binary.LittleEndian, &version); err != nil {
		return err
	}
	if version < historyBinaryVersionLegacy || version > historyBinaryVersion {
		return fmt.Errorf("unsupported history version: %d", version)
	}

	var flags uint8
	var reserved uint8
	var sampleIntervalSeconds uint32
	var count uint32
	var updatedAt int64
	if err := binary.Read(reader, binary.LittleEndian, &flags); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &reserved); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &sampleIntervalSeconds); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &count); err != nil {
		return err
	}
	if err := binary.Read(reader, binary.LittleEndian, &updatedAt); err != nil {
		return err
	}

	points := make([]types.TemperatureHistoryPoint, 0, count)
	for i := uint32(0); i < count; i++ {
		var timestamp int64
		var cpuTemp int32
		var gpuTemp int32
		var fanRPM int32
		var cpuPower float32
		var gpuPower float32
		if err := binary.Read(reader, binary.LittleEndian, &timestamp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &cpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &gpuTemp); err != nil {
			return err
		}
		if err := binary.Read(reader, binary.LittleEndian, &fanRPM); err != nil {
			return err
		}
		var cpuFanRPM int32
		var gpuFanRPM int32
		if version >= historyBinaryVersionPower {
			if err := binary.Read(reader, binary.LittleEndian, &cpuPower); err != nil {
				return err
			}
			if err := binary.Read(reader, binary.LittleEndian, &gpuPower); err != nil {
				return err
			}
		}
		if version >= historyBinaryVersionFans {
			if err := binary.Read(reader, binary.LittleEndian, &cpuFanRPM); err != nil {
				return err
			}
			if err := binary.Read(reader, binary.LittleEndian, &gpuFanRPM); err != nil {
				return err
			}
		}
		points = append(points, types.TemperatureHistoryPoint{
			Timestamp: normalizeTimestampMillis(timestamp),
			CPUTemp:   int(cpuTemp),
			GPUTemp:   int(gpuTemp),
			CPUPower:  normalizeHistoryPower(float64(cpuPower)),
			GPUPower:  normalizeHistoryPower(float64(gpuPower)),
			FanRPM:    int(fanRPM),
			CPUFanRPM: int(cpuFanRPM),
			GPUFanRPM: int(gpuFanRPM),
		})
	}

	// v4 起在采样点之后追加时间轴事件。旧版本文件读到这里就结束，事件为空。
	var events []types.TimelineEvent
	if version >= historyBinaryVersion {
		var eventCount uint32
		if err := binary.Read(reader, binary.LittleEndian, &eventCount); err != nil {
			return err
		}
		if eventCount > timelineEventCapacity {
			return fmt.Errorf("history event count out of range: %d", eventCount)
		}
		events = make([]types.TimelineEvent, 0, eventCount)
		for i := uint32(0); i < eventCount; i++ {
			var timestamp int64
			if err := binary.Read(reader, binary.LittleEndian, &timestamp); err != nil {
				return err
			}
			eventType, err := readHistoryString(reader)
			if err != nil {
				return err
			}
			labelKey, err := readHistoryString(reader)
			if err != nil {
				return err
			}
			if eventType == "" || labelKey == "" {
				continue
			}
			events = append(events, types.TimelineEvent{
				Timestamp: normalizeTimestampMillis(timestamp),
				Type:      eventType,
				LabelKey:  labelKey,
			})
		}
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	r.enabled = flags&historyEnabledFlag != 0
	if sampleIntervalSeconds > 0 {
		r.sampleInterval = time.Duration(sampleIntervalSeconds) * time.Second
	}
	r.applyLoadedPointsLocked(points)
	r.events = events
	r.pruneEventsLocked(0)
	return nil
}

// readHistoryString 读取 uint16 长度前缀的 UTF-8 字符串。
func readHistoryString(reader io.Reader) (string, error) {
	var length uint16
	if err := binary.Read(reader, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	if length == 0 {
		return "", nil
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func (r *HistoryRecorder) applyLoadedPointsLocked(points []types.TemperatureHistoryPoint) {
	if len(points) > r.capacity {
		points = points[len(points)-r.capacity:]
	}
	for i := range r.points {
		r.points[i] = types.TemperatureHistoryPoint{}
	}
	copy(r.points, points)
	r.next = len(points)
	if r.next >= r.capacity {
		r.next = 0
		r.filled = true
	} else {
		r.filled = len(points) == r.capacity
	}
	if len(points) > 0 {
		r.lastSampleAt = points[len(points)-1].Timestamp
	} else {
		r.lastSampleAt = 0
	}
}

func (r *HistoryRecorder) snapshotPointsLocked() []types.TemperatureHistoryPoint {
	points := make([]types.TemperatureHistoryPoint, 0, r.pointCountLocked())
	if r.filled {
		points = append(points, r.points[r.next:]...)
		points = append(points, r.points[:r.next]...)
	} else {
		points = append(points, r.points[:r.next]...)
	}
	return points
}

func (r *HistoryRecorder) pointCountLocked() int {
	if r.filled {
		return r.capacity
	}
	return r.next
}

func (r *HistoryRecorder) serializeLocked() ([]byte, error) {
	if r.filePath == "" {
		return nil, nil
	}
	pointCount := r.pointCountLocked()
	var flags uint8
	if r.enabled {
		flags |= historyEnabledFlag
	}
	// header 24B + 每点 36B (timestamp, CPU/GPU temperature, fan RPM, CPU/GPU power, 笔记本 CPU/GPU 风扇)
	buf := make([]byte, 0, len(historyBinaryMagic)+24+pointCount*36)
	buf = append(buf, historyBinaryMagic...)
	buf = binary.LittleEndian.AppendUint16(buf, historyBinaryVersion)
	buf = append(buf, flags, 0) // flags + reserved
	buf = binary.LittleEndian.AppendUint32(buf, uint32(r.sampleInterval/time.Second))
	buf = binary.LittleEndian.AppendUint32(buf, uint32(pointCount))
	buf = binary.LittleEndian.AppendUint64(buf, uint64(time.Now().UnixMilli()))
	appendPoint := func(p types.TemperatureHistoryPoint) {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(normalizeTimestampMillis(p.Timestamp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.CPUTemp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.GPUTemp)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.FanRPM)))
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(float32(normalizeHistoryPower(p.CPUPower))))
		buf = binary.LittleEndian.AppendUint32(buf, math.Float32bits(float32(normalizeHistoryPower(p.GPUPower))))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.CPUFanRPM)))
		buf = binary.LittleEndian.AppendUint32(buf, uint32(int32(p.GPUFanRPM)))
	}
	if r.filled {
		for _, p := range r.points[r.next:] {
			appendPoint(p)
		}
		for _, p := range r.points[:r.next] {
			appendPoint(p)
		}
	} else {
		for _, p := range r.points[:r.next] {
			appendPoint(p)
		}
	}

	buf = binary.LittleEndian.AppendUint32(buf, uint32(len(r.events)))
	for _, event := range r.events {
		buf = binary.LittleEndian.AppendUint64(buf, uint64(normalizeTimestampMillis(event.Timestamp)))
		buf = appendHistoryString(buf, event.Type)
		buf = appendHistoryString(buf, event.LabelKey)
	}
	return buf, nil
}

// appendHistoryString 以 uint16 长度前缀写入字符串。
func appendHistoryString(buf []byte, value string) []byte {
	if len(value) > int(^uint16(0)) {
		value = value[:^uint16(0)]
	}
	buf = binary.LittleEndian.AppendUint16(buf, uint16(len(value)))
	return append(buf, value...)
}

// writeFile 在锁外执行磁盘 IO。flushMutex 串行化多次并发 Flush 调用。
func (r *HistoryRecorder) writeFile(payload []byte) error {
	if payload == nil || r.filePath == "" {
		return nil
	}
	r.flushMutex.Lock()
	defer r.flushMutex.Unlock()

	if err := os.MkdirAll(filepath.Dir(r.filePath), 0755); err != nil {
		return err
	}
	tmpPath := r.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, payload, 0644); err != nil {
		return err
	}
	_ = os.Remove(r.filePath)
	if err := os.Rename(tmpPath, r.filePath); err != nil {
		_ = os.Remove(tmpPath)
		return os.WriteFile(r.filePath, payload, 0644)
	}
	return nil
}

func (r *HistoryRecorder) clearLocked() {
	for i := range r.points {
		r.points[i] = types.TemperatureHistoryPoint{}
	}
	r.next = 0
	r.filled = false
	r.lastSampleAt = 0
	r.events = nil
}

func (r *HistoryRecorder) logError(format string, args ...any) {
	if r.logger != nil {
		r.logger.Error(format, args...)
	}
}
