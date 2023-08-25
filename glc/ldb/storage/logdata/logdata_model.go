/**
 * 日志模型
 * 1）面向日志接口，设定常用属性方便扩充
 */

package logdata

import (
	"encoding/json"

	"github.com/gotoeasy/glang/cmn"
)

// LogDataModel 日志模型
// Text是必须有的日志内容，Id自增，内置其他属性可选
// 其中Tags是空格分隔的标签，日期外各属性值会按空格分词
// 对应的json属性统一全小写
type LogDataModel struct {
	Id         string   `json:"id,omitempty"`         // 从1开始递增(36进制字符串)
	Text       string   `json:"text,omitempty"`       // 【必须】日志内容，多行时仅为首行，直接显示用，是全文检索对象
	Date       string   `json:"date,omitempty"`       // 日期（格式YYYY-MM-DD HH:MM:SS.SSS）
	System     string   `json:"system,omitempty"`     // 系统名
	ServerName string   `json:"servername,omitempty"` // 服务器名
	ServerIp   string   `json:"serverip,omitempty"`   // 服务器IP
	ClientIp   string   `json:"clientip,omitempty"`   // 客户端IP
	TraceId    string   `json:"traceid,omitempty"`    // 跟踪ID
	LogType    string   `json:"logtype,omitempty"`    // 日志类型（1:登录日志、2:操作日志）
	LogLevel   string   `json:"loglevel,omitempty"`   // 日志级别（debug、info、error等）
	User       string   `json:"user,omitempty"`       // 用户
	Module     string   `json:"module,omitempty"`     // 模块
	Operation  string   `json:"action,omitempty"`     // 操作
	Detail     string   `json:"detail,omitempty"`     // 多行时的详细日志信息，通常是包含错误堆栈等的日志内容（这部分内容不做索引处理）
	Tags       []string `json:"tags,omitempty"`       // 自定义标签，都作为关键词看待处理
	Keywords   []string `json:"keywords,omitempty"`   // 自定义的关键词
	Sensitives []string `json:"sensitives,omitempty"` // 要删除的敏感词
}

func (d *LogDataModel) ToJson() string {
	bt, _ := json.Marshal(d)
	return cmn.BytesToString(bt)
}

func (d *LogDataModel) LoadJson(jsonstr string) error {
	return json.Unmarshal(cmn.StringToBytes(jsonstr), d)
}
