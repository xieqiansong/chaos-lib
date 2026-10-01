--
-- PostgreSQL database dump
--

\restrict 48Yjo12VB9FAYfENoGa79cFLuPTwltL9Xb8c9u3vH7VAbNUJ3ubbUmqJhycSDuY

-- Dumped from database version 17.10 (Debian 17.10-1.pgdg12+1)
-- Dumped by pg_dump version 17.10 (Debian 17.10-1.pgdg12+1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', FALSE);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS vector WITH SCHEMA public;


--
-- Name: EXTENSION vector; Type: COMMENT; Schema: -; Owner:
--

COMMENT ON EXTENSION vector IS 'vector data type and ivfflat and hnsw access methods';


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: bookmarks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.bookmarks
(
    id         text NOT NULL,
    parent_id  text,
    title      text,
    url        text,
    is_folder  boolean,
    sort_index bigint,
    date_added bigint
);


ALTER TABLE public.bookmarks
    OWNER TO postgres;

--
-- Name: TABLE bookmarks; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.bookmarks IS '书签表';


--
-- Name: COLUMN bookmarks.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.id IS '书签唯一标识';


--
-- Name: COLUMN bookmarks.parent_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.parent_id IS '父级书签/文件夹ID，NULL 表示顶级';


--
-- Name: COLUMN bookmarks.title; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.title IS '书签标题';


--
-- Name: COLUMN bookmarks.url; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.url IS '书签地址';


--
-- Name: COLUMN bookmarks.is_folder; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.is_folder IS '是否为文件夹';


--
-- Name: COLUMN bookmarks.sort_index; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.sort_index IS '排序序号';


--
-- Name: COLUMN bookmarks.date_added; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.bookmarks.date_added IS '添加时间戳';


--
-- Name: browser_histories; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.browser_histories
(
    id              character varying NOT NULL,
    last_visit_time numeric,
    title           text,
    type_count      bigint,
    url             text,
    visit_count     bigint,
    is_deleted      boolean DEFAULT FALSE
);


ALTER TABLE public.browser_histories
    OWNER TO postgres;

--
-- Name: TABLE browser_histories; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.browser_histories IS '浏览器历史记录表';


--
-- Name: COLUMN browser_histories.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.id IS '历史记录唯一标识';


--
-- Name: COLUMN browser_histories.last_visit_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.last_visit_time IS '最后访问时间戳';


--
-- Name: COLUMN browser_histories.title; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.title IS '页面标题';


--
-- Name: COLUMN browser_histories.type_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.type_count IS '类型统计数';


--
-- Name: COLUMN browser_histories.url; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.url IS '页面地址';


--
-- Name: COLUMN browser_histories.visit_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.visit_count IS '访问次数';


--
-- Name: COLUMN browser_histories.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_histories.is_deleted IS '软删除标记';


--
-- Name: browser_history_visits; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.browser_history_visits
(
    id                 character varying NOT NULL,
    history_id         text,
    is_local           boolean,
    referring_visit_id text,
    transition         text,
    visit_id           text,
    visit_time         numeric,
    is_deleted         boolean DEFAULT FALSE
);


ALTER TABLE public.browser_history_visits
    OWNER TO postgres;

--
-- Name: TABLE browser_history_visits; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.browser_history_visits IS '浏览器历史访问明细表';


--
-- Name: COLUMN browser_history_visits.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.id IS '访问记录唯一标识';


--
-- Name: COLUMN browser_history_visits.history_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.history_id IS '所属历史记录ID';


--
-- Name: COLUMN browser_history_visits.is_local; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.is_local IS '是否本地访问';


--
-- Name: COLUMN browser_history_visits.referring_visit_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.referring_visit_id IS '来源访问ID';


--
-- Name: COLUMN browser_history_visits.transition; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.transition IS '访问过渡类型';


--
-- Name: COLUMN browser_history_visits.visit_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.visit_id IS '访问ID';


--
-- Name: COLUMN browser_history_visits.visit_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.visit_time IS '访问时间戳';


--
-- Name: COLUMN browser_history_visits.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.browser_history_visits.is_deleted IS '软删除标记';


--
-- Name: cron_job_runs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.cron_job_runs
(
    id          bigint NOT NULL,
    job_id      bigint,
    started_at  timestamp with time zone,
    finished_at timestamp with time zone,
    success     boolean,
    output      text,
    error       text,
    created_at  timestamp with time zone
);


ALTER TABLE public.cron_job_runs
    OWNER TO postgres;

--
-- Name: TABLE cron_job_runs; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.cron_job_runs IS '定时任务运行记录表';


--
-- Name: COLUMN cron_job_runs.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.id IS '运行记录唯一标识';


--
-- Name: COLUMN cron_job_runs.job_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.job_id IS '所属定时任务ID';


--
-- Name: COLUMN cron_job_runs.started_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.started_at IS '开始时间';


--
-- Name: COLUMN cron_job_runs.finished_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.finished_at IS '结束时间';


--
-- Name: COLUMN cron_job_runs.success; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.success IS '是否成功';


--
-- Name: COLUMN cron_job_runs.output; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.output IS '运行输出';


--
-- Name: COLUMN cron_job_runs.error; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.error IS '错误信息';


--
-- Name: COLUMN cron_job_runs.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_job_runs.created_at IS '创建时间';


--
-- Name: cron_job_runs_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.cron_job_runs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.cron_job_runs_id_seq OWNER TO postgres;

--
-- Name: cron_job_runs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.cron_job_runs_id_seq OWNED BY public.cron_job_runs.id;


--
-- Name: cron_jobs; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.cron_jobs
(
    id            bigint NOT NULL,
    name          text,
    cron_expr     text,
    action_type   text,
    action_config text,
    enabled       boolean,
    timeout_sec   bigint,
    last_run_at   timestamp with time zone,
    last_status   text,
    is_deleted    boolean DEFAULT FALSE,
    created_at    timestamp with time zone,
    updated_at    timestamp with time zone
);


ALTER TABLE public.cron_jobs
    OWNER TO postgres;

--
-- Name: TABLE cron_jobs; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.cron_jobs IS '定时任务表';


--
-- Name: COLUMN cron_jobs.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.id IS '任务唯一标识';


--
-- Name: COLUMN cron_jobs.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.name IS '任务名称';


--
-- Name: COLUMN cron_jobs.cron_expr; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.cron_expr IS 'cron 表达式';


--
-- Name: COLUMN cron_jobs.action_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.action_type IS '动作类型';


--
-- Name: COLUMN cron_jobs.action_config; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.action_config IS '动作配置';


--
-- Name: COLUMN cron_jobs.enabled; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.enabled IS '是否启用';


--
-- Name: COLUMN cron_jobs.timeout_sec; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.timeout_sec IS '超时秒数';


--
-- Name: COLUMN cron_jobs.last_run_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.last_run_at IS '上次运行时间';


--
-- Name: COLUMN cron_jobs.last_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.last_status IS '上次运行状态';


--
-- Name: COLUMN cron_jobs.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.is_deleted IS '软删除标记';


--
-- Name: COLUMN cron_jobs.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.created_at IS '创建时间';


--
-- Name: COLUMN cron_jobs.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.cron_jobs.updated_at IS '更新时间';


--
-- Name: cron_jobs_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.cron_jobs_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.cron_jobs_id_seq OWNER TO postgres;

--
-- Name: cron_jobs_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.cron_jobs_id_seq OWNED BY public.cron_jobs.id;


--
-- Name: data_caches; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.data_caches
(
    id          bigint NOT NULL,
    created_at  timestamp with time zone,
    updated_at  timestamp with time zone,
    is_deleted  boolean DEFAULT FALSE,
    category    text,
    key         text,
    value       bytea,
    expire_at   timestamp with time zone,
    compression text,
    data_type   text,
    value_len   bigint  DEFAULT 0,
    value_md5   text
);


ALTER TABLE public.data_caches
    OWNER TO postgres;

--
-- Name: TABLE data_caches; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.data_caches IS '数据缓存表：按 category+key 存储程序内部缓存数据';


--
-- Name: COLUMN data_caches.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.id IS '主键 ID';


--
-- Name: COLUMN data_caches.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.created_at IS '创建时间';


--
-- Name: COLUMN data_caches.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.updated_at IS '更新时间';


--
-- Name: COLUMN data_caches.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.is_deleted IS '软删除标记（TRUE 表示已删除）';


--
-- Name: COLUMN data_caches.category; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.category IS '分类（缓存命名空间）';


--
-- Name: COLUMN data_caches.key; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.key IS '缓存键';


--
-- Name: COLUMN data_caches.value; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.value IS '缓存值（二进制内容，前端按 base64 编解码）';


--
-- Name: COLUMN data_caches.expire_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.expire_at IS '过期时间';


--
-- Name: COLUMN data_caches.compression; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.compression IS '压缩算法（预留字段）';


--
-- Name: COLUMN data_caches.data_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.data_type IS '数据类型（预留，用于统计）';


--
-- Name: COLUMN data_caches.value_len; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.value_len IS '值长度（预留，用于统计）';


--
-- Name: COLUMN data_caches.value_md5; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.data_caches.value_md5 IS '值 MD5 摘要（预留，用于统计）';


--
-- Name: data_caches_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.data_caches_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.data_caches_id_seq OWNER TO postgres;

--
-- Name: data_caches_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.data_caches_id_seq OWNED BY public.data_caches.id;


--
-- Name: file_links; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.file_links
(
    id          integer NOT NULL,
    source_path text,
    target_path text,
    status      boolean,
    remark      text,
    is_deleted  boolean DEFAULT FALSE,
    sort        bigint  DEFAULT 0,
    created_at  timestamp with time zone,
    updated_at  timestamp with time zone
);


ALTER TABLE public.file_links
    OWNER TO postgres;

--
-- Name: TABLE file_links; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.file_links IS '文件软链接表';


--
-- Name: COLUMN file_links.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.id IS '链接唯一标识';


--
-- Name: COLUMN file_links.source_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.source_path IS '源路径';


--
-- Name: COLUMN file_links.target_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.target_path IS '目标路径';


--
-- Name: COLUMN file_links.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.status IS '状态';


--
-- Name: COLUMN file_links.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.remark IS '备注';


--
-- Name: COLUMN file_links.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.is_deleted IS '软删除标记';


--
-- Name: COLUMN file_links.sort; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.sort IS '排序序号';


--
-- Name: COLUMN file_links.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.created_at IS '创建时间';


--
-- Name: COLUMN file_links.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.file_links.updated_at IS '更新时间';


--
-- Name: file_link_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.file_links
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.file_link_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: mqtt_sync_messages; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.mqtt_sync_messages
(
    id         bigint NOT NULL,
    msg_id     character varying(64),
    node_id    character varying(64),
    channel    character varying(64),
    payload    text,
    created_at timestamp with time zone,
    is_deleted boolean DEFAULT FALSE,
    updated_at timestamp with time zone
);


ALTER TABLE public.mqtt_sync_messages
    OWNER TO postgres;

--
-- Name: TABLE mqtt_sync_messages; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.mqtt_sync_messages IS 'MQTT 同步消息表';


--
-- Name: COLUMN mqtt_sync_messages.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.id IS '消息唯一标识';


--
-- Name: COLUMN mqtt_sync_messages.msg_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.msg_id IS '消息ID';


--
-- Name: COLUMN mqtt_sync_messages.node_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.node_id IS '节点ID';


--
-- Name: COLUMN mqtt_sync_messages.channel; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.channel IS '频道';


--
-- Name: COLUMN mqtt_sync_messages.payload; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.payload IS '消息内容';


--
-- Name: COLUMN mqtt_sync_messages.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.created_at IS '创建时间';


--
-- Name: COLUMN mqtt_sync_messages.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.is_deleted IS '软删除标记';


--
-- Name: COLUMN mqtt_sync_messages.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_messages.updated_at IS '更新时间';


--
-- Name: mqtt_sync_messages_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.mqtt_sync_messages_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.mqtt_sync_messages_id_seq OWNER TO postgres;

--
-- Name: mqtt_sync_messages_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.mqtt_sync_messages_id_seq OWNED BY public.mqtt_sync_messages.id;


--
-- Name: mqtt_sync_nodes; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.mqtt_sync_nodes
(
    id         bigint NOT NULL,
    node_id    character varying(64),
    created_at timestamp with time zone
);


ALTER TABLE public.mqtt_sync_nodes
    OWNER TO postgres;

--
-- Name: TABLE mqtt_sync_nodes; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.mqtt_sync_nodes IS 'MQTT 同步节点表';


--
-- Name: COLUMN mqtt_sync_nodes.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_nodes.id IS '节点唯一标识';


--
-- Name: COLUMN mqtt_sync_nodes.node_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_nodes.node_id IS '节点ID';


--
-- Name: COLUMN mqtt_sync_nodes.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.mqtt_sync_nodes.created_at IS '创建时间';


--
-- Name: mqtt_sync_nodes_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.mqtt_sync_nodes_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.mqtt_sync_nodes_id_seq OWNER TO postgres;

--
-- Name: mqtt_sync_nodes_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.mqtt_sync_nodes_id_seq OWNED BY public.mqtt_sync_nodes.id;


--
-- Name: nowcoder_questions; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.nowcoder_questions
(
    question_id    bigint                                 NOT NULL,
    uuid           text                                   NOT NULL,
    title          text                                   NOT NULL,
    difficulty     integer,
    exam_count     integer,
    knowledge      text,
    company_name   text,
    last_exam_time text,
    created_at     timestamp with time zone DEFAULT NOW() NOT NULL,
    qtype          integer                  DEFAULT 0     NOT NULL
);


ALTER TABLE public.nowcoder_questions
    OWNER TO postgres;

--
-- Name: TABLE nowcoder_questions; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.nowcoder_questions IS '牛客网题目表';


--
-- Name: COLUMN nowcoder_questions.question_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.question_id IS '题目ID';


--
-- Name: COLUMN nowcoder_questions.uuid; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.uuid IS '唯一标识';


--
-- Name: COLUMN nowcoder_questions.title; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.title IS '题目标题';


--
-- Name: COLUMN nowcoder_questions.difficulty; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.difficulty IS '难度';


--
-- Name: COLUMN nowcoder_questions.exam_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.exam_count IS '练习次数';


--
-- Name: COLUMN nowcoder_questions.knowledge; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.knowledge IS '知识点';


--
-- Name: COLUMN nowcoder_questions.company_name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.company_name IS '公司名称';


--
-- Name: COLUMN nowcoder_questions.last_exam_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.last_exam_time IS '上次练习时间';


--
-- Name: COLUMN nowcoder_questions.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.created_at IS '创建时间';


--
-- Name: COLUMN nowcoder_questions.qtype; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.nowcoder_questions.qtype IS '题目类型';


--
-- Name: port_forwarding; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.port_forwarding
(
    id                integer NOT NULL,
    name              text,
    port              bigint,
    target_host       text,
    target_port       bigint,
    status            boolean,
    is_deleted        boolean DEFAULT FALSE,
    ssh_connection_id bigint,
    remark            text,
    direction         text,
    bind_address      text,
    created_at        timestamp with time zone,
    updated_at        timestamp with time zone,
    last_error        text
);


ALTER TABLE public.port_forwarding
    OWNER TO postgres;

--
-- Name: TABLE port_forwarding; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.port_forwarding IS '端口转发配置表';


--
-- Name: COLUMN port_forwarding.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.id IS '转发唯一标识';


--
-- Name: COLUMN port_forwarding.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.name IS '名称';


--
-- Name: COLUMN port_forwarding.port; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.port IS '本地监听端口';


--
-- Name: COLUMN port_forwarding.target_host; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.target_host IS '目标主机';


--
-- Name: COLUMN port_forwarding.target_port; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.target_port IS '目标端口';


--
-- Name: COLUMN port_forwarding.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.status IS '状态';


--
-- Name: COLUMN port_forwarding.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.is_deleted IS '软删除标记';


--
-- Name: COLUMN port_forwarding.ssh_connection_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.ssh_connection_id IS '关联 SSH 连接ID';


--
-- Name: COLUMN port_forwarding.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.remark IS '备注';


--
-- Name: COLUMN port_forwarding.direction; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.direction IS '转发方向';


--
-- Name: COLUMN port_forwarding.bind_address; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.bind_address IS '绑定地址';


--
-- Name: COLUMN port_forwarding.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.created_at IS '创建时间';


--
-- Name: COLUMN port_forwarding.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.updated_at IS '更新时间';


--
-- Name: COLUMN port_forwarding.last_error; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.port_forwarding.last_error IS '上次错误信息';


--
-- Name: port_forwarding_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.port_forwarding
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.port_forwarding_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: project_groups; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.project_groups
(
    id             integer NOT NULL,
    name           text,
    absolute_path  text,
    remark         text,
    created_at     timestamp with time zone,
    updated_at     timestamp with time zone,
    is_deleted     boolean DEFAULT FALSE,
    order_num      bigint  DEFAULT 0,
    is_recycle_bin boolean DEFAULT FALSE
);


ALTER TABLE public.project_groups
    OWNER TO postgres;

--
-- Name: TABLE project_groups; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.project_groups IS '项目组表';


--
-- Name: COLUMN project_groups.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.id IS '项目组唯一标识';


--
-- Name: COLUMN project_groups.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.name IS '项目组名称';


--
-- Name: COLUMN project_groups.absolute_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.absolute_path IS '项目组根目录绝对路径，其下项目通过相对路径定位';


--
-- Name: COLUMN project_groups.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.remark IS '备注';


--
-- Name: COLUMN project_groups.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.created_at IS '创建时间';


--
-- Name: COLUMN project_groups.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.updated_at IS '更新时间';


--
-- Name: COLUMN project_groups.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.is_deleted IS '软删除标记';


--
-- Name: COLUMN project_groups.order_num; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.order_num IS '排序序号，越小越靠前';


--
-- Name: COLUMN project_groups.is_recycle_bin; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.project_groups.is_recycle_bin IS '是否为回收站';


--
-- Name: project_groups_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.project_groups
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.project_groups_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: projects; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.projects
(
    id               integer NOT NULL,
    group_id         bigint,
    name             text,
    absolute_path    text,
    relative_path    text,
    git_url          text,
    remark           text,
    last_accessed_at timestamp with time zone,
    created_at       timestamp with time zone,
    updated_at       timestamp with time zone,
    is_deleted       boolean DEFAULT FALSE
);


ALTER TABLE public.projects
    OWNER TO postgres;

--
-- Name: TABLE projects; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.projects IS '项目表';


--
-- Name: COLUMN projects.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.id IS '项目唯一标识';


--
-- Name: COLUMN projects.group_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.group_id IS '所属项目组ID';


--
-- Name: COLUMN projects.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.name IS '项目名称';


--
-- Name: COLUMN projects.absolute_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.absolute_path IS '项目绝对路径，等于 组绝对路径 + 相对路径';


--
-- Name: COLUMN projects.relative_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.relative_path IS '相对所属项目组根目录的路径';


--
-- Name: COLUMN projects.git_url; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.git_url IS 'Git 仓库地址';


--
-- Name: COLUMN projects.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.remark IS '备注';


--
-- Name: COLUMN projects.last_accessed_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.last_accessed_at IS '上次访问时间';


--
-- Name: COLUMN projects.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.created_at IS '创建时间';


--
-- Name: COLUMN projects.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.updated_at IS '更新时间';


--
-- Name: COLUMN projects.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.projects.is_deleted IS '软删除标记';


--
-- Name: projects_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.projects
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.projects_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: quick_edit_files; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.quick_edit_files
(
    id         integer NOT NULL,
    name       text,
    file_path  text,
    remark     text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_deleted boolean                  DEFAULT FALSE
);


ALTER TABLE public.quick_edit_files
    OWNER TO postgres;

--
-- Name: TABLE quick_edit_files; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.quick_edit_files IS '快捷编辑文件表';


--
-- Name: COLUMN quick_edit_files.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.id IS '文件唯一标识';


--
-- Name: COLUMN quick_edit_files.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.name IS '文件名称';


--
-- Name: COLUMN quick_edit_files.file_path; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.file_path IS '文件路径';


--
-- Name: COLUMN quick_edit_files.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.remark IS '备注';


--
-- Name: COLUMN quick_edit_files.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.created_at IS '创建时间';


--
-- Name: COLUMN quick_edit_files.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.updated_at IS '更新时间';


--
-- Name: COLUMN quick_edit_files.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_files.is_deleted IS '软删除标记';


--
-- Name: quick_edit_files_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.quick_edit_files
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.quick_edit_files_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: quick_edit_snapshots; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.quick_edit_snapshots
(
    id         integer NOT NULL,
    file_id    bigint,
    content    text,
    size_bytes bigint,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_deleted boolean                  DEFAULT FALSE
);


ALTER TABLE public.quick_edit_snapshots
    OWNER TO postgres;

--
-- Name: TABLE quick_edit_snapshots; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.quick_edit_snapshots IS '快捷编辑快照表';


--
-- Name: COLUMN quick_edit_snapshots.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.id IS '快照唯一标识';


--
-- Name: COLUMN quick_edit_snapshots.file_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.file_id IS '所属文件ID';


--
-- Name: COLUMN quick_edit_snapshots.content; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.content IS '文件内容快照';


--
-- Name: COLUMN quick_edit_snapshots.size_bytes; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.size_bytes IS '快照字节大小';


--
-- Name: COLUMN quick_edit_snapshots.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.created_at IS '创建时间';


--
-- Name: COLUMN quick_edit_snapshots.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.quick_edit_snapshots.is_deleted IS '软删除标记';


--
-- Name: quick_edit_snapshots_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.quick_edit_snapshots
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.quick_edit_snapshots_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: sdk_sources; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.sdk_sources
(
    id         integer NOT NULL,
    name       text,
    sources    jsonb,
    current    text,
    enabled    boolean DEFAULT TRUE,
    note       text,
    is_deleted boolean DEFAULT FALSE
);


ALTER TABLE public.sdk_sources
    OWNER TO postgres;

--
-- Name: TABLE sdk_sources; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.sdk_sources IS 'SDK 来源表：一行代表一个 SDK 类型(jdk/maven/python/...)，sources 存其来源数组';


--
-- Name: COLUMN sdk_sources.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.id IS 'SDK 类型唯一标识';


--
-- Name: COLUMN sdk_sources.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.name IS 'SDK 类型唯一标识，作 GET /sdks 的 map key 与切换 :name';


--
-- Name: COLUMN sdk_sources.sources; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.sources IS '来源 JSON 数组，元素 {kind:repo|single, root:绝对路径}；repo 与 single 可混合';


--
-- Name: COLUMN sdk_sources.current; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.current IS '当前启用版本的绝对路径；单值即保证同时仅一个版本启用';


--
-- Name: COLUMN sdk_sources.enabled; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.enabled IS '该 SDK 类型是否启用，禁用后不参与版本读取';


--
-- Name: COLUMN sdk_sources.note; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.note IS '备注';


--
-- Name: COLUMN sdk_sources.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.sdk_sources.is_deleted IS '软删除标记';


--
-- Name: sdk_sources_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.sdk_sources
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.sdk_sources_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: ssh_connections; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.ssh_connections
(
    id          bigint NOT NULL,
    name        text,
    host        text,
    port        bigint,
    username    text,
    auth_type   text,
    password    text,
    private_key text,
    passphrase  text,
    remark      text,
    created_at  timestamp with time zone,
    updated_at  timestamp with time zone,
    is_deleted  boolean DEFAULT FALSE
);


ALTER TABLE public.ssh_connections
    OWNER TO postgres;

--
-- Name: TABLE ssh_connections; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.ssh_connections IS 'SSH 连接配置表';


--
-- Name: COLUMN ssh_connections.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.id IS '连接唯一标识';


--
-- Name: COLUMN ssh_connections.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.name IS '连接名称';


--
-- Name: COLUMN ssh_connections.host; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.host IS '主机地址';


--
-- Name: COLUMN ssh_connections.port; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.port IS '端口';


--
-- Name: COLUMN ssh_connections.username; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.username IS '用户名';


--
-- Name: COLUMN ssh_connections.auth_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.auth_type IS '认证类型';


--
-- Name: COLUMN ssh_connections.password; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.password IS '密码';


--
-- Name: COLUMN ssh_connections.private_key; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.private_key IS '私钥';


--
-- Name: COLUMN ssh_connections.passphrase; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.passphrase IS '私钥口令';


--
-- Name: COLUMN ssh_connections.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.remark IS '备注';


--
-- Name: COLUMN ssh_connections.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.created_at IS '创建时间';


--
-- Name: COLUMN ssh_connections.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.updated_at IS '更新时间';


--
-- Name: COLUMN ssh_connections.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.ssh_connections.is_deleted IS '软删除标记';


--
-- Name: ssh_connections_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.ssh_connections_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.ssh_connections_id_seq OWNER TO postgres;

--
-- Name: ssh_connections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.ssh_connections_id_seq OWNED BY public.ssh_connections.id;


--
-- Name: standard_datas; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.standard_datas
(
    id           bigint NOT NULL,
    created_at   timestamp with time zone,
    updated_at   timestamp with time zone,
    is_deleted   boolean DEFAULT FALSE,
    name         text,
    code         text,
    description  text,
    category     text,
    quantity     bigint,
    price        numeric,
    enabled      boolean,
    config       text,
    effective_at timestamp with time zone,
    sort         bigint  DEFAULT 0
);


ALTER TABLE public.standard_datas
    OWNER TO postgres;

--
-- Name: TABLE standard_datas; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.standard_datas IS '标准数据表';


--
-- Name: COLUMN standard_datas.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.id IS '数据唯一标识';


--
-- Name: COLUMN standard_datas.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.created_at IS '创建时间';


--
-- Name: COLUMN standard_datas.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.updated_at IS '更新时间';


--
-- Name: COLUMN standard_datas.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.is_deleted IS '软删除标记';


--
-- Name: COLUMN standard_datas.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.name IS '名称';


--
-- Name: COLUMN standard_datas.code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.code IS '编码';


--
-- Name: COLUMN standard_datas.description; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.description IS '描述';


--
-- Name: COLUMN standard_datas.category; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.category IS '分类';


--
-- Name: COLUMN standard_datas.quantity; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.quantity IS '数量';


--
-- Name: COLUMN standard_datas.price; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.price IS '价格';


--
-- Name: COLUMN standard_datas.enabled; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.enabled IS '是否启用';


--
-- Name: COLUMN standard_datas.config; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.config IS '配置';


--
-- Name: COLUMN standard_datas.effective_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.effective_at IS '生效时间';


--
-- Name: COLUMN standard_datas.sort; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.standard_datas.sort IS '排序序号';


--
-- Name: standard_datas_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

CREATE SEQUENCE public.standard_datas_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER SEQUENCE public.standard_datas_id_seq OWNER TO postgres;

--
-- Name: standard_datas_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: postgres
--

ALTER SEQUENCE public.standard_datas_id_seq OWNED BY public.standard_datas.id;


--
-- Name: task_plan_old; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.task_plan_old
(
    id               character varying(36)                                      NOT NULL,
    pid              character varying(36)                                      NOT NULL,
    name             character varying(100)                                     NOT NULL,
    code             character varying(100)                                     NOT NULL,
    type             character varying(32) DEFAULT 'DEFAULT'::character varying NOT NULL,
    priority         integer               DEFAULT 0                            NOT NULL,
    remarks          text,
    sort_code        integer               DEFAULT 0                            NOT NULL,
    start_time       timestamp without time zone,
    end_time         timestamp without time zone,
    status           character varying(32) DEFAULT 'DEFAULT'::character varying NOT NULL,
    complete_time    timestamp without time zone,
    url_link         character varying(512),
    create_by        character varying(36),
    update_by        character varying(36),
    create_date      timestamp without time zone,
    update_date      timestamp without time zone,
    data_status      integer               DEFAULT 0                            NOT NULL,
    proficiency      integer               DEFAULT 0                            NOT NULL,
    last_review_time timestamp without time zone,
    review_record    text
);


ALTER TABLE public.task_plan_old
    OWNER TO postgres;

--
-- Name: TABLE task_plan_old; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.task_plan_old IS '任务计划';


--
-- Name: COLUMN task_plan_old.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.id IS '主键';


--
-- Name: COLUMN task_plan_old.pid; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.pid IS '父ID';


--
-- Name: COLUMN task_plan_old.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.name IS '名称';


--
-- Name: COLUMN task_plan_old.code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.code IS '编码';


--
-- Name: COLUMN task_plan_old.type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.type IS '类型';


--
-- Name: COLUMN task_plan_old.priority; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.priority IS '优先级';


--
-- Name: COLUMN task_plan_old.remarks; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.remarks IS '备注';


--
-- Name: COLUMN task_plan_old.sort_code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.sort_code IS '排序';


--
-- Name: COLUMN task_plan_old.start_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.start_time IS '开始时间';


--
-- Name: COLUMN task_plan_old.end_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.end_time IS '结束时间';


--
-- Name: COLUMN task_plan_old.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.status IS '状态';


--
-- Name: COLUMN task_plan_old.complete_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.complete_time IS '完成时间';


--
-- Name: COLUMN task_plan_old.url_link; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.url_link IS '链接';


--
-- Name: COLUMN task_plan_old.create_by; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.create_by IS '创建人';


--
-- Name: COLUMN task_plan_old.update_by; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.update_by IS '更新人';


--
-- Name: COLUMN task_plan_old.create_date; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.create_date IS '创建时间';


--
-- Name: COLUMN task_plan_old.update_date; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.update_date IS '更新时间';


--
-- Name: COLUMN task_plan_old.data_status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.data_status IS '数据状态';


--
-- Name: COLUMN task_plan_old.proficiency; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.proficiency IS '熟练度';


--
-- Name: COLUMN task_plan_old.last_review_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.last_review_time IS '上次复习时间';


--
-- Name: COLUMN task_plan_old.review_record; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plan_old.review_record IS '复习记录';


--
-- Name: task_plans; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.task_plans
(
    id                  integer NOT NULL,
    parent_id           bigint,
    name                text,
    status              text                     DEFAULT 'created'::character varying,
    plan_type           text                     DEFAULT 'todo'::character varying,
    cron_expr           text,
    fsrs_stability      numeric                  DEFAULT 0,
    fsrs_difficulty     numeric                  DEFAULT 0,
    fsrs_reps           bigint                   DEFAULT 0,
    fsrs_lapses         bigint                   DEFAULT 0,
    fsrs_last_review_at timestamp with time zone,
    remark              text,
    link                text,
    created_at          timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at          timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_deleted          boolean                  DEFAULT FALSE,
    order_num           integer                  DEFAULT 0,
    priority            integer                  DEFAULT 5,
    fsrs_state          integer                  DEFAULT 0,
    fsrs_learning_steps integer                  DEFAULT 0,
    content_size        bigint,
    is_suspended        boolean                  DEFAULT FALSE,
    code                text,
    interval_days       bigint,
    interval_hour       bigint,
    interval_minute     bigint,
    task_count          bigint                   DEFAULT 0,
    completed_count     bigint                   DEFAULT 0,
    total_study_time    bigint                   DEFAULT 0,
    last_completed_at   timestamp with time zone,
    raw_link            text
);


ALTER TABLE public.task_plans
    OWNER TO postgres;

--
-- Name: TABLE task_plans; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.task_plans IS '任务计划表';


--
-- Name: COLUMN task_plans.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.id IS '任务计划唯一标识';


--
-- Name: COLUMN task_plans.parent_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.parent_id IS '父级任务计划ID，用于树形目录结构。NULL 表示顶级目录';


--
-- Name: COLUMN task_plans.name; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.name IS '任务计划名称';


--
-- Name: COLUMN task_plans.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.status IS '任务计划状态: created(已创建) / started(已开始) / completed(已完成) / archived(已归档)';


--
-- Name: COLUMN task_plans.plan_type; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.plan_type IS '任务类型: todo(待办任务) / cron(周期重复任务) / interval(间隔任务)';


--
-- Name: COLUMN task_plans.cron_expr; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.cron_expr IS 'cron: cron 表达式，plan_type=cron 时必填';


--
-- Name: COLUMN task_plans.fsrs_stability; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_stability IS 'fsrs: 记忆稳定性，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.fsrs_difficulty; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_difficulty IS 'fsrs: 卡片难度，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.fsrs_reps; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_reps IS 'fsrs: 复习次数，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.fsrs_lapses; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_lapses IS 'fsrs: 遗忘次数，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.fsrs_last_review_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_last_review_at IS 'fsrs: 上次复习时间，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.remark IS '备注';


--
-- Name: COLUMN task_plans.link; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.link IS '关联链接';


--
-- Name: COLUMN task_plans.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.created_at IS '创建时间';


--
-- Name: COLUMN task_plans.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.updated_at IS '更新时间';


--
-- Name: COLUMN task_plans.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.is_deleted IS '软删除标记';


--
-- Name: COLUMN task_plans.order_num; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.order_num IS '排序序号，越小越靠前';


--
-- Name: COLUMN task_plans.priority; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.priority IS '优先级';


--
-- Name: COLUMN task_plans.fsrs_state; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_state IS 'fsrs: 卡片状态，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.fsrs_learning_steps; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.fsrs_learning_steps IS 'fsrs: 学习步骤，plan_type=interval 时使用';


--
-- Name: COLUMN task_plans.content_size; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.content_size IS '内容大小';


--
-- Name: COLUMN task_plans.is_suspended; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.is_suspended IS '是否暂停';


--
-- Name: COLUMN task_plans.code; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.code IS '编码';


--
-- Name: COLUMN task_plans.interval_days; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.interval_days IS 'interval: 间隔天数';


--
-- Name: COLUMN task_plans.interval_hour; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.interval_hour IS 'interval: 间隔小时数';


--
-- Name: COLUMN task_plans.interval_minute; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.interval_minute IS 'interval: 间隔分钟数';


--
-- Name: COLUMN task_plans.task_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.task_count IS '任务总数（子任务计数）';


--
-- Name: COLUMN task_plans.completed_count; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.completed_count IS '已完成任务数';


--
-- Name: COLUMN task_plans.total_study_time; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.total_study_time IS '累计学习时长（秒）';


--
-- Name: COLUMN task_plans.last_completed_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.last_completed_at IS '上次完成时间';


--
-- Name: COLUMN task_plans.raw_link; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.task_plans.raw_link IS '原始链接';


--
-- Name: task_plans_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.task_plans
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.task_plans_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: tasks; Type: TABLE; Schema: public; Owner: postgres
--

CREATE TABLE public.tasks
(
    id             integer NOT NULL,
    plan_id        bigint,
    status         text                     DEFAULT 'pending'::text,
    started_at     timestamp with time zone,
    completed_at   timestamp with time zone,
    deadline       timestamp with time zone,
    remark         text,
    created_at     timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    is_deleted     boolean                  DEFAULT FALSE,
    scheduled_date timestamp with time zone,
    rating         bigint,
    updated_at     timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


ALTER TABLE public.tasks
    OWNER TO postgres;

--
-- Name: TABLE tasks; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON TABLE public.tasks IS '任务表（每次执行的实例记录）';


--
-- Name: COLUMN tasks.id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.id IS '任务唯一标识';


--
-- Name: COLUMN tasks.plan_id; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.plan_id IS '所属任务计划ID';


--
-- Name: COLUMN tasks.status; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.status IS '任务状态: active(已开始/待处理) / done(已完成)';


--
-- Name: COLUMN tasks.started_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.started_at IS '任务开始时间（cron/间隔任务的计算时间，待办任务为创建时间）';


--
-- Name: COLUMN tasks.completed_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.completed_at IS '任务完成时间';


--
-- Name: COLUMN tasks.deadline; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.deadline IS '计划完成截止时间，超过此时间未完成视为逾期';


--
-- Name: COLUMN tasks.remark; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.remark IS '备注';


--
-- Name: COLUMN tasks.created_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.created_at IS '记录创建时间';


--
-- Name: COLUMN tasks.is_deleted; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.is_deleted IS '软删除标记';


--
-- Name: COLUMN tasks.scheduled_date; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.scheduled_date IS '计划日期';


--
-- Name: COLUMN tasks.rating; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.rating IS '完成评分';


--
-- Name: COLUMN tasks.updated_at; Type: COMMENT; Schema: public; Owner: postgres
--

COMMENT ON COLUMN public.tasks.updated_at IS '更新时间';


--
-- Name: tasks_id_seq; Type: SEQUENCE; Schema: public; Owner: postgres
--

ALTER TABLE public.tasks
    ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
        SEQUENCE NAME public.tasks_id_seq
        START WITH 1
        INCREMENT BY 1
        NO MINVALUE
        NO MAXVALUE
        CACHE 1
        );


--
-- Name: cron_job_runs id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cron_job_runs
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.cron_job_runs_id_seq'::regclass);


--
-- Name: cron_jobs id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cron_jobs
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.cron_jobs_id_seq'::regclass);


--
-- Name: data_caches id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.data_caches
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.data_caches_id_seq'::regclass);


--
-- Name: mqtt_sync_messages id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.mqtt_sync_messages
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.mqtt_sync_messages_id_seq'::regclass);


--
-- Name: mqtt_sync_nodes id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.mqtt_sync_nodes
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.mqtt_sync_nodes_id_seq'::regclass);


--
-- Name: ssh_connections id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ssh_connections
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.ssh_connections_id_seq'::regclass);


--
-- Name: standard_datas id; Type: DEFAULT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.standard_datas
    ALTER COLUMN id SET DEFAULT NEXTVAL('public.standard_datas_id_seq'::regclass);


--
-- Name: bookmarks bookmarks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.bookmarks
    ADD CONSTRAINT bookmarks_pkey PRIMARY KEY (id);


--
-- Name: browser_histories browser_history_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.browser_histories
    ADD CONSTRAINT browser_history_pkey PRIMARY KEY (id);


--
-- Name: browser_history_visits browser_history_visits_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.browser_history_visits
    ADD CONSTRAINT browser_history_visits_pkey PRIMARY KEY (id);


--
-- Name: cron_job_runs cron_job_runs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cron_job_runs
    ADD CONSTRAINT cron_job_runs_pkey PRIMARY KEY (id);


--
-- Name: cron_jobs cron_jobs_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.cron_jobs
    ADD CONSTRAINT cron_jobs_pkey PRIMARY KEY (id);


--
-- Name: data_caches data_caches_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.data_caches
    ADD CONSTRAINT data_caches_pkey PRIMARY KEY (id);


--
-- Name: file_links file_link_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.file_links
    ADD CONSTRAINT file_link_pkey PRIMARY KEY (id);


--
-- Name: mqtt_sync_messages mqtt_sync_messages_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.mqtt_sync_messages
    ADD CONSTRAINT mqtt_sync_messages_pkey PRIMARY KEY (id);


--
-- Name: mqtt_sync_nodes mqtt_sync_nodes_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.mqtt_sync_nodes
    ADD CONSTRAINT mqtt_sync_nodes_pkey PRIMARY KEY (id);


--
-- Name: nowcoder_questions nowcoder_questions_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.nowcoder_questions
    ADD CONSTRAINT nowcoder_questions_pkey PRIMARY KEY (question_id, qtype);


--
-- Name: task_plan_old pk_task_plan; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_plan_old
    ADD CONSTRAINT pk_task_plan PRIMARY KEY (id);


--
-- Name: port_forwarding port_forwarding_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.port_forwarding
    ADD CONSTRAINT port_forwarding_pkey PRIMARY KEY (id);


--
-- Name: project_groups project_groups_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.project_groups
    ADD CONSTRAINT project_groups_pkey PRIMARY KEY (id);


--
-- Name: projects projects_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_pkey PRIMARY KEY (id);


--
-- Name: quick_edit_files quick_edit_files_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.quick_edit_files
    ADD CONSTRAINT quick_edit_files_pkey PRIMARY KEY (id);


--
-- Name: quick_edit_snapshots quick_edit_snapshots_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.quick_edit_snapshots
    ADD CONSTRAINT quick_edit_snapshots_pkey PRIMARY KEY (id);


--
-- Name: sdk_sources sdk_sources_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.sdk_sources
    ADD CONSTRAINT sdk_sources_pkey PRIMARY KEY (id);


--
-- Name: ssh_connections ssh_connections_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.ssh_connections
    ADD CONSTRAINT ssh_connections_pkey PRIMARY KEY (id);


--
-- Name: standard_datas standard_datas_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.standard_datas
    ADD CONSTRAINT standard_datas_pkey PRIMARY KEY (id);


--
-- Name: task_plans task_plans_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_plans
    ADD CONSTRAINT task_plans_pkey PRIMARY KEY (id);


--
-- Name: tasks tasks_pkey; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tasks
    ADD CONSTRAINT tasks_pkey PRIMARY KEY (id);


--
-- Name: task_plan_old uni_code; Type: CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_plan_old
    ADD CONSTRAINT uni_code UNIQUE (code);


--
-- Name: idx_history_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_history_id ON public.browser_history_visits USING btree (history_id);


--
-- Name: idx_mqtt_sync_messages_msg_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_mqtt_sync_messages_msg_id ON public.mqtt_sync_messages USING btree (msg_id);


--
-- Name: idx_mqtt_sync_nodes_node_id; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_mqtt_sync_nodes_node_id ON public.mqtt_sync_nodes USING btree (node_id);


--
-- Name: idx_pid; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_pid ON public.task_plan_old USING btree (pid);


--
-- Name: idx_project_groups_path; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_project_groups_path ON public.project_groups USING btree (absolute_path);


--
-- Name: idx_project_groups_recycle; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_project_groups_recycle ON public.project_groups USING btree (is_recycle_bin) WHERE (is_recycle_bin = TRUE);


--
-- Name: idx_projects_abs_path; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_projects_abs_path ON public.projects USING btree (absolute_path) WHERE (is_deleted = FALSE);


--
-- Name: idx_projects_group; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_projects_group ON public.projects USING btree (group_id);


--
-- Name: idx_quick_edit_files_path; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_quick_edit_files_path ON public.quick_edit_files USING btree (file_path);


--
-- Name: idx_quick_edit_snapshots_file_time; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_quick_edit_snapshots_file_time ON public.quick_edit_snapshots USING btree (file_id, created_at);


--
-- Name: idx_sdk_sources_name; Type: INDEX; Schema: public; Owner: postgres
--

CREATE UNIQUE INDEX idx_sdk_sources_name ON public.sdk_sources USING btree (name) WHERE (is_deleted = FALSE);


--
-- Name: idx_status_type; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_status_type ON public.task_plan_old USING btree (status, type);


--
-- Name: idx_task_plans_parent; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_task_plans_parent ON public.task_plans USING btree (parent_id);


--
-- Name: idx_task_plans_type_status; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_task_plans_type_status ON public.task_plans USING btree (plan_type, status);


--
-- Name: idx_tasks_plan; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_tasks_plan ON public.tasks USING btree (plan_id);


--
-- Name: idx_tasks_queue; Type: INDEX; Schema: public; Owner: postgres
--

CREATE INDEX idx_tasks_queue ON public.tasks USING btree (status, deadline);


--
-- Name: projects projects_group_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.projects
    ADD CONSTRAINT projects_group_id_fkey FOREIGN KEY (group_id) REFERENCES public.project_groups (id) ON DELETE RESTRICT;


--
-- Name: quick_edit_snapshots quick_edit_snapshots_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.quick_edit_snapshots
    ADD CONSTRAINT quick_edit_snapshots_file_id_fkey FOREIGN KEY (file_id) REFERENCES public.quick_edit_files (id) ON DELETE CASCADE;


--
-- Name: task_plans task_plans_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.task_plans
    ADD CONSTRAINT task_plans_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.task_plans (id) ON DELETE SET NULL;


--
-- Name: tasks tasks_plan_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
--

ALTER TABLE ONLY public.tasks
    ADD CONSTRAINT tasks_plan_id_fkey FOREIGN KEY (plan_id) REFERENCES public.task_plans (id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--

\unrestrict 48Yjo12VB9FAYfENoGa79cFLuPTwltL9Xb8c9u3vH7VAbNUJ3ubbUmqJhycSDuY

