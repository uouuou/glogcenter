package controller

import (
	"fmt"
	"glc/com"
	"glc/conf"
	"glc/gweb"
	"glc/ldb/status"
	"glc/ldb/sysmnt"
	"glc/ver"
	"time"

	"github.com/gotoeasy/glang/cmn"
)

var glcLatest string = ver.VERSION

// 查询是否测试模式
func TestModeController(req *gweb.HttpRequest) *gweb.HttpResult {
	return gweb.Result(conf.IsTestMode())
}

// 查询版本信息
func VersionController(req *gweb.HttpRequest) *gweb.HttpResult {
	rs := cmn.OfMap("version", ver.VERSION, "latest", glcLatest) // version当前版本号，latest最新版本号
	return gweb.Result(rs)
}

// StorageNamesController 查询日志仓名称列表
func StorageNamesController(req *gweb.HttpRequest) *gweb.HttpResult {
	if !InWhiteList(req) && InBlackList(req) {
		return gweb.Error403() // 黑名单，访问受限
	}
	for _, s := range GetSessionid() {
		if conf.IsEnableLogin() && req.GetFormParameter("token") == s["sessionid"] {
			rs := com.GetStorageNames(conf.GetStorageRoot(), ".sysmnt")
			return gweb.Result(rs)

		}
	}
	return gweb.Error403() // 登录检查

}

// StorageListController 查询日志仓信息列表
func StorageListController(req *gweb.HttpRequest) *gweb.HttpResult {
	if !InWhiteList(req) && InBlackList(req) {
		return gweb.Error403() // 黑名单，访问受限
	}
	for _, s := range GetSessionid() {
		if conf.IsEnableLogin() && req.GetFormParameter("token") == s["sessionid"] {
			rs := sysmnt.GetStorageList()
			return gweb.Result(rs)

		}
	}
	return gweb.Error403() // 登录检查
}

// StorageDeleteController 删除指定日志仓
func StorageDeleteController(req *gweb.HttpRequest) *gweb.HttpResult {
	if !InWhiteList(req) && InBlackList(req) {
		return gweb.Error403() // 黑名单，访问受限
	}
	for _, s := range GetSessionid() {
		if conf.IsEnableLogin() && req.GetFormParameter("token") == s["sessionid"] {
			name := req.GetFormParameter("storeName")
			if name == ".sysmnt" {
				return gweb.Error500("不能删除 .sysmnt")
			} else if conf.IsStoreNameAutoAddDate() {
				if conf.GetSaveDays() > 0 {
					ymd := cmn.Right(name, 8)
					if cmn.Len(ymd) == 8 && cmn.Startwiths(ymd, "20") {
						msg := fmt.Sprintf("当前是日志仓自动维护模式，最多保存 %d 天，不支持手动删除", conf.GetSaveDays())
						return gweb.Error500(msg)
					}
				}
			} else if name == "logdata" {
				return gweb.Error500("日志仓 " + name + " 正在使用，不能删除")
			}

			if status.IsStorageOpening(name) {
				return gweb.Error500("日志仓 " + name + " 正在使用，不能删除")
			}

			err := sysmnt.DeleteStorage(name)
			if err != nil {
				cmn.Error("日志仓", name, "删除失败", err)
				return gweb.Error500("日志仓 " + name + " 正在使用，不能删除")
			}
			return gweb.Ok()

		}
	}
	return gweb.Error403() // 登录检查
}

// 尝试查询最新版本号（注：服务不一定总是可用，每小时查取一次）
func init() {
	go func() {
		url := "https://glc.gotoeasy.top/glogcenter/current/version.json?v=" + ver.VERSION
		v := cmn.GetGlcLatestVersion(url)
		glcLatest = cmn.IifStr(v != "", v, glcLatest)
		ticker := time.NewTicker(time.Hour)
		for range ticker.C {
			v = cmn.GetGlcLatestVersion(url)
			glcLatest = cmn.IifStr(v != "", v, glcLatest)
		}
	}()
}
