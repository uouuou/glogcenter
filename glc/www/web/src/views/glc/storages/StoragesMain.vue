<template>
  <div v-show="visible">
    <el-row style="align-items: center;height:26px;">
      <SvgIcon name="detail" height="16" width="16" style="margin: 0 6px 0 0;color:var(--el-color-primary)" />
      <span style="font-size:16px">日志仓信息列表</span>
    </el-row>

    <el-divider style="margin: 0 0 8px" />

    <GxToolbar style="margin-bottom: 8px" class="c-btn">
      <template #left>
        <GxButton icon="refresh-right" @click="search">刷 新</GxButton>
      </template>

      <template #right>
        <el-tooltip content="缩放" placement="top">
          <el-button circle @click="emitter.emit('main:switchMaximizePage')">
            <SvgIcon name="zoom" />
          </el-button>
        </el-tooltip>
        <GxPageTableConfig :tid="tid" :page-config="pageSettingStore" />
      </template>
    </GxToolbar>

    <GxTable ref="table" v-loading="showTableLoadding" scrollbar-always-on stripe :enable-header-contextmenu="false"
      :tid="tid" :data="tableData" :max-height="tableHeight" :height="tableHeight" class="c-gx-table c-glc-table"
      row-key="id">
      <template #$operation="{ row }">
        <!-- 新增：继续索引 / 强制补建 / 从ID重建 -->
        <el-button size="small" type="primary" @click="resume(row)">继续索引</el-button>
        <el-button size="small" type="success" @click="forceResume(row)">强制补建</el-button>
        <el-button size="small" @click="resetResume(row)">从ID重建</el-button>
        <!-- 原有：删除 -->
        <el-button size="small" type="warning" @click="remove(row)">删除</el-button>
      </template>
    </GxTable>

    <div>
      <div style="display:flex;justify-content:space-between;">
        <div style="min-height:30px;padding-top:5px; line-height:30px; color: #909399;" v-text="info"></div>
      </div>
    </div>

  </div>
</template>

<script setup>
import { userLogout } from "~/api";
import { useEmitter, usePageMainHooks, useTabsState } from "~/pkgs";

const tabsState = useTabsState();
const emitter = useEmitter(tabsState.activePath);
const router = useRouter();

const opt = {
  emitter,
  withoutSearchForm: true,
};
const { visible, tableData, getTableHeight, pageSettingStore, showTableLoadding } = usePageMainHooks(opt);

const table = ref(); // 表格实例
const tid = ref('storagesMain231126'); // 表格ID
const info = ref(''); // 底部提示信息

const tableHeight = computed(() => getTableHeight(false, true, 19)); // 表格高度

// 初期默认检索
onMounted(() => {
  const configStore = $emitter.emit('$table:config', { id: tid.value });
  !configStore.columns.length && $emitter.emit('$table:config', { id: tid.value, update: true }); // 首次使用开启默认布局
  search()
});

function search() {
  // 日志仓列表查询
  showTableLoadding.value = true;
  const url = `/v1/store/list`;
  $post(url, {}, null, { 'Content-Type': 'application/x-www-form-urlencoded' }).then(rs => {
    console.log(rs)
    if (rs.success) {
      const data = rs.result.data || [];
      tableData.value.splice(0, tableData.value.length, ...data);
      info.value = rs.result.info;
      document.querySelector('.c-glc-table .el-scrollbar__wrap').scrollTop = 0; // 滚动到顶部
    } else if (rs.code == 403) {
      userLogout(); // 403 时登出
      router.push('/glc/login');
    }
  }).finally(() => {
    showTableLoadding.value = false;
  });
}

async function remove(row) {
  // 日志仓删除
  if (await $msg.confirm(`确定要删除日志仓 ${row.name} 吗？`)) {
    const url = `/v1/store/delete`;
    $post(url, { storeName: row.name }, null, { 'Content-Type': 'application/x-www-form-urlencoded' }).then(rs => {
      console.log(rs)
      if (rs.success) {
        $msg.info(`已删除日志仓 ${row.name}`);
        search();
      } else if (rs.code == 403) {
        userLogout(); // 403 时登出
        router.push('/glc/login');
      } else {
        $msg.error(rs.message);
      }
    }).finally(() => {
      showTableLoadding.value = false;
    });
  }
}

function callResumeApi(params, onDone) {
  const url = `/v1/index/resume`;
  showTableLoadding.value = true;
  $post(url, params, null, { 'Content-Type': 'application/x-www-form-urlencoded' }).then(rs => {
    console.log('resume:', rs);
    if (rs.success) {
      const r = rs.result || {};
      const tip = [
        r.info || '',
        r.started ? '（已启动异步补建）' : '',
        r.reset ? `；${r.reset}` : ''
      ].join('');
      $msg.info(tip || '已触发继续索引');
      info.value = tip;
      onDone && onDone(true, r);
      // 触发一次刷新，便于看到 indexed/total 的变化
      setTimeout(() => search(), 600);
    } else if (rs.code == 403) {
      userLogout();
      router.push('/glc/login');
      onDone && onDone(false);
    } else {
      $msg.error(rs.message || '触发继续索引失败');
      onDone && onDone(false);
    }
  }).finally(() => {
    showTableLoadding.value = false;
  });
}

function resume(row) {
  // 继续索引（不强制，只唤醒后台补建）
  if (!row || !row.name) return;
  callResumeApi({ storeName: row.name }, null);
}

function forceResume(row) {
  // 强制补建（容错跳过坏记录，直到完成）
  if (!row || !row.name) return;
  callResumeApi({ storeName: row.name, force: 1 }, null);
}

async function resetResume(row) {
  // 从指定ID重建（将进度重置为 fromId-1，然后开启补建）
  if (!row || !row.name) return;
  // 简单输入框获取 fromId（优先使用全局 $msg.prompt，如无则降级 window.prompt）
  let fromId = null;
  if ($msg && $msg.prompt) {
    try {
      const ret = await $msg.prompt(`请输入起始ID（将从该ID开始重建，实际会重置到 fromId-1）：`, '从ID重建', {
        inputPattern: /^\d+$/,
        inputErrorMessage: '请输入正整数ID',
      });
      if (ret && ret.value != null) {
        fromId = (ret.value + '').trim();
      }
    } catch (e) {
      return; // 取消
    }
  } else {
    const v = window.prompt(`请输入起始ID（将从该ID开始重建，实际会重置到 fromId-1）：`, '');
    if (v == null) return; // 取消
    fromId = (v + '').trim();
  }
  if (!fromId || !/^\d+$/.test(fromId)) {
    $msg.error('请输入有效的正整数ID');
    return;
  }
  callResumeApi({ storeName: row.name, fromId, force: 1 }, null);
}

</script>

<style>
.c-glc-table .el-popper.is-dark {
  display: none;
}

.x-detail {
  padding: 5px 5px 5px 30px;
  background-color: floralwhite;
}

.c-search-form .c-search-form-item {
  margin-bottom: 0;
}

.c-search-form .c-btn-badge.el-badge .el-button--small.is-circle {
  /* width: var(--el-button-size);
  min-width: var(--el-button-size); */
  width: 30px;
  min-width: 30px;
  height: 30px;
  min-height: 30px;
  margin-top: -3px;
}

.c-search-form.el-form--inline .el-input {
  --el-input-width: 100%;
}

.c-datapicker.el-popper.is-pure {
  margin-left: -100px;
}
</style>
