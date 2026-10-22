<script setup lang="ts">
// 个人物品主界面：物品列表 + 详情抽屉（流水 / 装配）+ 业务动作 + 派生查询。
// 业务动作均走 itemActions（事务型：先写流水再更新主表），逻辑全部在后端 service 层。
import {ref, reactive, computed, onMounted} from 'vue'
import DataTable from '@/components/DataTable.vue'
import DataFormDialog from '@/components/DataFormDialog.vue'
import type {DataTableColumn} from '@/components/dataTable/types'
import {action} from '@/utils/request'
import {prompt, showError, showSuccess} from '@/utils/message'
import {useCrudAction} from '@/composables/useCrudAction'
import {
    itemsApi, itemActions, itemQueries,
    ITEM_TYPE_OPTIONS, STATUS_OPTIONS, STATUS_LABEL, RECORD_TYPE_LABEL,
    type Item,
    type ItemRecord, type ComponentView, type AssemblyHistory, type CategoryStat, type ReplenishCycle,
} from '@/api/items'
import {listDictOptions} from '@/api/dataDict'

const tableRef = ref<any>(null)
const {run} = useCrudAction()

const columns: DataTableColumn[] = [
    {field: 'Name', title: '名称', minWidth: 160, searchable: true},
    {field: 'ItemType', title: '类型', width: 90, formatter: (r: any) => (r.ItemType === 1 ? '耐用品' : '消耗品')},
    {field: 'CategoryID', title: '分类', width: 110, formatter: (r: any) => {
        const opt = categoryOptions.value.find(o => o.value === r.CategoryID)
        return opt ? opt.label : ''
    }},
    {field: 'Quantity', title: '数量', width: 100, sortable: true, formatter: (r: any) => `${r.Quantity} ${r.Unit || ''}`},
    {field: 'Status', title: '状态', width: 90, formatter: (r: any) => STATUS_LABEL[r.Status] || r.Status},
    {field: 'PurchasePrice', title: '价格', width: 100, formatter: (r: any) => (r.PurchasePrice != null ? `¥${r.PurchasePrice}` : '')},
    {field: 'PurchaseChannel', title: '来源', width: 100},
    {field: 'PurchaseDate', title: '购买日期', minWidth: 120, type: 'datetime', datetimeFormat: 'yyyy-MM-dd'},
    {field: 'actions', title: '操作', width: 130, fixed: 'right', type: 'actions'},
]

// 修改物品：复用内置表单字段配置（fields），只提交表单内的字段。
// quantity 不在 fields 中，后端也做了保护，不会误改库存。
const editVisible = ref(false)
const editSaving = ref(false)
const editForm = reactive<Record<string, any>>({})
let editingId = 0

function openEdit(row: any) {
    editingId = row.ID
    for (const f of fields.value) {
        const v = row[f.field]
        editForm[f.field] = v === undefined || v === '' ? (f.defaultValue ?? null) : v
    }
    editVisible.value = true
}

function submitEdit() {
    for (const f of fields.value) {
        if (f.required && (editForm[f.field] === null || editForm[f.field] === '')) {
            showError(`${f.title}不能为空`)
            return
        }
    }
    editSaving.value = true
    itemsApi.update(editingId, {...editForm}).then(() => {
        showSuccess('修改成功')
        editVisible.value = false
        tableRef.value?.refresh()
    }).catch((e: any) => {
        showError(e?.message || '修改失败')
    }).finally(() => {
        editSaving.value = false
    })
}

// 分类 / 位置 / 来源下拉选项（统一从数据字典加载）
const categoryOptions = ref<{label: string; value: number}[]>([])
const locationOptions = ref<{label: string; value: number}[]>([])
const sourceOptions = ref<{label: string; value: string}[]>([])
onMounted(async () => {
    try {
        const [cats, locs, srcs] = await Promise.all([
            listDictOptions('item_category'),
            listDictOptions('item_location'),
            listDictOptions('purchase_source'),
        ])
        categoryOptions.value = cats.map(c => ({label: c.Label, value: c.ID}))
        locationOptions.value = locs.map(l => ({label: l.Label, value: l.ID}))
        sourceOptions.value = srcs.map(s => ({label: s.Label, value: s.Value}))
        await loadItemOptions()
    } catch {
        // 字典加载失败不阻塞页面，仅退化为无选项下拉
    }
})

