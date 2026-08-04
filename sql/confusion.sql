SHOW ENGINE INNODB STATUS;

/*
=====================================
2026-07-31 11:29:22 131116230055488 INNODB MONITOR OUTPUT
=====================================
Per second averages calculated from the last 22 seconds
-----------------
BACKGROUND THREAD
-----------------
srv_master_thread loops: 1 srv_active, 0 srv_shutdown, 111 srv_idle
srv_master_thread log flush and writes: 0
    ----------
    SEMAPHORES
    ----------
    OS WAIT ARRAY INFO: reservation count 6
    OS WAIT ARRAY INFO: signal count 6
    RW-shared spins 0, rounds 0, OS waits 0
    RW-excl spins 0, rounds 0, OS waits 0
    RW-sx spins 0, rounds 0, OS waits 0
    Spin rounds per wait: 0.00 RW-shared, 0.00 RW-excl, 0.00 RW-sx
    ------------
    TRANSACTIONS
    ------------
    Trx id counter 4736270
Purge done for trx's n:o < 4736267 undo n:o < 0 state: running but idle
History list length 5
LIST OF TRANSACTIONS FOR EACH SESSION:
---TRANSACTION 412591673752792, not started
0 lock struct(s), heap size 1128, 0 row lock(s)
---TRANSACTION 412591673751984, not started
0 lock struct(s), heap size 1128, 0 row lock(s)
---TRANSACTION 412591673751176, not started
0 lock struct(s), heap size 1128, 0 row lock(s)
--------
FILE I/O
--------
I/O thread 0 state: waiting for completed aio requests (insert buffer thread)
I/O thread 1 state: waiting for completed aio requests (read thread)
I/O thread 2 state: waiting for completed aio requests (read thread)
I/O thread 3 state: waiting for completed aio requests (read thread)
I/O thread 4 state: waiting for completed aio requests (read thread)
I/O thread 5 state: waiting for completed aio requests (write thread)
I/O thread 6 state: waiting for completed aio requests (write thread)
I/O thread 7 state: waiting for completed aio requests (write thread)
I/O thread 8 state: waiting for completed aio requests (write thread)
Pending normal aio reads: [0, 0, 0, 0] , aio writes: [0, 0, 0, 0] ,
 ibuf aio reads:
Pending flushes (fsync) log: 0; buffer pool: 0
1285 OS file reads, 251 OS file writes, 100 OS fsyncs
0.18 reads/s, 16384 avg bytes/read, 0.00 writes/s, 0.00 fsyncs/s
-------------------------------------
INSERT BUFFER AND ADAPTIVE HASH INDEX
-------------------------------------
Ibuf: size 1, free list len 1094, seg size 1096, 0 merges
merged operations:
 insert 0, delete mark 0, delete 0
discarded operations:
 insert 0, delete mark 0, delete 0
Hash table size 34679, node heap has 3 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
Hash table size 34679, node heap has 1 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
Hash table size 34679, node heap has 0 buffer(s)
0.00 hash searches/s, 0.86 non-hash searches/s
---
LOG
---
Log capacity                 104857600
Log capacity used            104857600
Log sequence number          315805631200
Log buffer assigned up to    315805631200
Log buffer completed up to   315805631200
Log written up to            315805631200
Log flushed up to            315805631200
Added dirty pages up to      315805631200
Pages flushed up to          315805631200
Last checkpoint at           315805631200
Log minimum file id is       96436
Log maximum file id is       96436
28 log i/o's done, 0.00 log i/o's/second
----------------------
BUFFER POOL AND MEMORY
----------------------
Total large memory allocated 0
Dictionary memory allocated 469514
Buffer pool size   8192
Free buffers       6906
Database pages     1282
Old database pages 493
Modified db pages  0
Pending reads      0
Pending writes: LRU 0, flush list 0, single page 0
Pages made young 0, not young 0
0.00 youngs/s, 0.00 non-youngs/s
Pages read 1139, created 143, written 186
0.18 reads/s, 0.00 creates/s, 0.00 writes/s
Buffer pool hit rate 925 / 1000, young-making rate 0 / 1000 not 0 / 1000
Pages read ahead 0.00/s, evicted without access 0.00/s, Random read ahead 0.00/s
LRU len: 1282, unzip_LRU len: 0
I/O sum[0]:cur[4], unzip sum[0]:cur[0]
--------------
ROW OPERATIONS
--------------
0 queries inside InnoDB, 0 queries in queue
0 read views open inside InnoDB
Process ID=1, Main thread ID=131116132451904 , state=sleeping
Number of rows inserted 0, updated 0, deleted 0, read 0
0.00 inserts/s, 0.00 updates/s, 0.00 deletes/s, 0.00 reads/s
Number of system rows inserted 8, updated 331, deleted 8, read 5010
0.00 inserts/s, 0.00 updates/s, 0.00 deletes/s, 0.32 reads/s
----------------------------
END OF INNODB MONITOR OUTPUT
============================


 */


 show VARIABLES LIKE '%innodb_print_all_deadlocks%';


select * from information_schema.tables where TABLE_SCHEMA = 'confusion';

show index from backup_chrome_history;

EXPLAIN FORMAT=JSON
SELECT *
FROM backup_chrome_history
WHERE tenant_id = 'WIN11-HP'
  AND table_name = 'meta';



show VARIABLES LIKE '%long_query_time%' ;





select @@tx_isolation;
select @@transaction_isolation;
set session transaction_isolation='READ-UNCOMMITTED';
set session transaction_isolation='READ-COMMITTED';
set session transaction_isolation='REPEATABLE-READ';
set session transaction_isolation='SERIALIZABLE';











