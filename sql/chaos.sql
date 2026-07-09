-- chaos.sql
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



SELECT DATE(started_at) AS date, COUNT(*) AS count
FROM "tasks"
WHERE status = 'active'
  AND is_deleted = FALSE
GROUP BY DATE(started_at)
ORDER BY date ASC;

SELECT * from task_plans where name = '每日计划';



SELECT id, name, link, SUBSTR(link, 0, 24) AS content_path
FROM task_plans
WHERE link != '';


SELECT * from project_groups;