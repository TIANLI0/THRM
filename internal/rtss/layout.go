package rtss

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	anchorLayerName      = "THRM Anchor"
	anchorStateConfirmed = "confirmed"
	anchorStateCandidate = "candidate"
	anchorStateNeedsLast = "needs_last"
	anchorStateMissing   = "missing"
)

// LayoutStatus describes the active OverlayEditor layout without changing it.
// The path is intentionally exposed so the UI can tell users which layout was
// inspected before offering a write operation.
type LayoutStatus struct {
	Supported   bool   `json:"supported"`
	Installed   bool   `json:"installed"`
	InstallPath string `json:"installPath"`
	ConfigPath  string `json:"configPath"`
	LayoutPath  string `json:"layoutPath"`
	LayoutName  string `json:"layoutName"`
	BackupPath  string `json:"backupPath"`
	AnchorState string `json:"anchorState"`
	AnchorIndex int    `json:"anchorIndex"`
	LayerCount  int    `json:"layerCount"`
}

type overlayLayer struct {
	index     int
	name      string
	text      string
	size      int
	positionX int
	positionY int
	startLine int
	endLine   int
}

type layoutInspection struct {
	layers         []overlayLayer
	declaredLayers int
	layersLine     int
}

func parseOverlayLayout(data []byte) layoutInspection {
	lines := strings.Split(string(data), "\n")
	inspection := layoutInspection{layersLine: -1}
	current := -1
	section := ""
	for lineIndex, raw := range lines {
		line := strings.TrimSpace(strings.TrimSuffix(raw, "\r"))
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			if current >= 0 {
				inspection.layers[current].endLine = lineIndex
			}
			section = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")
			if strings.HasPrefix(strings.ToLower(section), "layer") {
				if index, err := strconv.Atoi(section[len("Layer"):]); err == nil && index >= 0 {
					current = len(inspection.layers)
					inspection.layers = append(inspection.layers, overlayLayer{index: index, size: 100, startLine: lineIndex})
					continue
				}
			}
			current = -1
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = unquote(strings.TrimSpace(value))
		if strings.EqualFold(section, "General") && current < 0 && strings.EqualFold(key, "Layers") {
			if count, err := strconv.Atoi(value); err == nil && count >= 0 {
				inspection.declaredLayers = count
				inspection.layersLine = lineIndex
			}
		}
		if current < 0 || current >= len(inspection.layers) {
			continue
		}
		switch {
		case strings.EqualFold(key, "Name"):
			inspection.layers[current].name = value
		case strings.EqualFold(key, "Text"):
			inspection.layers[current].text = value
		case strings.EqualFold(key, "Size"):
			if size, err := strconv.Atoi(value); err == nil {
				inspection.layers[current].size = size
			}
		case strings.EqualFold(key, "PositionX"):
			if position, err := strconv.Atoi(value); err == nil {
				inspection.layers[current].positionX = position
			}
		case strings.EqualFold(key, "PositionY"):
			if position, err := strconv.Atoi(value); err == nil {
				inspection.layers[current].positionY = position
			}
		}
	}
	if current >= 0 {
		inspection.layers[current].endLine = len(lines)
	}
	return inspection
}

func unquote(value string) string {
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		return value[1 : len(value)-1]
	}
	return value
}

