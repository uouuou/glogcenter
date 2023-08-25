package controller

import (
	"glc/conf"
	"glc/gweb"
	"glc/ldb"

	"github.com/gotoeasy/glang/cmn"
)

// LogSearchController 日志检索（表单提交方式）
func LogSearchController(req *gweb.HttpRequest) *gweb.HttpResult {
	for _, s := range GetSessionid() {
		if conf.IsEnableLogin() && req.GetFormParameter("token") == s["sessionid"] {
			storeName := req.GetFormParameter("storeName")
			//searchKey := tokenizer.GetSearchKey(req.GetFormParameter("searchKey"))
			searchKey := req.GetFormParameter("searchKey")
			currentId := cmn.StringToUint32(req.GetFormParameter("currentId"), 0)
			forward := cmn.StringToBool(req.GetFormParameter("forward"), true)
			datetimeFrom := req.GetFormParameter("datetimeFrom")
			datetimeTo := req.GetFormParameter("datetimeTo")
			system := req.GetFormParameter("system")
			loglevel := req.GetFormParameter("loglevel")

			if !cmn.IsBlank(system) {
				system = "~" + cmn.Trim(system)
			}
			if !cmn.IsBlank(loglevel) {
				loglevel = "!" + cmn.Trim(loglevel)
			}

			eng := ldb.NewEngine(storeName)
			rs := eng.Search(searchKey, system, datetimeFrom, datetimeTo, loglevel, currentId, forward)
			rs.PageSize = cmn.IntToString(conf.GetPageSize())
			return gweb.Result(rs)

		}
	}
	return gweb.Error403() // 登录检查
}
