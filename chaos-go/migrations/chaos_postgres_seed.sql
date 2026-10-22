-- chaos_postgres_seed.sql
-- 由 internal/domain/items/seed.go 的 Seed() 转换而来的幂等数据播种 SQL（PostgreSQL）。
--
-- 对应关系：
--   seed.go 的 Seed() 在自动迁移后调用，负责把「物品分类 / 存放位置 / 购入来源」
--   三类枚举收敛进数据字典（dict_type + dict_data）。本文件复刻其「全新库播种」分支：
--     1) 幂等确保 dict_type 存在（item_category / item_location / purchase_source）；
--     2) 仅当某类型在 dict_data 中尚无数据时才播种默认项（对齐 Go 的 dictDataCount 判空）；
--     3) 物品分类 / 存放位置的父子层级通过捕获数据库生成的 ID 重建 parent_id。
--
-- ⚠️ 执行顺序：必须在 chaos_postgres_update.sql 之后运行
--    （该文件已创建 dict_type / dict_data，并已 DROP 遗留表 item_category / item_location）。
--    因此 seed.go 中「从遗留表迁移」的分支在此不再适用，仅保留「默认播种」分支。
-- ⚠️ 物品 Item 对 category_id / location_id 的旧 ID 重映射（seed.go 的 remapItemRef）
--    仅在从遗留表迁移时才有意义；遗留表已删除，全新库无旧引用，故此处省略。
-- 全程幂等，可重复执行。

-- ── 1. 确保 dict_type 存在（幂等）──────────────────────────
INSERT INTO public.dict_type (code, name, status, sort, created_at, updated_at)
SELECT 'item_category', '物品分类', true, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM public.dict_type WHERE code = 'item_category' AND is_deleted = 0);

INSERT INTO public.dict_type (code, name, status, sort, created_at, updated_at)
SELECT 'item_location', '存放位置', true, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM public.dict_type WHERE code = 'item_location' AND is_deleted = 0);

INSERT INTO public.dict_type (code, name, status, sort, created_at, updated_at)
SELECT 'purchase_source', '购入来源', true, 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
WHERE NOT EXISTS (SELECT 1 FROM public.dict_type WHERE code = 'purchase_source' AND is_deleted = 0);

-- ── 2. 物品分类（item_category）：含父子层级 ────────────────
DO $$
DECLARE
  v_cnt integer;
BEGIN
  SELECT count(*) INTO v_cnt FROM public.dict_data WHERE type_code = 'item_category' AND is_deleted = 0;
  IF v_cnt = 0 THEN
    CREATE TEMP TABLE _seed_cat (name text, icon text, parent text) ON COMMIT DROP;
    INSERT INTO _seed_cat (name, icon, parent) VALUES
      ('数码',     'device', ''),
      ('手机',     '',       '数码'),
      ('电脑',     '',       '数码'),
      ('外设',     '',       '数码'),
      ('生活用品', 'home',   ''),
      ('日用品',   '',       '生活用品'),
      ('服饰',     'shirt',  ''),
      ('交通工具', 'bike',   '');

    CREATE TEMP TABLE _seed_cat_id (name text, id integer) ON COMMIT DROP;
    INSERT INTO _seed_cat_id (name, id)
    SELECT s.name, d.id
    FROM (
      INSERT INTO public.dict_data (type_code, label, value, parent_id, sort, status, ext, created_at, updated_at)
      SELECT 'item_category', name, name, 0, 0, true,
             CASE WHEN icon <> '' THEN '{"icon":"' || icon || '"}' ELSE '' END,
             CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
      FROM _seed_cat
      RETURNING id, label
    ) d
    JOIN _seed_cat s ON s.name = d.label;

    UPDATE public.dict_data t
    SET parent_id = p.id
    FROM _seed_cat c
    JOIN _seed_cat_id ci ON ci.name = c.name
    JOIN _seed_cat_id p  ON p.name = c.parent
    WHERE t.id = ci.id AND c.parent <> '';
  END IF;
END $$;

-- ── 3. 存放位置（item_location）：含父子层级 ────────────────
DO $$
DECLARE
  v_cnt integer;
BEGIN
  SELECT count(*) INTO v_cnt FROM public.dict_data WHERE type_code = 'item_location' AND is_deleted = 0;
  IF v_cnt = 0 THEN
    CREATE TEMP TABLE _seed_loc (name text, parent text) ON COMMIT DROP;
    INSERT INTO _seed_loc (name, parent) VALUES
      ('家',         ''),
      ('书房',       '家'),
      ('书桌抽屉',   '书房'),
      ('办公室',     ''),
      ('随身携带',   '');

    CREATE TEMP TABLE _seed_loc_id (name text, id integer) ON COMMIT DROP;
    INSERT INTO _seed_loc_id (name, id)
    SELECT s.name, d.id
    FROM (
      INSERT INTO public.dict_data (type_code, label, value, parent_id, sort, status, ext, created_at, updated_at)
      SELECT 'item_location', name, name, 0, 0, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP
      FROM _seed_loc
      RETURNING id, label
    ) d
    JOIN _seed_loc s ON s.name = d.label;

    UPDATE public.dict_data t
    SET parent_id = p.id
    FROM _seed_loc c
    JOIN _seed_loc_id ci ON ci.name = c.name
    JOIN _seed_loc_id p  ON p.name = c.parent
    WHERE t.id = ci.id AND c.parent <> '';
  END IF;
END $$;

-- ── 4. 购入来源（purchase_source）：扁平，按数组序排序 ───────
DO $$
DECLARE
  v_cnt integer;
BEGIN
  SELECT count(*) INTO v_cnt FROM public.dict_data WHERE type_code = 'purchase_source' AND is_deleted = 0;
  IF v_cnt = 0 THEN
    INSERT INTO public.dict_data (type_code, label, value, parent_id, sort, status, ext, created_at, updated_at)
    VALUES
      ('purchase_source', '淘宝',   '淘宝',   0, 0, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
      ('purchase_source', '京东',   '京东',   0, 1, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
      ('purchase_source', '闲鱼',   '闲鱼',   0, 2, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
      ('purchase_source', '拼多多', '拼多多', 0, 3, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
      ('purchase_source', '线下',   '线下',   0, 4, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP),
      ('purchase_source', '其他',   '其他',   0, 5, true, '', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP);
  END IF;
END $$;