func inspectOverlayLayout(data []byte) (state string, anchorIndex, layerCount int) {
	inspection := parseOverlayLayout(data)
	layers := append([]overlayLayer(nil), inspection.layers...)
	sort.SliceStable(layers, func(i, j int) bool { return layers[i].index < layers[j].index })
	layerCount = inspection.declaredLayers
	if layerCount < len(layers) {
		layerCount = len(layers)
	}
	anchorIndex = -1
	candidateIndex := -1
	for i, layer := range layers {
		if layer.size != 1 || strings.TrimSpace(layer.text) != "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(layer.name), anchorLayerName) {
			anchorIndex = i
			break
		}
		if candidateIndex < 0 {
			candidateIndex = i
		}
	}
	if anchorIndex >= 0 {
		if anchorIndex == len(layers)-1 && (layerCount == 0 || anchorIndex == layerCount-1) {
			return anchorStateConfirmed, anchorIndex, layerCount
		}
		return anchorStateNeedsLast, anchorIndex, layerCount
	}
	if candidateIndex >= 0 {
		if candidateIndex == len(layers)-1 && (layerCount == 0 || candidateIndex == layerCount-1) {
			return anchorStateCandidate, candidateIndex, layerCount
		}
		return anchorStateNeedsLast, candidateIndex, layerCount
	}
	return anchorStateMissing, -1, layerCount
}

func appendAnchor(data []byte) ([]byte, error) {
	inspection := parseOverlayLayout(data)
	if inspection.layersLine < 0 {
		return nil, fmt.Errorf("RTSS 布局中未找到有效的 [General] Layers 配置")
	}
	maxIndex := -1
	for _, layer := range inspection.layers {
		if layer.index > maxIndex {
			maxIndex = layer.index
		}
	}
	newIndex := maxIndex + 1
	if inspection.declaredLayers > newIndex {
		newIndex = inspection.declaredLayers
	}
	positionX, positionY := defaultAnchorPosition(inspection.layers)
	lines := strings.Split(string(data), "\n")
	newline := "\n"
	if strings.Contains(string(data), "\r\n") {
		newline = "\r\n"
	}
	for i, raw := range lines {
		if i == inspection.layersLine {
			prefix := raw[:len(raw)-len(strings.TrimLeft(raw, " \t"))]
			ending := ""
			if strings.HasSuffix(raw, "\r") {
				ending = "\r"
			}
			lines[i] = prefix + "Layers=" + strconv.Itoa(newIndex+1) + ending
		}
	}
	block := []string{
		"[Layer" + strconv.Itoa(newIndex) + "]",
		"Name=" + anchorLayerName,
		"Text=",
		"PositionX=" + strconv.Itoa(positionX),
		"PositionY=" + strconv.Itoa(positionY),
		"ExtentX=0",
		"ExtentY=0",
		"ExtentOrigin=0",
		"Size=1",
	}
	result := strings.Join(lines, "\n")
	result = strings.TrimRight(result, "\r\n") + newline + strings.Join(block, newline) + newline
	return []byte(result), nil
}

// defaultAnchorPosition puts a newly created anchor one text row below the
// lowest existing text baseline. RTSS layouts commonly use negative Y values
// for rows below the top line, so "below" means one less than the minimum Y.
// Keeping the most common X avoids placing a vertical layout's anchor in a
// side column when one diagnostic item has a custom horizontal offset.
func defaultAnchorPosition(layers []overlayLayer) (int, int) {
	if len(layers) == 0 {
		return 0, 1
	}
	xCounts := make(map[int]int)
	minY := 0
	hasText := false
	for _, layer := range layers {
		if strings.TrimSpace(layer.text) == "" {
			continue
		}
		if !hasText || layer.positionY < minY {
			minY = layer.positionY
		}
		hasText = true
		xCounts[layer.positionX]++
	}
	if !hasText {
		return 0, 1
	}
	positionX := 0
	maxCount := 0
	for x, count := range xCounts {
		if count > maxCount ||
			(count == maxCount && (absInt(x) < absInt(positionX) ||
				(absInt(x) == absInt(positionX) && x < positionX))) {
			positionX = x
			maxCount = count
		}
	}
	return positionX, minY - 1
}

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func configureAnchor(data []byte) ([]byte, error) {
	inspection := parseOverlayLayout(data)
	exact := make([]int, 0, 1)
	candidates := make([]int, 0, 1)
	for i, layer := range inspection.layers {
		if layer.size != 1 || strings.TrimSpace(layer.text) != "" {
			continue
		}
		candidates = append(candidates, i)
		if strings.EqualFold(strings.TrimSpace(layer.name), anchorLayerName) {
			exact = append(exact, i)
		}
	}
	if len(exact) > 1 || (len(exact) == 0 && len(candidates) > 1) {
		return nil, fmt.Errorf("发现多个 1%% 空文字图层，无法安全判断定位图层")
	}
	if len(exact) == 1 {
		return markAndMoveAnchor(data, inspection, exact[0])
	}
	if len(candidates) == 1 {
		return markAndMoveAnchor(data, inspection, candidates[0])
	}
	return appendAnchor(data)
}