// 内置增改查表单字段（数量受后端保护，禁止在此直接编辑；状态变更建议走抽屉里的业务动作）
const fields = computed(() => [
    {field: 'Name', title: '名称', type: 'text', span: 12, required: true},
    {field: 'ItemType', title: '类型', type: 'select', span: 12, options: ITEM_TYPE_OPTIONS},
    {field: 'CategoryID', title: '分类', type: 'select', span: 12, options: categoryOptions.value, defaultValue: null, clearable: true},
    {field: 'LocationID', title: '位置', type: 'select', span: 12, options: locationOptions.value, defaultValue: null, clearable: true},
    {field: 'Brand', title: '品牌', type: 'text', span: 12},
    {field: 'Model', title: '型号', type: 'text', span: 12},
    {field: 'Spec', title: '规格', type: 'text', span: 12},
    {field: 'Color', title: '颜色', type: 'text', span: 12},
    {field: 'Unit', title: '单位', type: 'text', span: 12, defaultValue: '个'},
    {field: 'MinQuantity', title: '低库存阈值', type: 'number', span: 12, min: 0, defaultValue: null},
    {field: 'Status', title: '状态', type: 'select', span: 12, options: STATUS_OPTIONS},
    {field: 'SerialNo', title: '序列号', type: 'text', span: 12},
    {field: 'PurchaseChannel', title: '购入来源', type: 'select', span: 12, options: sourceOptions.value, clearable: true, allowCreate: true},
    {field: 'PurchasePrice', title: '购买价', type: 'number', span: 12, min: 0, precision: 2, step: 0.01, defaultValue: null},
    {field: 'PurchaseDate', title: '购买日期', type: 'datetime', span: 12},
    {field: 'WarrantyEnd', title: '保修到期', type: 'datetime', span: 12},
    {field: 'ExpireDate', title: '保质期', type: 'datetime', span: 12},
    {field: 'Tags', title: '标签', type: 'text', span: 12, placeholder: '逗号分隔'},
    {field: 'Remark', title: '备注', type: 'textarea', span: 24, rows: 2},
])

// 详情抽屉
const drawerVisible = ref(false)
const current = ref<Item | null>(null)
const recordRows = ref<ItemRecord[]>([])
const hostConfigRows = ref<ComponentView[]>([])
const componentHistoryRows = ref<AssemblyHistory[]>([])
const activeTab = ref('records')

async function openDetail(row: any) {
    current.value = row as Item
    drawerVisible.value = true
    activeTab.value = 'records'
    await loadRecords()
    await loadAssemblies()
}

async function loadRecords() {
    if (!current.value) return
    const res = await action<{ list: ItemRecord[] }>('item-records', 'list', {},
        {page: 1, pageSize: 200, filter: {item_id: current.value.ID}})
    recordRows.value = (res?.list ?? []).map((it: any) => ({...it, ID: it.ID ?? it.Id}))
}

async function loadAssemblies() {
    if (!current.value) return
    const [host, hist] = await Promise.all([
        itemQueries.hostConfig(current.value.ID),
        itemQueries.componentHistory(current.value.ID),
    ])
    hostConfigRows.value = host || []
    componentHistoryRows.value = hist || []
}

