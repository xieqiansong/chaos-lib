// Package crud 提供「配置即接口」的通用 CRUD 处理能力，作为所有简单表的标准基线。
// 基线只生成「POST + Action」风格路由（详见仓库根《接口规范.md》），不再提供 Restful 路由。
//
// 设计目标：对一个只含「列表/详情/创建/更新/删除(软删)」的普通表，
// 只需在业务包里定义模型并调用 crud.RegisterActions[Model]，无需手写任何 handler。
// 业务特有的派生字段与副作用通过 Opts[T] 的回调（ToResponse / AfterXxx）注入，基线不感知具体业务。
//
// 约定（与前端 useRestApi 严格对应，统一「POST + Action」）：
//   - 列表 POST /<prefix>/list        meta: page, pageSize, sort, order, keyword, filter, like
//   - 详情 POST /<prefix>/get         data: { id }
//   - 创建 POST /<prefix>/create      data: 创建字段
//   - 更新 POST /<prefix>/update      data: 部分字段（含 id）
//   - 删除 POST /<prefix>/delete      data: { id }（软删除）
//   - 批量创建 POST /<prefix>/batchCreate     data: { items }
//   - 批量删除 POST /<prefix>/batchDelete     data: { ids }
//   - 状态切换 POST /<prefix>/status  data: { id, status }（带副作用，见 ToggleOpts）
//
// 状态切换等扩展能力不内置在基线里，而是由业务包按需以「自定义动作」自行实现并挂载
// （见各业务包的 RegisterV1）。这样基线只负责纯 CRUD，扩展能力下沉到业务包，互不污染。
//
// 列表响应统一为分页结构 { list, pagination }；单条/创建/更新返回 data；删除返回空 data；
// 错误统一由 resp 信封以 code 区分（HTTP 恒为 200）。
package crud

import (
	"time"

	"chaos-go/internal/framework/web"
	"gorm.io/plugin/soft_delete"
)

// BaseModel 标准基字段：所有简单表嵌入它即可获得统一的
// 主键 / 创建时间 / 更新时间 / 逻辑删除。IsDeleted 不对外暴露（json:"-"）。
//
// 逻辑删除交给 gorm.io/plugin/soft_delete 的 flag 模式：
// 查询自动追加 is_deleted = 0、Delete() 自动置 1，Unscoped() 可绕过。
// 因此业务与 crud 基线都不必再手写 is_deleted 条件。
type BaseModel struct {
	ID        int                   `gorm:"primaryKey" json:"ID"`
	CreatedAt time.Time             `json:"CreatedAt"`
	UpdatedAt time.Time             `json:"UpdatedAt"`
	IsDeleted soft_delete.DeletedAt `gorm:"softDelete:flag" json:"-"`
}

// Deleted 判定该记录是否已被软删。
// soft_delete.DeletedAt 底层是 uint，不能直接当 bool 用（!x.IsDeleted 无法编译），
// 统一经本方法判断，嵌入 BaseModel 的模型自动获得。
func (m BaseModel) Deleted() bool {
	return m.IsDeleted != 0
}

// Opts[T] 资源级配置。T 为业务模型类型（如 StandardData），约束了回调与切片的元素类型。
type Opts[T any] struct {
	// Searchable: 参与模糊搜索的「列名」（snake_case），前端按字段名自动转 snake 后透传。
	Searchable []string
	// Sortable: 允许排序的「列名」白名单（snake_case），未列出则忽略 sort 参数。
	Sortable []string
	// Protected: update 动作更新时剔除的业务字段（如 status）。
	// 用于「只能通过带副作用的专属路由改写」的字段，避免通用 update 绕过副作用。
	Protected []string

	// ToResponse: 读方向回调，把 []*T 整批转换为响应形态（通常 DTO 切片）。
	// 未配置则原样返回模型；单条接口（get/create/update）内部包成 1 元素切片复用同一回调并取首元素。
	ToResponse func(rows []*T) any

	// BeforeCreate: 写方向回调，在 ShouldBindJSON 之后、tx.Create 之前执行。
	// 入参为 *T，可就地规范化字段；返回 error 则直接 400 并拒绝创建。
	BeforeCreate func(row *T) error

	// AfterCreate / AfterUpdate / AfterDelete: 写方向回调，在事务内、提交前执行。
	// 入参为 *T；返回 error 则整笔回滚（钩子不成功就不提交）。
	AfterCreate func(row *T) error
	AfterUpdate func(row *T) error
	AfterDelete func(row *T) error

	// V1ListHandler: 可选列表处理器覆盖（仅 v1 动作路由生效）。设置后，POST /<prefix>/list 走该
	// 自定义实现而非基线 listAction，用于 v1 下列表需要派生数据 / 外部副作用（如扫描磁盘）的场景
	// （如项目管理：合并已认领 + 未认领目录）。自定义实现须自行处理分页/搜索/软删过滤，
	// 并返回统一分页结构 { list, pagination }。
	//
	// 注：原 Restful 侧的 ListHandler 已随 crud.Register 下线移除，仅保留 v1 的 V1ListHandler。
	V1ListHandler func(c *web.Context)

	// V1CreateHandler: 可选创建处理器覆盖（仅 v1 动作路由生效）。设置后，POST /<prefix>/create 走该
	// 自定义实现而非基线 createAction。用于「创建不走通用插库」的模型（如笔记：文件优先模型，
	// 须由专用 create 走文件系统创建，基线 BeforeCreate 已将其禁掉）。此时基线 createAction
	// 及其 BeforeCreate 校验均被跳过，由自定义实现自行绑定与返回。
	V1CreateHandler func(c *web.Context)
}