func markAndMoveAnchor(data []byte, inspection layoutInspection, target int) ([]byte, error) {
	if target < 0 || target >= len(inspection.layers) {
		return nil, fmt.Errorf("定位图层索引无效")
	}
	lines := strings.Split(string(data), "\n")
	type layerSection struct {
		index  int
		target bool
		lines  []string
	}
	sections := make([]layerSection, 0, len(inspection.layers))
	firstLine := len(lines)
	lastLine := 0
	for i, layer := range inspection.layers {
		if layer.startLine < 0 || layer.endLine <= layer.startLine || layer.endLine > len(lines) {
			return nil, fmt.Errorf("定位图层边界无效")
		}
		if layer.startLine < firstLine {
			firstLine = layer.startLine
		}
		if layer.endLine > lastLine {
			lastLine = layer.endLine
		}
		sections = append(sections, layerSection{
			index:  layer.index,
			target: i == target,
			lines:  append([]string(nil), lines[layer.startLine:layer.endLine]...),
		})
	}
	for i := 1; i < len(inspection.layers); i++ {
		if inspection.layers[i-1].endLine != inspection.layers[i].startLine {
			return nil, fmt.Errorf("RTSS 布局的图层之间包含其他段落，无法安全重排")
		}
	}
	sort.SliceStable(sections, func(i, j int) bool { return sections[i].index < sections[j].index })
	for i := 1; i < len(sections); i++ {
		if sections[i-1].index == sections[i].index {
			return nil, fmt.Errorf("RTSS 布局包含重复的 Layer%d", sections[i].index)
		}
	}
	targetPosition := -1
	for i := range sections {
		if sections[i].target {
			targetPosition = i
			break
		}
	}
	if targetPosition < 0 {
		return nil, fmt.Errorf("未找到定位图层内容")
	}
	targetSection := sections[targetPosition]
	nameUpdated := false
	for i := 1; i < len(targetSection.lines); i++ {
		line := strings.TrimSuffix(targetSection.lines[i], "\r")
		key, _, ok := strings.Cut(line, "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(key), "Name") {
			continue
		}
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		ending := ""
		if strings.HasSuffix(targetSection.lines[i], "\r") {
			ending = "\r"
		}
		targetSection.lines[i] = indent + "Name=" + anchorLayerName + ending
		nameUpdated = true
		break
	}
	if !nameUpdated {
		ending := ""
		if len(targetSection.lines) > 0 && strings.HasSuffix(targetSection.lines[0], "\r") {
			ending = "\r"
		}
		targetSection.lines = append(targetSection.lines[:1], append([]string{"Name=" + anchorLayerName + ending}, targetSection.lines[1:]...)...)
	}
	sections = append(append(sections[:targetPosition], sections[targetPosition+1:]...), targetSection)
	for i := range sections {
		ending := ""
		if strings.HasSuffix(sections[i].lines[0], "\r") {
			ending = "\r"
		}
		sections[i].lines[0] = "[Layer" + strconv.Itoa(i) + "]" + ending
	}
	updated := append([]string(nil), lines[:firstLine]...)
	for _, section := range sections {
		updated = append(updated, section.lines...)
	}
	updated = append(updated, lines[lastLine:]...)
	return []byte(strings.Join(updated, "\n")), nil
}