// 购入新物品
const purchaseVisible = ref(false)
const purchaseForm = reactive({
    Name: '', ItemType: 1, CategoryID: null as number | null, LocationID: null as number | null,
    Brand: '', Model: '', Spec: '', Color: '', SerialNo: '', Unit: '个', Quantity: 1, Status: 'in_use',
    MinQuantity: null as number | null, PurchaseChannel: null as string | null, PurchasePrice: null as number | null,
    PurchaseDate: null as string | null, WarrantyEnd: null as string | null, ExpireDate: null as string | null,
    Tags: '', Remark: '',
    RecordDate: new Date().toISOString(), Note: '',
})
function resetPurchase() {
    Object.assign(purchaseForm, {
        Name: '', ItemType: 1, CategoryID: null, LocationID: null, Brand: '', Model: '',
        Spec: '', Color: '', SerialNo: '', Unit: '个', Quantity: 1, Status: 'in_use', MinQuantity: null,
        PurchaseChannel: null, PurchasePrice: null, PurchaseDate: null, WarrantyEnd: null, ExpireDate: null, Tags: '', Remark: '',
        RecordDate: new Date().toISOString(), Note: '',
    })
}
function submitPurchase() {
    run(async () => {
        const payload = {
            item: {
                Name: purchaseForm.Name, ItemType: purchaseForm.ItemType, CategoryID: purchaseForm.CategoryID,
                LocationID: purchaseForm.LocationID, Brand: purchaseForm.Brand || null, Model: purchaseForm.Model || null,
                Spec: purchaseForm.Spec || null, Color: purchaseForm.Color || null, SerialNo: purchaseForm.SerialNo || null,
                Unit: purchaseForm.Unit, Quantity: purchaseForm.Quantity, Status: purchaseForm.Status,
                MinQuantity: purchaseForm.MinQuantity, PurchaseChannel: purchaseForm.PurchaseChannel || null, PurchasePrice: purchaseForm.PurchasePrice,
                PurchaseDate: purchaseForm.PurchaseDate, WarrantyEnd: purchaseForm.WarrantyEnd, ExpireDate: purchaseForm.ExpireDate,
                Tags: purchaseForm.Tags || null, Remark: purchaseForm.Remark || null,
            },
            recordDate: purchaseForm.RecordDate, note: purchaseForm.Note || null,
        }
        await itemActions.purchase(payload)
    }, {
        success: '购入成功',
        error: '购入失败',
        onDone: () => { purchaseVisible.value = false; resetPurchase(); tableRef.value?.refresh() },
    })
}

// 补货
const replenishVisible = ref(false)
const replenishForm = reactive({Qty: 1, Amount: null as number | null, RecordDate: new Date().toISOString(), Note: '', ConsumeOld: true})
function openReplenish() {
    replenishForm.Qty = 1; replenishForm.Amount = null; replenishForm.RecordDate = new Date().toISOString(); replenishForm.Note = ''; replenishForm.ConsumeOld = true
    replenishVisible.value = true
}
function submitReplenish() {
    if (!current.value) return
    run(async () => {
        await itemActions.replenish({
            itemId: current.value!.ID, qty: replenishForm.Qty, amount: replenishForm.Amount,
            recordDate: replenishForm.RecordDate, note: replenishForm.Note || null, consumeOld: replenishForm.ConsumeOld,
        })
    }, {success: '补货成功', error: '补货失败', onDone: () => { replenishVisible.value = false; tableRef.value?.refresh(); loadRecords() }})
}

// 简单动作：消耗 / 借出 / 归还 / 维修 / 处置 / 盘点
async function simpleAction(kind: 'consume' | 'lend' | 'return' | 'repair' | 'dispose' | 'inventory') {
    if (!current.value) return
    const id = current.value.ID
    const common = {itemId: id, recordDate: new Date().toISOString()}
    if (kind === 'consume') {
        const {value} = await prompt('本次消耗数量', {inputValue: '1'})
        const qty = Number(value)
        if (!(qty > 0)) return
        await run(() => itemActions.consume({...common, qty, note: null}), {success: '消耗成功', error: '消耗失败', onDone: () => { tableRef.value?.refresh(); loadRecords() }})
    } else if (kind === 'inventory') {
        const {value} = await prompt('实际盘点数量', {inputValue: String(current.value.Quantity)})
        const actual = Number(value)
        if (isNaN(actual)) return
        await run(() => itemActions.inventory({...common, actualQty: actual, note: null}), {success: '盘点已修正', error: '盘点失败', onDone: () => { tableRef.value?.refresh(); loadRecords() }})
    } else if (kind === 'repair') {
        const {value} = await prompt('维修费用（可空）', {inputValue: ''})
        const amount = value ? Number(value) : null
        await run(() => itemActions.repair({...common, amount, note: null}), {success: '已记录维修', error: '操作失败', onDone: () => { tableRef.value?.refresh(); loadRecords() }})
    } else {
        const map = {lend: ['借出', itemActions.lend], return: ['归还', itemActions.return], dispose: ['处置', itemActions.dispose]} as const
        const [label, fn] = map[kind as 'lend' | 'return' | 'dispose']
        await run(() => fn({...common, note: null}), {confirm: `确认${label}该物品？`, success: `${label}成功`, error: '操作失败', onDone: () => { tableRef.value?.refresh(); loadRecords() }})
    }
}

