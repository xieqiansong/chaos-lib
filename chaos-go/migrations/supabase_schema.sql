-- 通用键值对配置/数据表
CREATE TABLE IF NOT EXISTS kv_store (
    id          BIGSERIAL PRIMARY KEY,
    "key"       VARCHAR(255) NOT NULL,
    "value"     TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT uk_kv_store_key UNIQUE ("key")
);

-- 字段注释
COMMENT ON TABLE kv_store IS '通用键值对配置/数据表';
COMMENT ON COLUMN kv_store.id IS '主键ID';
COMMENT ON COLUMN kv_store."key" IS '配置键';
COMMENT ON COLUMN kv_store."value" IS '配置值';
COMMENT ON COLUMN kv_store.created_at IS '创建时间';
COMMENT ON COLUMN kv_store.updated_at IS '更新时间';