package controller

import (
	"github.com/bytedance/sonic"
	"glc/conf"
	"glc/gweb"
	"glc/ldb"
	"glc/ldb/storage/logdata"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gotoeasy/glang/cmn"
)

var mapSystem = make(map[string]int64)
var muSystem sync.Mutex

// JsonLogAddBatchController 添加日志（JSON提交方式）
func JsonLogAddBatchController(req *gweb.HttpRequest) *gweb.HttpResult {

	// 开启API秘钥校验时才检查
	if conf.IsEnableSecurityKey() && req.GetHeader(conf.GetHeaderSecurityKey()) != conf.GetSecurityKey() {
		return gweb.Error(403, "未经授权的访问，拒绝服务")
	}

	var mds []logdata.LogDataModel
	err := req.BindJSON(&mds)
	if err != nil {
		cmn.Error("请求参数有误", err)
		return gweb.Error500(err.Error())
	}

	for i := 0; i < len(mds); i++ {
		md := &mds[i]
		md.Text = cmn.Trim(md.Text)
		if md.Text != "" {
			addDataModelLog(md)
			if conf.IsClusterMode() {
				go TransferGlc(conf.LogTransferAdd, md.ToJson()) // 转发其他GLC服务
			}
		}
	}
	return gweb.Ok()

}

// JsonLogAddController 添加日志（JSON提交方式）
func JsonLogAddController(req *gweb.HttpRequest) *gweb.HttpResult {

	// 开启API秘钥校验时才检查
	if conf.IsEnableSecurityKey() && req.GetHeader(conf.GetHeaderSecurityKey()) != conf.GetSecurityKey() {
		return gweb.Error(403, "未经授权的访问，拒绝服务")
	}

	md := &logdata.LogDataModel{}
	err := req.BindJSON(md)
	if err != nil {
		cmn.Error("请求参数有误", err)
		return gweb.Error500(err.Error())
	}

	// 客户端IP地址没有时
	if md.ClientIp == "" {
		md.ClientIp = req.GetClientIp()
	}
	if md.ServerIp == "" {
		md.ServerIp = md.ClientIp
	}
	md.Text = cmn.Trim(md.Text)
	if md.Text != "" {
		md.Text = cleanAllNestedJSON(md.Text)
		addDataModelLog(md)
		if conf.IsClusterMode() {
			go TransferGlc(conf.LogTransferAdd, md.ToJson()) // 转发其他GLC服务
		}
	}

	return gweb.Ok()
}

// 递归处理任意位置的嵌套JSON字符串
func cleanAllNestedJSON(jsonStr string) string {
	var data interface{}
	// 首先解析外层JSON
	err := sonic.Unmarshal([]byte(jsonStr), &data)
	if err != nil {
		return jsonStr // 如果解析失败，返回原始字符串
	}
	// 递归处理所有嵌套的JSON字符串
	cleanedData := recursiveCleanJSON(data)
	// 重新编码为紧凑JSON
	cleanedBytes, err := sonic.Marshal(cleanedData)
	if err != nil {
		return jsonStr
	}
	return string(cleanedBytes)
}

// 递归处理数据结构中的所有嵌套JSON字符串
func recursiveCleanJSON(data interface{}) interface{} {
	switch v := data.(type) {
	case map[string]interface{}:
		// 处理对象/字典
		for key, value := range v {
			v[key] = recursiveCleanJSON(value)
		}
		return v
	case []interface{}:
		// 处理数组
		for i, value := range v {
			v[i] = recursiveCleanJSON(value)
		}
		return v
	case string:
		// 检查字符串是否可能是JSON (包括转义的JSON)
		return processJSONString(v)
	default:
		// 其他基本类型保持不变
		return v
	}
}

// 处理可能是JSON的字符串
func processJSONString(s string) interface{} {
	// 尝试直接解析
	var directData interface{}
	if err := sonic.UnmarshalString(s, &directData); err == nil {
		// 直接解析成功，递归处理
		return recursiveCleanJSON(directData)
	}

	// 检查是否是转义的JSON字符串 - 关键修复
	trimmed := strings.TrimSpace(s)
	if len(trimmed) > 1 &&
		((strings.HasPrefix(trimmed, "{") && strings.Contains(trimmed, "\\\"")) ||
			(strings.HasPrefix(trimmed, "[") && strings.Contains(trimmed, "\\\""))) {

		// 尝试解除一层转义并重新解析
		unescaped := strings.ReplaceAll(trimmed, "\\\"", "\"")
		unescaped = strings.ReplaceAll(unescaped, "\\\\", "\\")
		unescaped = strings.ReplaceAll(unescaped, "\\/", "/")

		var unescapedData interface{}
		if err := sonic.UnmarshalString(unescaped, &unescapedData); err == nil {
			// 成功解析转义后的JSON，返回嵌套对象而非字符串
			return recursiveCleanJSON(unescapedData)
		}
	}

	// 不是JSON或处理失败，保持原样
	return s
}

// JsonLogTransferAddController 添加日志（来自数据转发）
func JsonLogTransferAddController(req *gweb.HttpRequest) *gweb.HttpResult {

	// 开启API秘钥校验时才检查
	if conf.IsEnableSecurityKey() && req.GetHeader(conf.GetHeaderSecurityKey()) != conf.GetSecurityKey() {
		return gweb.Error(403, "未经授权的访问，拒绝服务")
	}

	md := &logdata.LogDataModel{}
	err := req.BindJSON(md)
	if err != nil {
		cmn.Error("请求参数有误", err)
		return gweb.Error500(err.Error())
	}

	md.Text = cmn.Trim(md.Text)
	addDataModelLog(md)
	// addTextLog(md)
	return gweb.Ok()
}

// 添加日志
func addDataModelLog(data *logdata.LogDataModel) {
	engine := ldb.NewDefaultEngine()

	// 按配置要求在IP字段上附加城市信息（当IP含空格时认为已附加过）
	if conf.IsIpAddCity() {
		if data.ClientIp != "" && !cmn.Contains(data.ClientIp, " ") {
			data.ClientIp = cmn.GetCityIp(data.ClientIp)
		}
		if data.ServerIp != "" && !cmn.Contains(data.ServerIp, " ") {
			data.ServerIp = cmn.GetCityIp(data.ServerIp)
		}
	}

	engine.AddLogDataModel(data)

	// 缓存系统名称备用查询
	if data.System != "" {
		if muSystem.TryLock() {
			defer muSystem.Unlock()
			mapSystem[data.System] = time.Now().UnixMilli()
		}
	}

}

// GetAllSystemNames 取近1天缓存的系统名称并清理
func GetAllSystemNames() []string {
	var mapSet = make(map[string]bool)
	var rs []string
	var dels []string
	now := time.Now().UnixMilli()
	muSystem.Lock()
	defer muSystem.Unlock()
	for key, value := range mapSystem {
		if now-value < 86400000 {
			tmp := cmn.ToLower(key)
			if _, has := mapSet[tmp]; !has {
				rs = append(rs, key) // 一天内
				mapSet[tmp] = true
			}
		} else {
			dels = append(dels, key) // 超过1天待删除
		}
	}
	for _, key := range dels {
		delete(mapSystem, key)
	}
	sort.Slice(rs, func(i, j int) bool {
		return rs[i] < rs[j]
	})
	return rs
}
