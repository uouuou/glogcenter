package controller

import (
	"fmt"
	"glc/com"
	"glc/conf"
	"glc/gweb"
	"glc/ldb/storage"
	"glc/ldb/storage/indexword"

	"github.com/gotoeasy/glang/cmn"
)

// IndexResumeController 触发指定日志仓（支持按日期）继续索引
// 参数：
// - storeName: 指定日志仓名称（例如 logdata-20250101），如果为空且开启“仓名自动追加日期”则需传入 date/ymd
// - date/ymd: 指定日期（YYYYMMDD），在 conf.IsStoreNameAutoAddDate() 为 true 时用于拼接仓名
// - force: 可选，=1 时异步强制补建所有未完成索引（容错跳过坏记录）
// - fromId: 可选，从此ID开始重建（内部会将进度重置为 fromId-1 再补建）；与 resetTo 二选一
// - resetTo: 可选，直接将已建索引进度重置为该值（请谨慎，不能小于现有数据的真实已建范围，且不会删除已存在的词索引）
// 安全：沿用白/黑名单与（可选）登录校验
func IndexResumeController(req *gweb.HttpRequest) *gweb.HttpResult {
	if (!InWhiteList(req) && InBlackList(req)) || (conf.IsEnableLogin() && GetUsernameByToken(req.GetToken()) == "") {
		return gweb.Error403() // 黑名单检查、登录检查
	}

	storeName := cmn.Trim(req.GetFormParameter("storeName"))
	ymd := cmn.Trim(req.GetFormParameter("date"))
	if ymd == "" {
		// 兼容参数名 ymd
		ymd = cmn.Trim(req.GetFormParameter("ymd"))
	}
	force := cmn.Trim(req.GetFormParameter("force")) == "1"
	fromIdStr := cmn.Trim(req.GetFormParameter("fromId"))
	resetToStr := cmn.Trim(req.GetFormParameter("resetTo"))

	if storeName == "" {
		if conf.IsStoreNameAutoAddDate() {
			if ymd == "" {
				return gweb.Error500("请提供 storeName 或 date(YYYYMMDD)")
			}
			if cmn.Len(ymd) != 8 || !cmn.Startwiths(ymd, "20") {
				return gweb.Error500("date 格式应为 YYYYMMDD")
			}
			storeName = "logdata-" + ymd
		} else {
			storeName = "logdata" // 非自动追加日期模式，只有单一仓
		}
	}

	// 检查仓是否存在（不存在也可触发，若该仓为空则无索引可建）
	names := com.GetStorageNames(conf.GetStorageRoot(), ".sysmnt")
	has := false
	for _, n := range names {
		if cmn.ToLower(n) == cmn.ToLower(storeName) {
			has = true
			break
		}
	}

	// 打开存储句柄（唤醒构建协程）
	handle := storage.NewLogDataStorageHandle(storeName)
	total := handle.TotalCount()
	curIdx := indexword.NewWordIndexStorage(storeName).GetIndexedCount()

	// 可选重置进度
	resetInfo := ""
	if fromIdStr != "" || resetToStr != "" {
		var to uint32
		if fromIdStr != "" {
			fromId := cmn.StringToUint32(fromIdStr, 0)
			if fromId > 0 {
				to = fromId - 1
			} else {
				to = 0
			}
		} else {
			to = cmn.StringToUint32(resetToStr, curIdx)
		}
		if to > total {
			to = total
		}
		handle.ResetIndexedCount(to)
		curIdx = to
		resetInfo = fmt.Sprint("已重置索引进度到 ", to)
	}

	info := "已触发继续索引，后台将自动持续构建"
	if !has {
		info = info + "（提示：尚未发现该日志仓目录，若该仓为空则无索引可建）"
	}
	if curIdx >= total {
		info = "索引已完成，无需继续"
	}

	// 可选：异步强制补建（跳过坏记录，直到完成）
	var started bool
	if force {
		started = true
		go func() {
			built := handle.ForceIndexAll()
			cmn.Info("IndexResumeController 异步补建完成:", storeName, ", built=", built)
		}()
		info = info + "；已异步启动强制补建（容错跳过坏记录）"
	}

	return gweb.Result(cmn.OfMap(
		"storeName", storeName,
		"total", total,
		"indexed", curIdx,
		"started", started,
		"reset", resetInfo,
		"info", info,
	))
}
