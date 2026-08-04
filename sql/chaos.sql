-- chaos.sql

-- 查询当前待办任务
SELECT t.started_at,
       t.deadline,
       tp.name,
       tp.fsrs_stability,
       tp.fsrs_difficulty,
       tp.fsrs_lapses,
       tp.fsrs_state,
       tp.fsrs_reps,
       tp.fsrs_last_review_at
FROM tasks t
         INNER JOIN public.task_plans tp ON tp.id = t.plan_id
WHERE t.status = 'active'
ORDER BY t.started_at;


-- PostgresSQL获取所有表
SELECT table_name
FROM information_schema.tables;

select * from quick_edit_files;

select CAST('1111111111' as INTEGER);
