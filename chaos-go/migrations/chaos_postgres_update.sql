-- chaos_postgres_update.sql
-- 变动日志（增量 DDL）。先应用 chaos_postgres_schema.sql 快照，再按时间顺序应用本文件。
-- chaos_postgres_schema.sql 为手动导出的基准快照，请勿修改；所有结构变动追加到本文件末尾并注明日期与用途。

-- 2026-10-04 引入 gorm.io/plugin/soft_delete（flag 模式）：is_deleted 由 boolean 改为 smallint。
-- 原因：插件以整数 0/1 读写该列，PostgreSQL 下 boolean 列与整数比较/赋值会类型不匹配。
-- 存量数据按 TRUE→1 / FALSE→0 转换；SQLite 本就以整数存储，无需变更。

-- projects / sdk_sources 上存在引用 is_deleted 的部分索引(WHERE is_deleted = FALSE)，
-- 改列类型时 PG 会重算谓词导致 smallint = boolean 报错，故先删除，类型转换后再重建(谓词改为 = 0)。
DROP INDEX IF EXISTS idx_projects_abs_path;
DROP INDEX IF EXISTS idx_sdk_sources_name;

ALTER TABLE public.browser_histories        ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.browser_history_visits   ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.cron_jobs                ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.data_caches              ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.file_links               ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.mqtt_sync_messages       ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.port_forwarding          ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.project_groups           ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.quick_edit_files         ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.quick_edit_snapshots     ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.ssh_connections          ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.standard_datas           ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.task_plans               ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.tasks                    ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.api_logs                 ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.projects                 ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.sdk_sources              ALTER COLUMN is_deleted DROP DEFAULT, ALTER COLUMN is_deleted TYPE smallint USING (COALESCE(is_deleted::int, 0)::smallint), ALTER COLUMN is_deleted SET DEFAULT 0;

-- 重建上面删除的部分索引，谓词改用 smallint 语义(is_deleted = 0 表示未删除)。
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_abs_path ON public.projects USING btree (absolute_path) WHERE (is_deleted = 0);
CREATE UNIQUE INDEX IF NOT EXISTS idx_sdk_sources_name ON public.sdk_sources USING btree (name) WHERE (is_deleted = 0);

-- 2026-10-22 新增数据字典模块（internal/domain/dict）：企业级枚举中心。
-- 设计：dict_type 定义一组枚举的容器（如 存放位置 / 物品分类 / 购入来源），
--       dict_data 为具体可选项，支持父子级（ParentID），其他业务模块按 TypeCode + Value 引用。
-- 软删除统一走 soft_delete flag 模式（is_deleted smallint，由 GORM 插件自动维护）。
CREATE TABLE IF NOT EXISTS public.dict_type (
    id         integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    is_deleted smallint NOT NULL DEFAULT 0,          -- soft_delete flag 模式
    name       varchar(50)  NOT NULL,
    code       varchar(64)  NOT NULL,
    status     boolean NOT NULL DEFAULT true,
    sort       integer NOT NULL DEFAULT 0,
    remark     varchar(255) DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dict_type_code ON public.dict_type (code);
CREATE INDEX IF NOT EXISTS idx_dict_type_deleted ON public.dict_type (is_deleted);

CREATE TABLE IF NOT EXISTS public.dict_data (
    id         integer GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    created_at timestamptz,
    updated_at timestamptz,
    is_deleted smallint NOT NULL DEFAULT 0,          -- soft_delete flag 模式
    type_code  varchar(64)  NOT NULL,
    label      varchar(100) NOT NULL,
    value      varchar(100) NOT NULL,
    parent_id  integer NOT NULL DEFAULT 0,
    sort       integer NOT NULL DEFAULT 0,
    status     boolean NOT NULL DEFAULT true,
    remark     varchar(255) DEFAULT '',
    ext        text DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_dict_data_type ON public.dict_data (type_code);
CREATE INDEX IF NOT EXISTS idx_dict_data_parent ON public.dict_data (parent_id);
CREATE INDEX IF NOT EXISTS idx_dict_data_deleted ON public.dict_data (is_deleted);


