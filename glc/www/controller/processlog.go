package controller

import (
	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/decoder"
	"strings"
	"sync"
)

// 缓存处理过的字符串结果
var (
	resultCache = make(map[string]interface{})
	cacheMutex  sync.RWMutex
)

// OptimizedCleanNestedJSON 入口函数 - 清理嵌套JSON
func OptimizedCleanNestedJSON(jsonStr string) string {
	// 先进行一次完整解析
	var data interface{}
	err := sonic.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return jsonStr // 解析失败返回原始字符串
	}
	// 处理所有嵌套JSON
	modified := processJSON(&data)

	if !modified {
		return jsonStr // 如果没有修改，直接返回原始字符串
	}
	// 重新编码为紧凑JSON
	cleanedBytes, err := sonic.Marshal(data)
	if err != nil {
		return jsonStr
	}
	return string(cleanedBytes)
}

// 递归处理JSON数据结构中所有可能的嵌套JSON字符串
func processJSON(data *interface{}) bool {
	modified := false

	switch v := (*data).(type) {
	case map[string]interface{}:
		// 处理对象/字典
		for key, value := range v {
			// 递归处理值
			valuePtr := &value
			if processJSON(valuePtr) {
				v[key] = *valuePtr
				modified = true
			}
		}

	case []interface{}:
		// 处理数组
		for i := range v {
			if processJSON(&v[i]) {
				modified = true
			}
		}

	case string:
		// 处理字符串 - 检查是否是嵌套JSON
		if len(v) < 2 {
			return false
		}

		// 快速检查是否是可能的JSON
		trimmed := strings.TrimSpace(v)
		if !((trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}') ||
			(trimmed[0] == '[' && trimmed[len(trimmed)-1] == ']')) {
			return false
		}

		// 使用缓存避免重复处理
		cacheMutex.RLock()
		cachedResult, found := resultCache[v]
		cacheMutex.RUnlock()

		if found {
			*data = cachedResult
			return true
		}

		// 尝试解析可能的JSON字符串
		var nestedData interface{}
		streamDecoder := decoder.NewStreamDecoder(strings.NewReader(v))
		if err := streamDecoder.Decode(&nestedData); err == nil {
			// 成功解析，继续递归处理
			nestedDataPtr := &nestedData
			processJSON(nestedDataPtr)

			// 缓存结果
			cacheMutex.Lock()
			resultCache[v] = *nestedDataPtr
			cacheMutex.Unlock()

			*data = *nestedDataPtr
			return true
		}
	}

	return modified
}

// 处理包含转义序列的字符串
func unescapeJSONString(s string) string {
	// 检查是否包含转义的引号
	if strings.Contains(s, "\\\"") {
		// 替换转义序列
		s = strings.ReplaceAll(s, "\\\"", "\"")
		s = strings.ReplaceAll(s, "\\\\", "\\")
		s = strings.ReplaceAll(s, "\\/", "/")
		return s
	}
	return s
}

// ProcessLogMessage 优化处理特殊情况：日志消息中的嵌套JSON
func ProcessLogMessage(logJSON string) string {
	// 特殊处理日志消息字段
	var data map[string]interface{}
	if err := sonic.Unmarshal([]byte(logJSON), &data); err != nil {
		return logJSON
	}

	// 检查是否是特定结构的日志
	fields, ok := data["fields"].(map[string]interface{})
	if !ok {
		return OptimizedCleanNestedJSON(logJSON)
	}

	// 查找message字段
	message, ok := fields["message"].(string)
	if !ok {
		return OptimizedCleanNestedJSON(logJSON)
	}

	// 快速检查message是否像JSON
	trimmed := strings.TrimSpace(message)
	if len(trimmed) > 1 && trimmed[0] == '{' && trimmed[len(trimmed)-1] == '}' {
		// 尝试解析message
		var msgData interface{}

		// 处理可能的转义
		unescaped := unescapeJSONString(message)
		if err := sonic.Unmarshal([]byte(unescaped), &msgData); err == nil {
			// 替换原始消息
			fields["message"] = msgData

			// 重新编码
			result, err := sonic.Marshal(data)
			if err == nil {
				return string(result)
			}
		}
	}

	// 兜底方案：使用通用处理
	return OptimizedCleanNestedJSON(logJSON)
}