// 装配 / 拆下（当前物品作为配件，主机从下拉选择）
const itemOptions = ref<{label: string; value: number}[]>([])
async function loadItemOptions() {
    try {
        const res = await itemsApi.fetch({page: 1, pageSize: 500})
        itemOptions.value = (res.rows as Item[])
            .filter(it => !current.value || it.ID !== current.value.ID)
            .map(it => ({label: it.Brand ? `${it.Name}（${it.Brand}）` : it.Name, value: it.ID}))
    } catch {
        // 加载失败不阻塞，仅退化为无选项下拉
    }
}

const asmVisible = ref(false)
const asmMode = ref<'install' | 'remove'>('install')
const asmForm = reactive({HostID: null as number | null, Slot: '', Qty: 1})

function openAsm(mode: 'install' | 'remove') {
    asmMode.value = mode
    asmForm.HostID = null
    asmForm.Slot = ''
    asmForm.Qty = 1
    loadItemOptions()
    asmVisible.value = true
}

function submitAsm() {
    if (!current.value) return
    if (asmMode.value === 'install' && !(asmForm.HostID && asmForm.HostID > 0)) {
        showError('请选择主机')
        return
    }
    const mode = asmMode.value
    const hostId = asmForm.HostID ?? null
    run(async () => {
        if (mode === 'install') {
            await itemActions.install({
                hostId, componentId: current.value!.ID, slot: asmForm.Slot || null, qty: asmForm.Qty,
                installedAt: new Date().toISOString(), note: null,
            })
        } else {
            await itemActions.remove({
                componentId: current.value!.ID, hostId, removedAt: new Date().toISOString(), note: null,
            })
        }
    }, {
        success: mode === 'install' ? '装配成功' : '拆下成功',
        error: mode === 'install' ? '装配失败' : '拆下失败',
        onDone: () => {
            asmVisible.value = false
            tableRef.value?.refresh()
            loadRecords()
            loadAssemblies()
        },
    })
}

// 派生查询弹窗
const queryVisible = ref(false)
const queryTitle = ref('')
const lowStockRows = ref<Item[]>([])
const warrantyRows = ref<Item[]>([])
const statRows = ref<CategoryStat[]>([])
const forecastRows = ref<ReplenishCycle[]>([])
const queryTab = ref('low')
async function openQuery(tab: string) {
    queryTab.value = tab
    queryVisible.value = true
    if (tab === 'low') { queryTitle.value = '消耗品库存告急'; lowStockRows.value = await itemQueries.lowStock() }
    else if (tab === 'warranty') { queryTitle.value = '保修到期提醒(30天)'; warrantyRows.value = await itemQueries.warrantySoon(30) }
    else if (tab === 'stat') { queryTitle.value = '分类统计'; statRows.value = await itemQueries.categoryStats() }
    else if (tab === 'forecast') { queryTitle.value = '补货周期预测'; forecastRows.value = await itemQueries.replenishForecast() }
}
</script>

