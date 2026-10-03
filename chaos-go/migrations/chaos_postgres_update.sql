-- chaos_postgres_update.sql
-- 变动日志（增量 DDL）。先应用 chaos_postgres_schema.sql 快照，再按时间顺序应用本文件。
-- chaos_postgres_schema.sql 为手动导出的基准快照，请勿修改；所有结构变动追加到本文件末尾并注明日期与用途。

-- 2026-10-04 引入 gorm.io/plugin/soft_delete（flag 模式）：is_deleted 由 boolean 改为 smallint。
-- 原因：插件以整数 0/1 读写该列，PostgreSQL 下 boolean 列与整数比较/赋值会类型不匹配。
-- 存量数据按 TRUE→1 / FALSE→0 转换；SQLite 本就以整数存储，无需变更。
ALTER TABLE public.browser_histories        ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.browser_history_visits   ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.cron_jobs                ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.data_caches              ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.file_links               ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.mqtt_sync_messages       ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.port_forwarding          ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.project_groups           ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.projects                 ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.quick_edit_files         ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.quick_edit_snapshots     ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.sdk_sources              ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.ssh_connections          ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.standard_datas           ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.task_plans               ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;
ALTER TABLE public.tasks                    ALTER COLUMN is_deleted TYPE smallint USING CASE WHEN is_deleted THEN 1 ELSE 0 END, ALTER COLUMN is_deleted SET DEFAULT 0;