<template>
    <div>
        <div class="toolbar">
            <el-button type="primary" @click="purchaseVisible = true">购入新物品</el-button>
            <el-button @click="openQuery('low')">低库存预警</el-button>
            <el-button @click="openQuery('warranty')">保修到期</el-button>
            <el-button @click="openQuery('stat')">分类统计</el-button>
            <el-button @click="openQuery('forecast')">补货预测</el-button>
        </div>

        <DataTable ref="tableRef" :columns="columns" :api="itemsApi" :fields="fields"
                   :enabled-actions="['delete']" title="个人物品" row-key="ID">
            <template #actions="{row}">
                <el-button link type="primary" @click="openEdit(row)">修改</el-button>
                <el-button link type="primary" @click="openDetail(row)">详情</el-button>
            </template>
        </DataTable>

        <!-- 修改对话框（复用内置表单字段配置） -->
        <DataFormDialog
            v-model="editVisible"
            title="修改物品"
            mode="edit"
            :fields="fields"
            :form="editForm"
            :saving="editSaving"
            @save="submitEdit"
        />

        <!-- 购入对话框 -->
        <el-dialog v-model="purchaseVisible" title="购入新物品" width="800px">
            <el-form label-width="110px">
                <el-row :gutter="12">
                    <el-col :span="12"><el-form-item label="名称" required><el-input v-model="purchaseForm.Name"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="类型">
                        <el-select v-model="purchaseForm.ItemType"><el-option v-for="o in ITEM_TYPE_OPTIONS" :key="o.value" :label="o.label" :value="o.value"/></el-select>
                    </el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="分类">
                        <el-select v-model="purchaseForm.CategoryID" clearable placeholder="可空">
                            <el-option v-for="c in categoryOptions" :key="c.value" :label="c.label" :value="c.value"/>
                        </el-select>
                    </el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="位置">
                        <el-select v-model="purchaseForm.LocationID" clearable placeholder="可空">
                            <el-option v-for="l in locationOptions" :key="l.value" :label="l.label" :value="l.value"/>
                        </el-select>
                    </el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="品牌"><el-input v-model="purchaseForm.Brand"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="型号"><el-input v-model="purchaseForm.Model"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="规格"><el-input v-model="purchaseForm.Spec"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="颜色"><el-input v-model="purchaseForm.Color"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="序列号"><el-input v-model="purchaseForm.SerialNo"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="购入来源">
                        <el-select v-model="purchaseForm.PurchaseChannel" clearable filterable allow-create default-first-option placeholder="可空/自定义">
                            <el-option v-for="o in sourceOptions" :key="o.value" :label="o.label" :value="o.value"/>
                        </el-select>
                    </el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="单位"><el-input v-model="purchaseForm.Unit"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="数量"><el-input-number v-model="purchaseForm.Quantity" :min="0" :step="1" controls-position="right"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="低库存阈值"><el-input-number v-model="purchaseForm.MinQuantity" :min="0" controls-position="right"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="状态">
                        <el-select v-model="purchaseForm.Status"><el-option v-for="o in STATUS_OPTIONS" :key="o.value" :label="o.label" :value="o.value"/></el-select>
                    </el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="购买价"><el-input-number v-model="purchaseForm.PurchasePrice" :min="0" :precision="2" controls-position="right"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="购买日期"><el-date-picker v-model="purchaseForm.PurchaseDate" type="date" value-format="YYYY-MM-DDTHH:mm:ssZ" placeholder="可空"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="保修到期"><el-date-picker v-model="purchaseForm.WarrantyEnd" type="date" value-format="YYYY-MM-DDTHH:mm:ssZ" placeholder="可空"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="保质期"><el-date-picker v-model="purchaseForm.ExpireDate" type="date" value-format="YYYY-MM-DDTHH:mm:ssZ" placeholder="可空"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="购入日期"><el-date-picker v-model="purchaseForm.RecordDate" type="date" value-format="YYYY-MM-DDTHH:mm:ssZ"/></el-form-item></el-col>
                    <el-col :span="12"><el-form-item label="标签"><el-input v-model="purchaseForm.Tags" placeholder="逗号分隔"/></el-form-item></el-col>
                    <el-col :span="24"><el-form-item label="备注"><el-input v-model="purchaseForm.Remark" type="textarea" :rows="2"/></el-form-item></el-col>
                </el-row>
            </el-form>
            <template #footer>
                <el-button @click="purchaseVisible = false">取消</el-button>
                <el-button type="primary" @click="submitPurchase">确认购入</el-button>
            </template>
        </el-dialog>

        <!-- 补货对话框 -->
        <el-dialog v-model="replenishVisible" title="消耗品补货" width="460px">
            <el-form label-width="100px">
                <el-form-item label="补货数量"><el-input-number v-model="replenishForm.Qty" :min="0" :step="1" controls-position="right"/></el-form-item>
                <el-form-item label="金额"><el-input-number v-model="replenishForm.Amount" :min="0" :precision="2" controls-position="right"/></el-form-item>
                <el-form-item label="补货日期"><el-date-picker v-model="replenishForm.RecordDate" type="date" value-format="YYYY-MM-DDTHH:mm:ssZ"/></el-form-item>
                <el-form-item label="清空旧库存"><el-switch v-model="replenishForm.ConsumeOld"/></el-form-item>
                <el-form-item label="备注"><el-input v-model="replenishForm.Note" type="textarea" :rows="2"/></el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="replenishVisible = false">取消</el-button>
                <el-button type="primary" @click="submitReplenish">确认补货</el-button>
            </template>
        </el-dialog>

        <!-- 装配 / 拆下对话框 -->
        <el-dialog v-model="asmVisible" :title="asmMode === 'install' ? '装入主机' : '从主机拆下'" width="460px">
            <el-form label-width="100px">
                <el-form-item :label="asmMode === 'install' ? '主机' : '主机（可空）'">
                    <el-select v-model="asmForm.HostID" clearable filterable
                               :placeholder="asmMode === 'install' ? '请选择主机' : '留空表示任意主机'">
                        <el-option v-for="o in itemOptions" :key="o.value" :label="o.label" :value="o.value"/>
                    </el-select>
                </el-form-item>
                <el-form-item v-if="asmMode === 'install'" label="槽位">
                    <el-input v-model="asmForm.Slot" placeholder="可空，如 内存槽1"/>
                </el-form-item>
                <el-form-item v-if="asmMode === 'install'" label="数量">
                    <el-input-number v-model="asmForm.Qty" :min="1" controls-position="right"/>
                </el-form-item>
            </el-form>
            <template #footer>
                <el-button @click="asmVisible = false">取消</el-button>
                <el-button type="primary" @click="submitAsm">{{ asmMode === 'install' ? '确认装入' : '确认拆下' }}</el-button>
            </template>
        </el-dialog>

        <!-- 详情抽屉 -->
        <el-drawer v-model="drawerVisible" :title="current ? current.Name : '详情'" size="560px">
            <template v-if="current">
                <el-descriptions :column="2" border size="small">
                    <el-descriptions-item label="类型">{{ current.ItemType === 1 ? '耐用品' : '消耗品' }}</el-descriptions-item>
                    <el-descriptions-item label="状态">{{ STATUS_LABEL[current.Status] || current.Status }}</el-descriptions-item>
                    <el-descriptions-item label="数量">{{ current.Quantity }} {{ current.Unit }}</el-descriptions-item>
                    <el-descriptions-item label="序列号">{{ current.SerialNo || '-' }}</el-descriptions-item>
                    <el-descriptions-item label="品牌/型号">{{ current.Brand || '-' }} {{ current.Model || '' }}</el-descriptions-item>
                    <el-descriptions-item label="保修到期">{{ current.WarrantyEnd || '-' }}</el-descriptions-item>
                </el-descriptions>

                <div class="drawer-actions">
                    <el-button size="small" @click="openReplenish" v-if="current.ItemType === 2">补货</el-button>
                    <el-button size="small" @click="simpleAction('consume')">消耗</el-button>
                    <el-button size="small" @click="simpleAction('lend')">借出</el-button>
                    <el-button size="small" @click="simpleAction('return')">归还</el-button>
                    <el-button size="small" @click="simpleAction('repair')">维修</el-button>
                    <el-button size="small" @click="simpleAction('inventory')">盘点</el-button>
                    <el-button size="small" @click="openAsm('install')">装配</el-button>
                    <el-button size="small" @click="openAsm('remove')">拆下</el-button>
                    <el-button size="small" type="danger" @click="simpleAction('dispose')">处置</el-button>
                </div>

                <el-tabs v-model="activeTab">
                    <el-tab-pane label="变动流水" name="records">
                        <el-table :data="recordRows" size="small" border>
                            <el-table-column prop="RecordType" label="类型" width="90">
                                <template #default="{row}">{{ RECORD_TYPE_LABEL[row.RecordType] || row.RecordType }}</template>
                            </el-table-column>
                            <el-table-column prop="Quantity" label="数量" width="80"/>
                            <el-table-column prop="Amount" label="金额" width="90"/>
                            <el-table-column prop="RecordDate" label="日期" width="120"/>
                            <el-table-column prop="Note" label="备注" min-width="120"/>
                        </el-table>
                    </el-tab-pane>
                    <el-tab-pane label="装配（作为主机）" name="host">
                        <el-table :data="hostConfigRows" size="small" border>
                            <el-table-column prop="Name" label="配件" min-width="120"/>
                            <el-table-column prop="Slot" label="槽位" width="100"/>
                            <el-table-column prop="Quantity" label="数量" width="70"/>
                            <el-table-column prop="InstalledAt" label="装入日期" width="120"/>
                        </el-table>
                        <el-empty v-if="!hostConfigRows.length" description="暂无在位配件"/>
                    </el-tab-pane>
                    <el-tab-pane label="流转（作为配件）" name="history">
                        <el-table :data="componentHistoryRows" size="small" border>
                            <el-table-column prop="HostName" label="主机" min-width="120"/>
                            <el-table-column prop="Slot" label="槽位" width="100"/>
                            <el-table-column prop="InstalledAt" label="装入" width="120"/>
                            <el-table-column prop="RemovedAt" label="拆下" width="120"/>
                            <el-table-column prop="State" label="状态" width="80"/>
                        </el-table>
                        <el-empty v-if="!componentHistoryRows.length" description="暂无流转记录"/>
                    </el-tab-pane>
                </el-tabs>
            </template>
        </el-drawer>

        <!-- 派生查询弹窗 -->
        <el-dialog v-model="queryVisible" :title="queryTitle" width="720px">
            <el-table v-if="queryTab === 'low'" :data="lowStockRows" size="small" border>
                <el-table-column prop="Name" label="名称" min-width="140"/>
                <el-table-column prop="Quantity" label="当前" width="80"/>
                <el-table-column prop="Unit" label="单位" width="70"/>
                <el-table-column prop="MinQuantity" label="阈值" width="80"/>
            </el-table>
            <el-table v-else-if="queryTab === 'warranty'" :data="warrantyRows" size="small" border>
                <el-table-column prop="Name" label="名称" min-width="140"/>
                <el-table-column prop="WarrantyEnd" label="到期日" width="130"/>
                <el-table-column prop="Brand" label="品牌" width="100"/>
            </el-table>
            <el-table v-else-if="queryTab === 'stat'" :data="statRows" size="small" border>
                <el-table-column label="分类" min-width="140"><template #default="{row}">{{ row.CategoryName || '未分类' }}</template></el-table-column>
                <el-table-column prop="Count" label="数量" width="90"/>
                <el-table-column prop="TotalValue" label="总价值" width="120"/>
            </el-table>
            <el-table v-else :data="forecastRows" size="small" border>
                <el-table-column prop="ItemName" label="物品" min-width="140"/>
                <el-table-column prop="Cycles" label="周期数" width="80"/>
                <el-table-column prop="AvgCycleDays" label="平均天数" width="100"/>
                <el-table-column prop="LastBuy" label="上次补货" width="130"/>
                <el-table-column prop="NextBuyEstimate" label="预计下次" width="130"/>
            </el-table>
            <el-empty v-if="(queryTab==='low'&&!lowStockRows.length)||(queryTab==='warranty'&&!warrantyRows.length)" description="无数据"/>
        </el-dialog>
    </div>
</template>

<style scoped>
.toolbar { margin-bottom: 12px; display: flex; gap: 8px; flex-wrap: wrap; }
.drawer-actions { margin: 12px 0; display: flex; gap: 8px; flex-wrap: wrap; }
</style>
